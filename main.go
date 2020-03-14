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
	"os"
	"path"
	"strings"
	"time"

	"github.com/goccmack/godsp"
	"github.com/goccmack/godsp/dwt"
	"github.com/goccmack/godsp/peaks"
)

const (
	// DWTLevel is the number of scales over which the DWT will be computed
	DWTLevel = 4
	// Scale is the number of times the energy envelope divides the length
	// of the signal
	Scale = 1 << DWTLevel

	// DefaultPeakSep is the default minimum peak separation distance
	DefaultPeakSep = 1000

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
	sep         int // Minimum number of samples between peaks
)

func main() {
	start := time.Now()
	getParams()

	var channels [][]float64
	channels, fs, _ = godsp.ReadWavFile(inFileName)

	numChannels = len(channels)

	// Compute parameters
	fss = int(float64(fs) / float64(Scale))
	db4 := dwt.Daubechies4(channels[0], DWTLevel)
	// coefs := godsp.LowpassFilterAll(db4.GetCoefficients(), .99)
	coefs := db4.GetCoefficients()
	absX := godsp.AbsAll(coefs)
	dsX := godsp.DownSampleAll(absX)
	// normX := godsp.RemoveAvgAllZ(dsX)
	sumX := godsp.SumVectors(dsX)
	sumX = godsp.DivS(sumX, godsp.Average(sumX))
	sumXPeaks := peaks.Get(sumX, 1000)

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

/*
writeMLBeat writes a beat for MatLab
*/
func writeMLBeat(peaks []int, btValue float64, numSamples int) {
	bt := make([]float64, numSamples)
	for _, pk := range peaks {
		for i := 0; i < 100; i++ {
			bt[pk*Scale+i] = btValue
		}
	}
	if outPlotData {
		godsp.WriteDataFile(bt, path.Join(outDir, "beat.txt"))
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
	sepFlag := flag.Int("sep", DefaultPeakSep, "")
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
	sep = *sepFlag
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
     beat -h
where 
	-h displays this help
	
    <WAV File> is the name of the input WAV file.
    
    -sep dist: Optional. The minum number of samples between adjacent peaks.
               Default: 1000
	
	-plot: Optional. Default false. Generate files for plotting in matlab.

    -o <out file>: Optional. Default <WAV File>.beat.json`
