package media

import (
	"image"
	"io"
	"sync"

	"github.com/GoldenFealla/VideoPlayerGo/internal/decoder"
	"github.com/asticode/go-astiav"
)

type VideoQueue struct {
	sync.RWMutex
	queue []decoder.VideoData
	head  int
	tail  int
	count int
}

func NewVideoQueue() *VideoQueue {
	return &VideoQueue{
		queue: make([]decoder.VideoData, 0, MAX_BUFFER),
	}
}

func (vq *VideoQueue) Add(d decoder.VideoData) {
	vq.Lock()
	defer vq.Unlock()

	if len(vq.queue) >= MAX_BUFFER {
		copy(vq.queue, vq.queue[1:MAX_BUFFER-1])
	}

	vq.queue = append(vq.queue, d)
}

func (vq *VideoQueue) Get() *decoder.VideoData {
	vq.Lock()
	defer vq.Unlock()

	if len(vq.queue) <= 0 {
		return nil
	}

	d := vq.queue[0]

	if d.Time <= decoder.Cur {
		copy(vq.queue, vq.queue[1:len(vq.queue)-1])
		return &d
	}

	return nil
}

var (
	MAX_BUFFER                       = 300
	SyncAudioReader, SyncAudioWriter = io.Pipe()

	RecieveChan = make(chan decoder.VideoData)
	OutputChan  = make(chan image.Image)

	Queue = NewVideoQueue()
)

func RunVideoReceiver() {
	for {
		data, ok := <-RecieveChan
		if !ok {
			continue
		}
		if data.Img == nil {
			continue
		}
		Queue.Add(data)
	}
}

func RunVideoSync() {
	for {
		// d := Queue.Get()
		// if d == nil {
		// 	continue
		// }
		// if d.Img == nil {
		// 	continue
		// }
		d := <-RecieveChan
		OutputChan <- d.Img
	}
}

func SendToDecoder(pkt *astiav.Packet) {
	if pkt.StreamIndex() == decoder.VideoIndex() {
		decoder.DecodeVideo(pkt)
	} else if pkt.StreamIndex() == decoder.AudioIndex() {
		decoder.DecodeAudio(pkt)
	}
	pkt.Free()
}
