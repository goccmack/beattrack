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
	"flag"
	"fmt"
	"math"
	"os"
	"path"
	"strings"
	"time"

	"github.com/goccmack/godsp"
	"github.com/goccmack/godsp/dwt"
)

const (
	// DWTLevel is the number of scales over which the DWT will be computed
	DWTLevel = 4
	// Scale is the number of times the energy envelope divides the length
	// of the signal
	Scale = 1 << DWTLevel
	// FrameSec is size of each frame in seconds
	FrameSec = 2
	// FrameIncSec is the number of seconds by which the frame is moved.
	FrameIncSec = 1
	// CorrelationSec is the maximum lag of the autocorrelation of the
	// energy envelope in seconds.
	CorrelationSec = 1.5
	// BinsPerSec is the number of bins per second in the histogram
	BinsPerSec = 30
	// SmoothWdw is the numer of energy envelope samples used to smooth the signal.
	SmoothWdw = 30
	// MovAvgWdw is the number if samples of energy envelop used to flatten the
	// autocorrelation of the energy envelop.
	MovAvgWdw = 200
	// Persitence of a peak in the peak detection algoritm
	PeakPersistence = .5
	// MaxDanceTempo in Hz
	MaxDanceTempo = 3.06

	// Directory for output
	outDir = "out"
)

var (
	inFileName  string
	outFileName string
	outPlotData = false

	maxCorrelationDelay int
	frameSize           int

	// Wav file parameters
	bitsPerSample int
	fs            int // Sampling frequency in Hz
	numSamples    int
	numChannels   int

	frameInc       int // Number of samples by which frame is moved
	histogram      []int
	binSize        int
	histogramPeaks []int

	frameRecords      = make([]*frameRecord, 0, 256)
	averageBeatLength int

	minDancePeakOffs int

	impulse []float64
)

func main() {
	start := time.Now()
	getParams()

	var channels [][]float64
	channels, fs, bitsPerSample = godsp.ReadWavFile(inFileName)
	fmt.Printf("bits per sample %d\n", bitsPerSample)

	numChannels = len(channels)

	// Compute parameters
	frameSize = FrameSec * fs / Scale
	frameInc = int(FrameIncSec*float64(fs)) / Scale
	fmt.Printf("frameSize=%d numSamples=%d\n", frameSize, numSamples)
	maxCorrelationDelay = int(CorrelationSec*float64(fs)) / Scale
	numBins := math.Ceil(BinsPerSec * CorrelationSec)
	binSize = int(math.Ceil(CorrelationSec * float64(fs) / (numBins * float64(Scale))))
	fmt.Printf("%d bins size %d\n", int(numBins), binSize)
	histogram = make([]int, int(numBins))
	impulse = getImpulse()
	minDancePeakOffs = getMinDancePeakOffs()
	fmt.Printf("minDancePeakOffs = %d\n", minDancePeakOffs)

	db4 := dwt.Daubechies4(channels[0], DWTLevel)
	// coefs := godsp.LowpassFilterAll(db4.GetCoefficients(), .99)
	coefs := db4.GetCoefficients()
	absX := godsp.AbsAll(coefs)
	dsX := godsp.DownSampleAll(absX)
	// normX := godsp.RemoveAvgAllZ(dsX)
	sumX := godsp.SumVectors(dsX)
	sumX = godsp.DivS(sumX, godsp.Average(sumX))

	if outPlotData {
		godsp.WriteDataFile(sumX, "out/sumX")
	}

	getMainRhythms(sumX)

	generateFrameRecords(sumX, len(channels[0]))

	if outPlotData {
		godsp.WriteIntDataFile(histogram, path.Join(outDir, "histogram"))
	}
	histogramPeaks = getHistogramPeaks()

	averageBeatLength = getAvgBeatLen()

	getBeatForFrames()

	writeFrameRecords()

	if outPlotData {
		writeMLBeat(godsp.Max(channels[0]), len(channels[0]))
		writeScaleBeat(len(sumX))
	}

	fmt.Println(time.Now().Sub(start))
}

func getAvgBeatLen() int {
	_, maxBin := godsp.FindMaxI(histogram)
	return binSize * maxBin
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
	for from < sLen/Scale-frameSize {
		fmt.Printf("processFrames: i %d, offs %d\n", frameNo, from)
		generateFrameRecord(channel[from:from+frameSize], frameNo, from)
		from, frameNo = from+frameInc, frameNo+1
	}
	return
}

func generateFrameRecord(sumX []float64, frameNo, offset int) {
	if outPlotData {
		godsp.WriteDataFile(sumX, fmt.Sprintf("%s/sumX%03d", outDir, frameNo))
	}
	acX := godsp.Xcorr(sumX, sumX, maxCorrelationDelay)
	acX = godsp.Sub(acX, godsp.MovAvg(acX, MovAvgWdw))
	for i := 0; i < MovAvgWdw; i++ {
		acX[i] = 0
	}
	for i := len(acX) - MovAvgWdw; i < len(acX); i++ {
		acX[i] = 0
	}
	godsp.Smooth(acX, 40)
	zeroNeg(acX)
	if outPlotData {
		godsp.WriteDataFile(acX, fmt.Sprintf("%s/sumAC%03d", outDir, frameNo))
	}
	pks := godsp.GetPeaks(acX)
	// pkIdx := pks.GetIndices(.2)
	pkIdx := pks.GetIndices(PeakPersistence)

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
		acE:            acX,
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

func getBeat(btlen, frmSize, frmOffs int) []float64 {
	fmt.Printf("getBeat %d %d %d\n", btlen, frmSize, frmOffs)
	bt := make([]float64, frameSize)
	for i := frmOffs; i < len(bt); i += btlen {
		bt[i] = 1
	}
	return bt
}

func getHistogramPeaks() []int {
	histPeaks := godsp.GetPeaksInt(histogram)
	peaks := histPeaks.GetIndices(.25)
	for i, pk := range peaks {
		peaks[i] = pk * binSize
	}
	if outPlotData {
		godsp.WriteIntDataFile(peaks, path.Join(outDir, "histogramPeaks"))
	}
	return peaks
}

func getBeatForFrames() {
	totalBeat, numBeat := 0, 0
	for _, fr := range frameRecords {
		getBeatForFrame(fr)
		if fr.err == nil {
			totalBeat += fr.beatLen
			numBeat++
		}
	}
	// fmt.Printf("getBeatForFrames: avg beat len = %d\n",
	// 	int(float64(totalBeat)/float64(numBeat)))
}

func getBeatForFrame(fr *frameRecord) {
	fr.beatLen, fr.errorValue, fr.err = getBeatLen(fr)

	fmt.Printf("getBeatForFrame %d: btLen %d\n", fr.frameNo, fr.beatLen)

	if fr.err == nil {
		getBeatOffset(fr)
	}
}

func getBeatLen(fr *frameRecord) (int, float64, error) {
	btLen, err := getBiggestEACPeakOffset(fr)
	if err != nil {
		return -1, 1, err
	}
	return btLen, 0, nil

	// btLen, err := getBestBeat(fr)
	// if err > .15 {
	// 	fmt.Printf("getBeatLen: fno %d, btLen %d, err %.3f\n", fr.frameNo, btLen, err)
	// 	return -1, err, fmt.Errorf("Beat error too large: %f", err)
	// }
	// return btLen, err, nil
}

// return the offset in the frame of the biggest peak
func getBiggestEACPeakOffset(fr *frameRecord) (int, error) {
	fmt.Printf("getBiggestPeak frm %d\n", fr.frameNo)
	biggest := -1
	for i, pk := range fr.acEEPeaks {
		fmt.Printf("  peak %d offs %d = %f - %.2f Hz\n", i, pk, fr.acE[pk],
			float64(fs)/float64(Scale*pk))
		if (biggest == -1 || fr.acE[pk] > fr.acE[fr.acEEPeaks[biggest]]) && pk >= minDancePeakOffs {
			biggest = i
		}
	}
	if biggest == -1 {
		return -1, fmt.Errorf("No valid peak")
	}
	fmt.Printf("  biggest %d %d=%f\n", biggest, fr.acEEPeaks[biggest], fr.acE[fr.acEEPeaks[biggest]])
	return fr.acEEPeaks[biggest], nil
}

/*
Return the base beat of the peak that most closely matches the histogram
*/
func getBestBeat(fr *frameRecord) (btLen int, err float64) {
	minErr, bestPkI, bestACPk := math.Inf(1), -1, -1
	for _, acPk := range fr.acEEPeaks {
		if acPk < frameInc {
			// fmt.Printf("  acPk %d\n", acPk)
			for pkI, hPk := range histogramPeaks {
				// fmt.Printf("    pki %d pk %d", pkI, hPk)
				if err := getErr(acPk, hPk); err < minErr {
					// fmt.Printf(" err %.3f", err)
					minErr = err
					bestPkI = pkI
					bestACPk = acPk
				}
				// fmt.Println()
			}
		}
	}
	// fmt.Printf("getBestBeat fno %d bestACPk %d bestPkI %d\n", fr.frameNo, bestACPk, bestPkI)
	return int(1 * float64(bestACPk) / float64(bestPkI+1)), minErr
}

func getErr(x, y int) float64 {
	return math.Abs(float64(x-y) / float64(y))
}

// func getBeatOffset(fr *frameRecord) {
// 	// XCorrelate energy envelope of this frame with impulse
// 	fr.xcEWithBeat = godsp.Xcorr(impulse, fr.energyEnvelope, frameSize)
// 	if outPlotData {
// 		godsp.WriteDataFile(fr.xcEWithBeat, getFileName(outDir, "xcEBeat", fr.frameNo))
// 	}

// 	earliestBeatOffset := fr.offset

// 	// fmt.Printf("getBeatOffset fno %d\n", fr.frameNo)
// 	if fr.frameNo > 0 {
// 		lastFrame := frameRecords[fr.frameNo-1]
// 		// fmt.Printf("  lastFrame %d err %s\n", lastFrame.frameNo, lastFrame.err)
// 		if lastFrame.err == nil {
// 			earliestBeatOffset = lastFrame.lastBeat() + lastFrame.beatLen
// 		}
// 		earliestBeatOffset -= fr.offset
// 	}

// 	fmt.Printf("  getBeatOffset fno %d, foffs %d eoffs %d eoffs\n", fr.frameNo, fr.offset, earliestBeatOffset)
// 	fr.beatOffs = findEnergyFront(fr.xcEWithBeat, earliestBeatOffset)
// 	if fr.beatOffs-fr.beatLen >= earliestBeatOffset {
// 		fr.beatOffs -= fr.beatLen
// 	}

// 	// fmt.Printf("    fno %d: offs %d beat after %d btOffs %d lastBt %d\n",
// 	// 	fr.frameNo, fr.offset, fr.beatOffs-fr.offset,
// 	// 	fr.beatOffs, fr.lastBeat())
// }

func getBeatOffset(fr *frameRecord) {
	fmt.Println("getBeatOffset frm ", fr.frameNo)
	bt := getBeat(fr.beatLen, len(fr.energyEnvelope), 0)
	// XCorrelate energy envelope of this frame with beat
	fr.xcEWithBeat = godsp.Xcorr(fr.energyEnvelope, bt, frameSize)
	fmt.Println("  XCor done")
	if outPlotData {
		godsp.WriteDataFile(fr.xcEWithBeat, getFileName(outDir, "xcEBeat", fr.frameNo))
	}

	earliestBeatOffset := fr.offset

	fmt.Printf("getBeatOffset fno %d\n", fr.frameNo)
	if fr.frameNo > 0 {
		lastFrame := frameRecords[fr.frameNo-1]
		// fmt.Printf("  lastFrame %d err %s\n", lastFrame.frameNo, lastFrame.err)
		if lastFrame.err == nil {
			earliestBeatOffset = lastFrame.lastBeat() + lastFrame.beatLen
		}
		earliestBeatOffset -= fr.offset
	}

	fmt.Printf("  getBeatOffset fno %d, foffs %d eoffs %d eoffs\n", fr.frameNo, fr.offset, earliestBeatOffset)
	fr.beatOffs = findEnergyFront(fr.xcEWithBeat, earliestBeatOffset)
	if fr.beatOffs-fr.beatLen >= earliestBeatOffset {
		fr.beatOffs -= fr.beatLen
	}

	fmt.Printf("    fno %d: offs %d beat after %d btOffs %d lastBt %d\n",
		fr.frameNo, fr.offset, fr.beatOffs-fr.offset,
		fr.beatOffs, fr.lastBeat())
}

func findEnergyFront(xc []float64, offset int) int {
	fmt.Printf("findEnergyFront len(xc)=%d offset=%d\n", len(xc), offset)
	wdw := 200
	avg := godsp.Average(xc[offset:])

	// fmt.Printf("findEFront\n")
	for i := offset; i < len(xc)-wdw; i += 10 {
		fmt.Printf("   %d: %f %f\n", i, avg, godsp.Max(xc[i:i+wdw]))
		if godsp.Max(xc[i:i+wdw]) >= avg {
			for j := i; j < i+wdw; j++ {
				if xc[j] >= avg {
					return j
				}
			}
			panic("max not found")
		}
	}
	return offset
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
	if minDiff*Scale > fs/10 {
		return -1
	}
	return minHI
}

func getImpulse() []float64 {
	x := make([]float64, frameSize)
	N := 100
	for i := 0; i < N; i++ {
		x[i] = float64(100 - i)
	}
	return x
}

func getMainRhythms(sumX []float64) {
	fmt.Println("Start AC")
	r := godsp.Xcorr(sumX, sumX, maxCorrelationDelay)
	pks := godsp.GetPeaks(r)
	idcs := pks.GetIndices(PeakPersistence)
	for _, idx := range idcs {
		fmt.Printf("%d: %.3f - %.1f Hz\n",
			idx, r[idx],
			(float64(44100)/16)/float64(idx))
	}
	fmt.Println("AC done")
}

func getMinDancePeakOffs() int {
	offs := float64(fs) / MaxDanceTempo
	return int(offs / float64(Scale))
}

/*
writeMLBeat writes a beat for MatLab
*/
func writeMLBeat(btValue float64, numSamples int) {
	bt := make([]float64, numSamples)
	for _, fr := range frameRecords {
		if fr.err == nil {
			for i := Scale * (fr.offset + fr.beatOffs); i <= Scale*fr.lastBeat(); i += Scale * fr.beatLen {
				for j := i; j < i+50; j++ {
					bt[j] = 1
				}
			}
		}
	}
	if outPlotData {
		godsp.WriteDataFile(bt, path.Join(outDir, "beat.txt"))
	}
}

func writeScaleBeat(numSamples int) {
	bt := make([]float64, numSamples)
	for _, fr := range frameRecords {
		if fr.err == nil {
			for i := fr.offset + fr.beatOffs; i <= fr.lastBeat(); i += fr.beatLen {
				bt[i] = 1
			}
		}
	}
	godsp.WriteDataFile(bt, "out/beat_scale")
}

func zeroNeg(x []float64) {
	for i, v := range x {
		if v < 0 {
			x[i] = 0
		}
	}
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
	help := flag.Bool("h", false, "")
	plot := flag.Bool("plot", false, "")
	outFile := flag.String("o", "", "")
	flag.Parse()
	if flag.NArg() != 1 {
		fail("WAV file name required")
	}
	if *help {
		usage()
		os.Exit(0)
	}
	outPlotData = *plot
	inFileName = flag.Arg(0)
	if *outFile == "" {
		outFileName = fromInFileName()
	} else {
		outFileName = *outFile
	}
}

func fromInFileName() string {
	dir, fname := path.Split(inFileName)
	fnames := strings.Split(fname, ".")
	fnames = append(fnames[:len(fnames)-1], "beat", "json")
	return path.Join(dir, strings.Join(fnames, "."))
}

func usage() {
	fmt.Println(usageString)
}

const usageString = `use: beattrack [-plot] [-o <out file>] <WAV File> or
     beat -h
where 
	-h displays this help
	
	<WAV File> is the name of the input WAV file.
	
	-plot: Optional. Default false. Generate files for plotting in matlab.

    -o <out file>: Optional. Default <WAV File>.beat.json`
