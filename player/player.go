package player

import (
	"GoldenFealla/go-video-player/codec"
	"fmt"
)

type Player struct {
	clock  *clock
	codec  *codec.Codec
	Volume float32
	pb     *playback
}

func NewPlayer() *Player {
	return &Player{
		codec:  codec.NewCodec(),
		clock:  &clock{t: 0},
		pb:     newplayback(20),
		Volume: 0.5,
	}
}

func (p *Player) Load(path string) error {
	_, am, err := p.codec.Load(path)
	if err != nil {
		return err
	}

	p.pb.load(am.Freq)
	return nil
}

func (p *Player) Seek() {

}

func (p *Player) Play() error {
	p.codec.Parse()
	return nil
}

func (p *Player) Second() float64 {
	return p.clock.t
}

func (p *Player) Clock() {
	for {
		data := p.codec.AudioBuffer.PeekBlocking()
		p.pb.play(data.Samples, p.Volume)
		p.clock.set(data.PTS)
		p.codec.AudioBuffer.Pop()
	}
}

func (p *Player) LatestFrame() codec.VideoData {
	f := p.codec.VideoBuffer.PeekBlocking()

	if f != nil {
		master := p.clock.get()
		diff := f.PTS - master
		fmt.Printf("\r%+.3f", diff)

		if diff > 0 {
			return codec.VideoData{}
		}

		if diff < -0.05 {
			p.codec.VideoBuffer.Pop()
			return codec.VideoData{}
		}

		newFrame := codec.VideoData{
			PTS:  f.PTS,
			H:    f.H,
			W:    f.W,
			Data: f.Data,
		}

		p.codec.VideoBuffer.Pop()
		return newFrame
	}

	return codec.VideoData{}
}
