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
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	"github.com/goccmack/godsp"
	"github.com/goccmack/godsp/dwt"
	"github.com/goccmack/godsp/peaks"
	"github.com/goccmack/goutil/ioutil"
)

const (
	// DWTLevel is the number of scales over which the DWT will be computed
	DWTLevel = 4
	// Scale is the number of times the energy envelope divides the length
	// of the signal
	Scale = 1 << DWTLevel

	// DefaultPeakSepMs is the default minimum peak separation distance
	DefaultPeakSepMs = 250

	// Directory for plot output
	outDir = "out"
)

var (
	inFileName  string
	outFileName string
	outPlotData = false

	// Wav file parameters
	fs          int // Sampling frequency in Hz
	fss         int // Samples/sec at highest DWT scale
	numChannels int

	sepMs = flag.Int("sep", DefaultPeakSepMs, "")
)

type OutRecord struct {
	FileName       string // Input file
	SampleRate     int    // Fs in Hz
	NumChannels    int    // Number of channels in wav file
	PeakSeparation int    // Minimum distance between peaks in milliseconds
	Peaks          []Peak // List of detected peaks
}

type Peak struct {
	Offset   int // Number of samples from start of channel at Fs
	MsOffset int // Number of milliseconds from start of channel
}

func main() {
	start := time.Now()
	getParams()

	var channels [][]float64
	channels, fs, _ = godsp.ReadWavFile(inFileName)

	numChannels = len(channels)

	// Compute parameters
	fss = int(float64(fs) / float64(Scale))
	sepFss := *sepMs * fs / (Scale * 1000)
	if sepFss <= 0 {
		minSepMs := Scale * 1000 / fs
		fail(fmt.Sprintf("sep is too small. Minimum for this file is %d", minSepMs))
	}
	db4 := dwt.Daubechies4(channels[0], DWTLevel)
	// coefs := godsp.LowpassFilterAll(db4.GetCoefficients(), .99)
	coefs := db4.GetCoefficients()
	absX := godsp.AbsAll(coefs)
	dsX := godsp.DownSampleAll(absX)
	// normX := godsp.RemoveAvgAllZ(dsX)
	sumX := godsp.SumVectors(dsX)
	sumX = godsp.DivS(sumX, godsp.Average(sumX))
	sumXPeaks := peaks.Get(sumX, sepFss)
	writeOutput(sumXPeaks, sepFss)

	if outPlotData {
		godsp.WriteDataFile(sumX, "out/sumX")
		writeSumXPeaks(sumXPeaks, len(sumX))
		writeMLBeat(sumXPeaks, godsp.Max(channels[0]), len(channels[0]))
	}

	fmt.Println(time.Now().Sub(start))
}

// beatLen is the length of a beat at highest DWT scale
func getFreq(beatLen int) float64 {
	return float64(fss) / float64(beatLen)
}

// returns the millisec offset of offs samples at fss
func toMilliSecOffs(offs int) int {
	return offs * 1000 / fss
}

/*
writeMLBeat writes a beat for MatLab
*/
func writeMLBeat(peaks []int, btValue float64, numSamples int) {
	bt := make([]float64, numSamples)
	for _, pk := range peaks {
		for i := 0; i < 100; i++ {
			offs := pk*Scale + i
			if offs < numSamples {
				bt[pk*Scale+i] = btValue
			}
		}
	}
	godsp.WriteDataFile(bt, path.Join(outDir, "beat.txt"))
}

// Write the JSON output file
func writeOutput(peaks []int, sepFss int) {
	or := &OutRecord{
		FileName:       inFileName,
		SampleRate:     fs,
		NumChannels:    numChannels,
		PeakSeparation: toMilliSecOffs(sepFss),
		Peaks:          make([]Peak, len(peaks)),
	}
	for i, pk := range peaks {
		or.Peaks[i].Offset = Scale * pk
		or.Peaks[i].MsOffset = toMilliSecOffs(pk)
	}
	buf, err := json.Marshal(or)
	if err != nil {
		panic(err)
	}
	if err := ioutil.WriteFile(outFileName, buf); err != nil {
		panic(err)
	}
}

func writeSumXPeaks(pks []int, numsamples int) {
	sumXPks := make([]float64, numsamples)
	for _, pk := range pks {
		sumXPks[pk] = 1
	}
	godsp.WriteDataFile(sumXPks, "out/sumXPksd")
}

/*** command line parameters ***/

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

const usageString = `use: beattrack [-sep dist] [-plot] [-o <out file>] <WAV File> or
     beattrack -h
where 
    -h displays this help
	
    <WAV File> is the name of the input WAV file.
    
    -sep ms: Optional. The mininum number of millisec between adjacent peaks.
               Default: 250
	
    -plot: Optional. Default false. Generate files for plotting in matlab.

    -o <out file>: Optional. Default <WAV File>.beat.json`
