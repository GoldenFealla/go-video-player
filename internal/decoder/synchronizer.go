package decoder

import (
	"io"

	"fyne.io/fyne/v2"
)

type Synchronizer struct {
	ad *AudioDecoder
	// vd *VideoDecoder

	vw io.Writer
	aw io.Writer

	cur float64
}

func NewSynchronizer(ad *AudioDecoder, vw io.Writer, aw io.Writer) *Synchronizer {
	return &Synchronizer{
		ad: ad,

		vw: vw,
		aw: aw,

		cur: 0,
	}
}

func (s *Synchronizer) Run(callback func(cur float64)) {
	for {
		data, ok := <-s.ad.written
		if !ok {
			break
		}
		sampleDuration := float64(data.n) / float64(44100)
		s.cur += sampleDuration

		s.aw.Write(data.b)

		fyne.Do(func() {
			callback(s.cur)
		})
	}
}
