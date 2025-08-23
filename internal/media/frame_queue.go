package media

import (
	"sync"

	"github.com/asticode/go-astiav"
)

type FrameQueue struct {
	frames []*astiav.Frame
	max    int

	head, tail, count int

	mutex sync.RWMutex
	cond  *sync.Cond
	busy  bool
}

func NewFrameQueue(max int) *FrameQueue {
	fq := &FrameQueue{
		frames: make([]*astiav.Frame, max),
		max:    max,
		head:   0,
		tail:   0,
		count:  0,
	}

	for i := range max {
		fq.frames[i] = astiav.AllocFrame()
	}

	fq.cond = sync.NewCond(&fq.mutex)
	return fq
}

func (fq *FrameQueue) Write(f *astiav.Frame) {
	fq.mutex.Lock()
	defer fq.mutex.Unlock()

	for fq.busy {
		fq.cond.Wait()
	}

	if fq.count >= fq.max {
		fq.cond.Wait()
	}

	fq.busy = true

	fq.frames[fq.tail].Ref(f)
	f.Copy(fq.frames[fq.tail])
	fq.frames[fq.tail].SetPts(f.Pts())

	fq.tail = (fq.tail + 1) % fq.max
	fq.count += 1

	fq.busy = false

	fq.cond.Broadcast()
}

func (fq *FrameQueue) Read() *astiav.Frame {
	fq.mutex.Lock()
	defer fq.mutex.Unlock()

	if fq.frames[fq.head] == nil {
		return nil
	}

	f := fq.frames[fq.head].Clone()
	fq.frames[fq.head].Unref()

	fq.head = (fq.head + 1) % fq.max
	fq.count -= 1

	fq.cond.Broadcast()

	return f
}

func (fq *FrameQueue) CurrentFramePTS() int64 {
	return fq.frames[fq.head].Pts()
}
