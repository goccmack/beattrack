//  Copyright 2019 Marius Ackerman
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"path"
	"time"

	"github.com/goccmack/godsp"
	"github.com/goccmack/godsp/dwt"
)

const (
	DWT_Level      = 4
	FrameSec       = 2
	CorrelationSec = 1.5
	BinsPerSec     = 20
	SmoothWdw      = 30

	// Directory for output
	outDir = "out"
)

type frameRecord struct {
	frameNo        int
	offset         int // offset in sound channel
	energyEnvelope []float64
	acEEPeaks      []int
	xcEWithBeat    []float64
	beatOffs       int // offset of beat in frame
	beatLen        int // samples
}

var (
	inFileName  string
	outPlotData = false

	maxCorrelationDelay int
	scale               = godsp.Pow2(DWT_Level)
	frameSize           int

	// Wav file parameters
	fs          int // Sampling frequency in Hz
	numSamples  int
	numChannels int

	// frameInc          int // Number of samples by which frame is moved
	histogram      []int
	binSize        int
	histogramPeaks []int

	frameRecords      = make([]*frameRecord, 0, 256)
	averageBeatLength int

	impulse []float64
)

func main() {
	start := time.Now()
	getParams()

	var channels [][]float64
	channels, fs = godsp.ReadWavFile(inFileName)
	fmt.Printf("Fs %d\n", fs)
	impulse = getImpulse()

	// Compute parameters
	frameSize = FrameSec * fs / scale
	fmt.Printf("frameSize=%d numSamples=%d\n", frameSize, numSamples)
	maxCorrelationDelay = int(CorrelationSec*float64(fs)) / scale
	numBins := math.Ceil(BinsPerSec * CorrelationSec)
	binSize = int(math.Ceil(CorrelationSec * float64(fs) / (numBins * float64(scale))))
	fmt.Printf("%d bins size %d\n", int(numBins), binSize)
	histogram = make([]int, int(numBins))

	db4 := dwt.Daubechies4(channels[0], DWT_Level)
	coefs := godsp.LowpassFilterAll(db4.GetCoefficients(), .99)
	absX := godsp.AbsAll(coefs)
	dsX := godsp.DownSampleAll(absX)
	normX := godsp.RemoveAvgAllZ(dsX)
	sumX := godsp.SumVectors(normX)

	generateFrameRecords(sumX, len(channels[0]))

	if outPlotData {
		godsp.WriteIntDataFile(histogram, path.Join(outDir, "histogram"))
	}
	histPeaks := godsp.GetPeaksInt(histogram)
	histogramPeaks = histPeaks.GetIndices(.25)
	if outPlotData {
		godsp.WriteIntDataFile(histogramPeaks, path.Join(outDir, "histogramPeaks"))
	}

	averageBeatLength = binSize * getAveragePeakPeriod(histogramPeaks)
	fmt.Printf("Average beat length (/scale) %d\n", averageBeatLength)

	xcorBeatWithEnergy()
	cleanBeat()

	writeFrameRecords()

	if outPlotData {
		writeMLBeat(godsp.Max(channels[0]))
	}

	fmt.Println(time.Now().Sub(start))
}

func cleanBeat() {
	for i, fr := range frameRecords {
		if i > 0 {
			firstBeat := fr.offset + fr.beatOffs
			if firstBeat-frameRecords[fr.frameNo-1].lastBeat() < fr.beatLen {
				fr.beatOffs += fr.beatLen
			}
		}
	}
}

func getAveragePeakPeriod(peaks []int) int {
	lastI, sum, n := 0, 0, 0
	for i, offs := range peaks {
		if i > 0 && i < BinsPerSec {
			sum += offs - lastI
			lastI = offs
			n++
		}
	}
	return int(float64(sum) / float64(n))
}

func generateFrameRecords(channel []float64, sLen int) {
	from, frameNo := 0, 0
	for from < sLen/scale {
		fmt.Printf("processFrames: i %d, offs %d\n", frameNo, from)
		generateFrameRecord(channel[from:from+frameSize], frameNo, from)
		from, frameNo = from+frameSize, frameNo+1
		// from, frameNo = from+frameInc, frameNo+1
	}
	return
}

func generateFrameRecord(sumX []float64, frameNo, offset int) {
	if outPlotData {
		godsp.WriteDataFile(sumX, fmt.Sprintf("%s/sumX%03d", outDir, frameNo))
	}
	acX := godsp.Xcorr(sumX, sumX, maxCorrelationDelay)
	godsp.Smooth(acX, 30)
	if outPlotData {
		godsp.WriteDataFile(acX, fmt.Sprintf("%s/sumAC%03d", outDir, frameNo))
	}
	pks := godsp.GetPeaks(acX)
	pkIdx := pks.GetIndices(.2)
	if outPlotData {
		godsp.WriteIntDataFile(pkIdx, fmt.Sprintf("%s/peaksAC%03d", outDir, frameNo))
	}

	for _, pki := range pkIdx {
		histogram[pki/binSize]++
	}

	fr := &frameRecord{
		frameNo:        frameNo,
		offset:         offset,
		energyEnvelope: sumX,
		acEEPeaks:      pkIdx,
	}
	frameRecords = append(frameRecords, fr)
}

func getFrameSize(s int) int {
	ceilLogS := math.Ceil(math.Log2(float64(s)))
	frmSz := int(math.Pow(2, ceilLogS))

	return frmSz
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func getImpulse() []float64 {
	x := make([]float64, fs/scale)
	N := 100
	for i := 0; i < N; i++ {
		x[i] = float64(100-i) / float64(N)
	}
	return x
}

func writeChans(channels [][]float64) {
	buf0 := new(bytes.Buffer)
	for _, f := range channels[0] {
		buf0.WriteString(fmt.Sprintf("%f\n", f))
	}
	buf1 := new(bytes.Buffer)
	for _, f := range channels[1] {
		buf1.WriteString(fmt.Sprintf("%f\n", f))
	}
	ioutil.WriteFile("chan0.txt", buf0.Bytes(), 0777)
	ioutil.WriteFile("chan1.txt", buf1.Bytes(), 0777)
}

func xcorBeatWithEnergy() {
	for _, f := range frameRecords {
		xcorFrameEnergyWithAvgBeat(f)
	}
}

func xcorFrameEnergyWithAvgBeat(fr *frameRecord) {
	fr.beatLen = getBeatLen(fr)
	getBeatOffset(fr)
	bt := make([]float64, frameSize)
	for i := fr.beatOffs; i >= 0; i -= fr.beatLen {
		bt[i] = godsp.Max(fr.energyEnvelope)
	}
	for i := fr.beatOffs; i < len(bt); i += fr.beatLen {
		bt[i] = godsp.Max(fr.energyEnvelope)
	}
	if outPlotData {
		godsp.WriteDataFile(bt, getFileName(outDir, "beat", fr.frameNo))
	}
}

func getBeatLen(fr *frameRecord) int {
	bestPkI, histPkI, bestDiff := -1, -1, 0xffffffff
	for pkI, pk := range fr.acEEPeaks {
		if pkI == 0 {
			continue
		}
		hPkI := getClosestHistogramPeak(pk)
		if hPkI == -1 {
			return averageBeatLength
		}
		diff := abs(pk - binSize*histogramPeaks[hPkI])
		if diff < bestDiff {
			bestPkI = pkI
			histPkI = hPkI
			bestDiff = diff
		}
	}
	if bestDiff > 0xffff {
		return averageBeatLength
	}
	return fr.acEEPeaks[bestPkI] / histPkI
}

func getBeatOffset(fr *frameRecord) {
	fr.xcEWithBeat = godsp.Xcorr(impulse, fr.energyEnvelope, fs/scale)
	if outPlotData {
		godsp.WriteDataFile(fr.xcEWithBeat, getFileName(outDir, "xcEBeat", fr.frameNo))
	}
	avg := godsp.Average(fr.xcEWithBeat)
	lo, hi := .5*avg, 1.5*avg
	i := 0
	for i < len(fr.xcEWithBeat) && fr.xcEWithBeat[i] > lo {
		i++
	}
	for i < len(fr.xcEWithBeat) && fr.xcEWithBeat[i] < hi {
		i++
	}
	for i-fr.beatLen > 0 {
		i -= fr.beatLen
	}
	fr.beatOffs = i
	if outPlotData {
		godsp.WriteIntDataFile([]int{i}, getFileName(outDir, "xcEBeatPks", fr.frameNo))
	}
}

func getClosestHistogramPeak(pkI int) int {
	minHI, minDiff := -1, 0xffffffff
	for hi, hpk := range histogramPeaks {
		if hi == 0 {
			continue
		}
		hpk1 := binSize * hpk
		if minHI == -1 {
			minHI = hi
		}
		diff := abs(hpk1 - pkI)
		if diff < minDiff {
			minHI = hi
			minDiff = diff
		}
	}
	if minDiff*scale > fs/10 {
		return -1
	}
	return minHI
}

/*
writeMLBeat writes a beat for MatLab
*/
func writeMLBeat(btValue float64) {
	bt := make([]float64, numSamples/numChannels)
	frmSize := frameSize * scale
	for _, fr := range frameRecords {
		btLen := 2 * // slow beat down for listening
			fr.beatLen * scale
		firstBeat := scale*fr.offset + fr.beatOffs
		frmMax := firstBeat + frmSize
		for i := firstBeat; i < frmMax; i += btLen {
			for j := i; j < i+50 && j < len(bt); j++ {
				bt[j] = btValue
			}
		}
	}
	if outPlotData {
		godsp.WriteDataFile(bt, path.Join(outDir, "beat.txt"))
	}
}

/*** frameRecord ***/

func (fr *frameRecord) lastBeat() int {
	firstBeat := fr.offset + fr.beatOffs
	lastBeat := firstBeat
	maxFrame := fr.offset + frameSize
	for ; lastBeat < maxFrame; lastBeat += fr.beatLen {
	}
	return lastBeat
}

//********************

func getFileName(dir, fname string, fileNo int) string {
	fname = fmt.Sprintf("%s%03d", fname, fileNo)
	return path.Join(dir, fname)
}

func fail(msg string) {
	fmt.Printf("Error: %s\n", msg)
	usage()
	os.Exit(1)
}

func getParams() {
	plot := flag.Bool("plot", false, "")
	flag.Parse()
	if flag.NArg() != 1 {
		fail("WAV file name required")
	}
	outPlotData = *plot
	inFileName = flag.Arg(0)
}

func usage() {
	fmt.Println(usageString)
}

const usageString = `use: [-plot] beattrack <WAV File>
where <WAV File> is the name of the input WAV file.
	-plot: Optional. Default false. Generate files for plotting in matlab.`
