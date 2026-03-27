package player

type Packet struct {
	StreamIndex int
	PTS         int64
	Data        []byte
}

type VideoFrame struct {
	PTS float64
	Y   []byte
	U   []byte
	V   []byte
	W   int
	H   int
}

type AudioFrame struct {
	PTS       float64
	NbSamples uint32
	Samples   []byte
	Rate      int
}
