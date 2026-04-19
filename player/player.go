package player

import (
	"log"
	"math"
	"time"

	"github.com/asticode/go-astiav"
	"github.com/veandco/go-sdl2/sdl"

	"GoldenFealla/go-video-player/codec"
)

const maxQueue = 7680 / 2

type Player struct {
	codec         *codec.Codec
	clock         *clock
	audiodeviceid sdl.AudioDeviceID
	Volume        float32

	sent     int64
	timebase astiav.Rational
}

func NewPlayer() *Player {
	return &Player{
		codec:  codec.NewCodec(),
		clock:  &clock{d: 0},
		Volume: 0.5,
	}
}

func (p *Player) Load(path string) error {
	var err error

	err = p.codec.Load(path)
	if err != nil {
		return err
	}

	p.loadAudioPlayback()

	if p.codec.HasAudio() {
		tb := p.codec.AudioTimebase()
		p.timebase = astiav.NewRational(1, tb.Den()*4)
	}

	return nil
}

func (p *Player) Seek() {

}

func (p *Player) Play() error {
	defer sdl.CloseAudioDevice(p.audiodeviceid)
	p.codec.Parse()
	return nil
}

func (p *Player) Second() float64 {
	return p.clock.d
}

func (p *Player) Clock() {
	for {
		data := p.codec.AudioBuffer.PeekBlocking()
		p.playAudio(data.Samples)
		p.clock.set(data.PTS)
		p.codec.AudioBuffer.Pop()
	}
}

func (p *Player) VideoBuffer() *codec.VideoBuffer {
	return p.codec.VideoBuffer
}

func (p *Player) playAudio(samples []byte) {
	for sdl.GetQueuedAudioSize(p.audiodeviceid) > uint32(maxQueue) {
		time.Sleep(time.Millisecond)
	}

	applyVolume(samples, p.Volume)
	sdl.QueueAudio(p.audiodeviceid, samples)
}

func (p *Player) loadAudioPlayback() error {
	var err error

	spec := sdl.AudioSpec{
		Freq:     int32(44100),
		Format:   sdl.AUDIO_S16,
		Channels: uint8(2),
		Samples:  128,
	}

	p.audiodeviceid, err = sdl.OpenAudioDevice("", false, &spec, nil, 0)
	if err != nil {
		panic(err)
	}

	log.Printf("opened audio device id: %v\n", p.audiodeviceid)
	sdl.PauseAudioDevice(p.audiodeviceid, false)

	return nil
}

func applyVolume(data []byte, vol float32) []byte {
	out := make([]byte, len(data))
	for i := 0; i+1 < len(data); i += 2 {
		// - - - - - - - - l l l l l l l l
		// r r r r r r r r - - - - - - - -
		// r r r r r r r r l l l l l l l l
		// scale then return
		sample := int16(data[i]) | int16(data[i+1])<<8
		scaled := float32(sample) * vol
		scaled = min(scaled, 32767)
		scaled = max(scaled, -32768)
		s := int16(scaled)
		out[i] = byte(s)
		out[i+1] = byte(s >> 8)
	}
	return out
}

func applyVolumeFloat(out []byte, vol float32) int {
	for i := 0; i+3 < len(out); i += 4 {
		bits := uint32(out[i]) |
			uint32(out[i+1])<<8 |
			uint32(out[i+2])<<16 |
			uint32(out[i+3])<<24

		f := math.Float32frombits(bits)
		f *= vol

		if f > 1.0 {
			f = 1.0
		}
		if f < -1.0 {
			f = -1.0
		}

		bits = math.Float32bits(f)

		out[i] = byte(bits)
		out[i+1] = byte(bits >> 8)
		out[i+2] = byte(bits >> 16)
		out[i+3] = byte(bits >> 24)
	}

	return len(out)
}
