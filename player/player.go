package player

import (
	"GoldenFealla/go-video-player/codec"

	"github.com/asticode/go-astiav"
)

type Player struct {
	clock *clock
	codec *codec.Codec
	pb    *playback

	Volume   float32
	Duration float32
}

func NewPlayer() *Player {
	return &Player{
		codec:  codec.NewCodec(),
		clock:  &clock{},
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
	p.Duration = float32(p.codec.Duration()) / float32(astiav.TimeBase)

	return nil
}

func (p *Player) Play() {
	quit := make(chan struct{})
	go p.codec.Parse(quit)
	go p.Clock(quit)
}

func (p *Player) SeekSecond(second float32) {
	if p.codec.Stopped {
		p.Play()
	}
	p.codec.SeekSecond(second)
}

func (p *Player) Clock(quit chan struct{}) {
	for {
		select {
		case <-quit:
			return
		default:
			data := p.codec.AudioBuffer.Peek()
			if data == nil {
				continue
			}

			p.pb.play(data.Samples, p.Volume)
			p.clock.set(data.PTS)
			p.codec.AudioBuffer.Pop()
		}
	}
}

func (p *Player) LatestFrame() codec.VideoData {
	if p.codec.Stopped {
		return codec.VideoData{}
	}
	f := p.codec.VideoBuffer.Peek()

	if f != nil {
		master := p.clock.get()
		diff := f.PTS - master

		if diff > 0.5 {
			p.codec.VideoBuffer.Pop()
			return codec.VideoData{}
		}

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

func (p *Player) GetSecond() float32 {
	return float32(p.clock.get())
}
