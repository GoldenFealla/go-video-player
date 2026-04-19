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

func (ab *AudioBuffer) Push(d AudioData) {
	ab.mu.Lock()
	defer ab.mu.Unlock()

	// wait if full
	for ab.size == ab.capa {
		ab.cond.Wait()
	}

	ab.data[ab.w] = d
	ab.w = (ab.w + 1) % ab.capa
	ab.size++

	ab.cond.Signal()
}

func (ab *AudioBuffer) PeekBlocking() AudioData {
	ab.mu.Lock()
	defer ab.mu.Unlock()

	for ab.size == 0 {
		ab.cond.Wait()
	}

	return ab.data[ab.r]
}
func (ab *AudioBuffer) Pop() {
	ab.mu.Lock()
	defer ab.mu.Unlock()

	// wait if empty
	for ab.size == 0 {
		ab.cond.Wait()
	}

	ab.data[ab.r] = AudioData{}
	ab.r = (ab.r + 1) % ab.capa
	ab.size--

	ab.cond.Signal()
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
	ab := &VideoBuffer{
		data: make([]VideoData, capacity),
		capa: capacity,
	}
	ab.cond = sync.NewCond(&ab.mu)
	return ab
}

func (ab *VideoBuffer) Push(d VideoData) {
	ab.mu.Lock()
	defer ab.mu.Unlock()

	// wait if full
	for ab.size == ab.capa {
		ab.cond.Wait()
	}

	ab.data[ab.w] = d
	ab.w = (ab.w + 1) % ab.capa
	ab.size++

	ab.cond.Signal()
}

func (ab *VideoBuffer) PeekBlocking() *VideoData {
	ab.mu.Lock()
	defer ab.mu.Unlock()

	for ab.size == 0 {
		ab.cond.Wait()
	}

	return &ab.data[ab.r]
}
func (ab *VideoBuffer) Pop() {
	ab.mu.Lock()
	defer ab.mu.Unlock()

	// wait if empty
	for ab.size == 0 {
		ab.cond.Wait()
	}

	ab.data[ab.r] = VideoData{}
	ab.r = (ab.r + 1) % ab.capa
	ab.size--

	ab.cond.Signal()
}
