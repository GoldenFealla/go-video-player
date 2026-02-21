package player

import (
	"sync"

	"github.com/veandco/go-sdl2/sdl"
)

type Clock struct {
	mu sync.Mutex

	totalBytes uint32

	DeviceID sdl.AudioDeviceID
}

func (c *Clock) UpdateAudio(l uint32) {
	c.mu.Lock()
	c.totalBytes += l
	c.mu.Unlock()
}

func (c *Clock) Audio() float64 {
	c.mu.Lock()
	t := c.totalBytes
	c.mu.Unlock()
	bp := t - sdl.GetQueuedAudioSize(c.DeviceID)
	elapsed := float64(bp) / float64(48000*2*4)
	return elapsed
}
