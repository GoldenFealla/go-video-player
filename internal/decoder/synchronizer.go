package decoder

import (
	"image"
	"io"
)

type Synchronizer struct {
	vd *VideoDecoder
	ad *AudioDecoder

	aw io.Writer

	cur float64
}

func NewSynchronizer(vd *VideoDecoder, ad *AudioDecoder, vw io.Writer, aw io.Writer) *Synchronizer {
	return &Synchronizer{
		vd: vd,
		ad: ad,

		aw: aw,

		cur: 0,
	}
}

func (s *Synchronizer) Run(update func(cur float64, i image.Image)) {
	for {
		data, ok := <-s.ad.written
		if !ok {
			break
		}
		sampleDuration := float64(data.n) / float64(44100)
		s.cur += sampleDuration

		s.aw.Write(data.b)

		for {
			fData := s.vd.queue.GetData()
			if fData == nil {
				break
			}
		}
	}
}
