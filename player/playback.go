package player

import (
	"log"
	"math"
	"time"

	"github.com/veandco/go-sdl2/sdl"
)

type playback struct {
	// the target time in milisecond
	target   int
	deviceid sdl.AudioDeviceID
	maxqueue uint32
}

func newplayback(target int) *playback {
	return &playback{
		target: target,
	}
}

func (pb *playback) load(freq int) {
	var err error

	spec := sdl.AudioSpec{
		Channels: uint8(2),
		Freq:     int32(freq),
		Format:   sdl.AUDIO_S16,
	}

	pb.deviceid, err = sdl.OpenAudioDevice("", false, &spec, nil, 0)
	if err != nil {
		panic(err)
	}

	pb.maxqueue = uint32(2 * 4 * freq * pb.target / 1000)

	log.Printf("opened audio device id: %v\n", pb.deviceid)
	sdl.PauseAudioDevice(pb.deviceid, false)
}

func (pb *playback) play(samples []byte, volume float32) {
	for sdl.GetQueuedAudioSize(pb.deviceid) > pb.maxqueue {
		time.Sleep(time.Millisecond)
	}

	applyVolume4(samples, volume)
	sdl.QueueAudio(pb.deviceid, samples)
}

func applyVolume4(out []byte, vol float32) []byte {
	// out := make([]byte, len(data))
	for i := 0; i+1 < len(out); i += 2 {
		sample := int16(out[i]) | int16(out[i+1])<<8
		scaled := float32(sample) * vol
		scaled = min(scaled, 32767)
		scaled = max(scaled, -32768)
		s := int16(scaled)
		out[i] = byte(s)
		out[i+1] = byte(s >> 8)
	}
	return out
}

func applyVolume8(out []byte, vol float32) int {
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
