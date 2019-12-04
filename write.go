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
	"github.com/goccmack/godsp/ioutil"
	"path"
	"strings"
)

type OutRecord struct {
	FileName           string  // Input file
	SampleRate         int     // Hz
	AverageBeatsPerSec float64 // Average of the whole file
	FrameRecords       []*OutFrameRecord
}

type OutFrameRecord struct {
	FrameNo   int
	FrameOffs int // offset of this frame in number of samples from start of channel
	BeatOffs  int // offset of the first beat from the start of the frame in samples
	BeatLen   int // length of a beat in this frame in samples
}

func writeFrameRecords() {
	or := &OutRecord{
		FileName:           inFileName,
		SampleRate:         fs,
		AverageBeatsPerSec: float64(fs) / float64(scale*averageBeatLength),
	}
	for _, fr := range frameRecords {
		or.FrameRecords = append(or.FrameRecords, getOutFrameRecord(fr))
	}
	buf, err := json.Marshal(or)
	if err != nil {
		panic(err)
	}
	if err := ioutil.WriteFile(getOutFileName(), buf); err != nil {
		panic(err)
	}
}

func getOutFileName() string {
	dir, fname := path.Split(inFileName)
	fnames := strings.Split(fname, ".")
	fnames = append(fnames[:len(fnames)-1], "beat", "json")
	return path.Join(dir, strings.Join(fnames, "."))
}

func getOutFrameRecord(fr *frameRecord) *OutFrameRecord {
	return &OutFrameRecord{
		FrameNo:   fr.frameNo,
		FrameOffs: scale * fr.offset,
		BeatOffs:  scale * (fr.offset + fr.beatOffs),
		BeatLen:   scale * fr.beatLen,
	}
}
