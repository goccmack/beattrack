package main

type frameRecord struct {
	frameNo        int
	offset         int // offset in sound channel
	energyEnvelope []float64
	acE            []float64 // autocorrelation of energy envelope
	acEEPeaks      []int
	xcEWithBeat    []float64
	beatOffs       int // offset of beat in frame
	beatLen        int // samples
	err            error
	errorValue     float64
	rhythms        []*Rhythm
}

func (fr *frameRecord) lastBeat() int {
	if fr.err != nil {
		return fr.offset
	}
	firstBeat := fr.offset + fr.beatOffs
	lastBeat := firstBeat
	maxFrame := fr.offset + frameInc
	for ; lastBeat < maxFrame-fr.beatLen; lastBeat += fr.beatLen {
	}
	// fmt.Printf("fr.lastBeat fno %d foffs %d bt offs %d bt len %d last beat %d maxFrame %d fsz %d\n",
	// 	fr.frameNo, fr.offset, fr.beatOffs, fr.beatLen, lastBeat, maxFrame, frameInc)
	return lastBeat
}

type Rhythm struct {
	Freq     float64 // Hz
	Energy   float64 // Xcorr energy of this frequency
	BeatLen  int     // Number of samples at Fs
	BeatOffs int     // Number of samples from start of frame at Fs
}
