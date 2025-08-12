package videodecoder

import (
	"errors"
	"fmt"
	"image"

	"github.com/asticode/go-astiav"
	"github.com/asticode/go-astikit"
)

// var (
// 	FRAME_BUFFER = 60
// )

// type FrameData struct {
// 	img image.Image
// 	pts int64
// }

// func (f *FrameData) Image() image.Image {
// 	return f.img
// }

// func (f *FrameData) Pts() int64 {
// 	return f.pts
// }

// type FrameQueue struct {
// 	q []*FrameData
// }

// func NewFrameQueue() *FrameQueue {
// 	return &FrameQueue{
// 		q: make([]*FrameData, FRAME_BUFFER),
// 	}
// }

// func (q *FrameQueue) Add(d *FrameData) {
// 	if len(q.q) > FRAME_BUFFER {
// 		q.q = q.q[1:]
// 	}
// 	q.q = append(q.q, d)
// }

// func (q *FrameQueue) GetData() *FrameData {
// 	if len(q.q) > 0 {
// 		frame := q.q[0]
// 		q.q = q.q[1:]
// 		return frame
// 	}

// 	return nil
// }

var (
	closer *astikit.Closer = astikit.NewCloser()

	stream   *astiav.Stream
	codecCtx *astiav.CodecContext

	decodedFrame *astiav.Frame

	outputChan chan image.Image
	// queue *FrameQueue
)

func Init(s *astiav.Stream, cc *astiav.CodecContext, dst chan image.Image) error {
	stream = s
	codecCtx = cc
	outputChan = dst

	closer = astikit.NewCloser()

	decodedFrame = astiav.AllocFrame()
	closer.Add(decodedFrame.Free)

	return nil
}

func UpdateFilter() error {
	return nil
}

func Decode(pkt *astiav.Packet) error {
	if err := codecCtx.SendPacket(pkt); err != nil {
		return fmt.Errorf("video decode: sending packet to audio decoder failed: %w", err)
	}

	for {
		stop, err := decode()
		if err != nil {
			return err
		}

		if stop {
			break
		}
	}

	return nil
}

func decode() (bool, error) {
	if err := codecCtx.ReceiveFrame(decodedFrame); err != nil {
		if errors.Is(err, astiav.ErrEof) || errors.Is(err, astiav.ErrEagain) {
			return true, nil
		}
		return true, fmt.Errorf("video decoding: receiving frame failed: %w", err)
	}

	defer decodedFrame.Unref()

	i, err := decodedFrame.Data().GuessImageFormat()
	if err != nil {
		return false, err
	}

	err = decodedFrame.Data().ToImage(i)
	if err != nil {
		return false, err
	}

	select {
	case outputChan <- i:
	default:
	}

	return false, nil
}

func Index() int {
	return stream.Index()
}

func Free() {
	closer.Close()
}
