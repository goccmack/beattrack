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
)

type OutRecord struct {
	FileName           string // Input file
	SampleRate         int    // Hz
	NumChannels        int
	AverageBeatsPerSec float64 // Average of the whole file
	FrameRecords       []*OutFrameRecord
}

type OutFrameRecord struct {
	FrameNo   int
	FrameOffs int // offset of this frame in number of samples from start of channel
	BeatOffs  int // offset of the first beat from the start of the frame in samples

	BeatOffsScale int // ToDo: delete me
	LastBeatScale int // ToDo: delete
	BeatLenScale  int // ToDo: delete

	BeatLen   int // length of a beat in this frame in samples
	TimePosMs int // position of the beat from the start in ms
}

func writeFrameRecords() {
	or := &OutRecord{
		FileName:           inFileName,
		SampleRate:         fs,
		NumChannels:        numChannels,
		AverageBeatsPerSec: float64(fs) / float64(Scale*averageBeatLength),
	}
	for _, fr := range frameRecords {
		or.FrameRecords = append(or.FrameRecords, getOutFrameRecord(fr))
	}
	buf, err := json.Marshal(or)
	if err != nil {
		panic(err)
	}
	if err := ioutil.WriteFile(outFileName, buf); err != nil {
		panic(err)
	}
}

func getOutFrameRecord(fr *frameRecord) *OutFrameRecord {
	return &OutFrameRecord{
		FrameNo:       fr.frameNo,
		FrameOffs:     Scale * fr.offset,
		BeatOffs:      Scale * fr.beatOffs,
		BeatOffsScale: fr.beatOffs,
		LastBeatScale: fr.lastBeat(),
		BeatLenScale:  fr.beatLen,
		BeatLen:       Scale * fr.beatLen,
		TimePosMs:     (Scale * fr.offset) * 1000 / fs,
	}
}
