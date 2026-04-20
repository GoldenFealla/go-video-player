package codec

import (
	"sync"
)

type AudioData struct {
	PTS     float64
	Samples []byte
}

type AudioBuffer struct {
	data []AudioData
	r, w int
	size int
	capa int

	mu   sync.Mutex
	cond *sync.Cond
}

func NewAudioBuffer(capacity int) *AudioBuffer {
	ab := &AudioBuffer{
		data: make([]AudioData, capacity),
		capa: capacity,
	}
	ab.cond = sync.NewCond(&ab.mu)
	return ab
}

func (ab *AudioBuffer) Push(d AudioData) bool {
	ab.mu.Lock()
	defer ab.mu.Unlock()

	for ab.size == ab.capa {
		ab.cond.Wait()
	}

	ab.data[ab.w] = d
	ab.w = (ab.w + 1) % ab.capa
	ab.size++

	ab.cond.Signal()
	return true
}

func (ab *AudioBuffer) Peek() *AudioData {
	ab.mu.Lock()
	defer ab.mu.Unlock()

	for ab.size == 0 {
		ab.cond.Wait()
	}

	return &ab.data[ab.r]
}

func (ab *AudioBuffer) Pop() {
	ab.mu.Lock()
	defer ab.mu.Unlock()

	for ab.size == 0 {
		ab.cond.Wait()
	}

	ab.data[ab.r] = AudioData{}
	ab.r = (ab.r + 1) % ab.capa
	ab.size--
	ab.cond.Signal()
}

func (ab *AudioBuffer) Clear() {
	ab.mu.Lock()
	defer ab.mu.Unlock()

	ab.r = 0
	ab.w = 0
	ab.size = 0

	for i := range ab.data {
		ab.data[i] = AudioData{}
	}

	ab.cond.Broadcast()
}

type VideoData struct {
	PTS  float64
	W    int
	H    int
	Data []byte
}

type VideoBuffer struct {
	data []VideoData
	r, w int
	size int
	capa int

	mu   sync.Mutex
	cond *sync.Cond
}

func NewVideoBuffer(capacity int) *VideoBuffer {
	vb := &VideoBuffer{
		data: make([]VideoData, capacity),
		capa: capacity,
	}
	vb.cond = sync.NewCond(&vb.mu)
	return vb
}

func (vb *VideoBuffer) Push(d VideoData) bool {
	vb.mu.Lock()
	defer vb.mu.Unlock()

	for vb.size == vb.capa {
		vb.cond.Wait()
	}

	vb.data[vb.w] = d
	vb.w = (vb.w + 1) % vb.capa
	vb.size++

	vb.cond.Signal()
	return true
}

func (vb *VideoBuffer) Peek() *VideoData {
	vb.mu.Lock()
	defer vb.mu.Unlock()
	return &vb.data[vb.r]
}

func (vb *VideoBuffer) Pop() {
	vb.mu.Lock()
	defer vb.mu.Unlock()

	// wait if empty
	for vb.size == 0 {
		vb.cond.Wait()
	}

	vb.data[vb.r] = VideoData{}
	vb.r = (vb.r + 1) % vb.capa
	vb.size--

	vb.cond.Signal()
}

func (vb *VideoBuffer) Clear() {
	vb.mu.Lock()
	defer vb.mu.Unlock()

	vb.r = 0
	vb.w = 0
	vb.size = 0

	for i := range vb.data {
		vb.data[i] = VideoData{}
	}

	vb.cond.Broadcast()
}
