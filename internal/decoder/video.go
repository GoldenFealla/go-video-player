package decoder

import (
	"errors"
	"fmt"
	"image"

	"github.com/asticode/go-astiav"
	"github.com/asticode/go-astikit"
)

var (
	FRAME_BUFFER = 60
)

type FrameData struct {
	img image.Image
	pts int64
}

func (f *FrameData) Image() image.Image {
	return f.img
}

func (f *FrameData) Pts() int64 {
	return f.pts
}

type FrameQueue struct {
	q []*FrameData
}

func NewFrameQueue() *FrameQueue {
	return &FrameQueue{
		q: make([]*FrameData, FRAME_BUFFER),
	}
}

func (q *FrameQueue) Add(d *FrameData) {
	if len(q.q) > FRAME_BUFFER {
		q.q = q.q[1:]
	}
	q.q = append(q.q, d)
}

func (q *FrameQueue) GetData() *FrameData {
	if len(q.q) > 0 {
		frame := q.q[0]
		q.q = q.q[1:]
		return frame
	}

	return nil
}

type VideoDecoder struct {
	closer *astikit.Closer

	stream   *astiav.Stream
	codecCtx *astiav.CodecContext

	decodedFrame *astiav.Frame

	queue *FrameQueue
}

func NewVideoDecoder() (*VideoDecoder, error) {
	// var err error
	d := &VideoDecoder{
		queue: NewFrameQueue(),
	}

	d.closer = astikit.NewCloser()

	d.decodedFrame = astiav.AllocFrame()
	d.closer.Add(d.decodedFrame.Free)

	return d, nil
}

func (vd *VideoDecoder) SetStream(s *astiav.Stream) {
	vd.stream = s
}

func (vd *VideoDecoder) SetCodecContext(cc *astiav.CodecContext) {
	vd.codecCtx = cc
	vd.closer.Add(vd.codecCtx.Free)
}

func (vd *VideoDecoder) UpdateFilter() error {
	return nil
}

func (vd *VideoDecoder) Decode(pkt *astiav.Packet) error {
	if err := vd.codecCtx.SendPacket(pkt); err != nil {
		return fmt.Errorf("video decode: sending packet to audio decoder failed: %w", err)
	}

	for {
		stop, err := vd.decode()
		if err != nil {
			return err
		}

		if stop {
			break
		}
	}

	return nil
}

func (vd *VideoDecoder) decode() (bool, error) {
	if err := vd.codecCtx.ReceiveFrame(vd.decodedFrame); err != nil {
		if errors.Is(err, astiav.ErrEof) || errors.Is(err, astiav.ErrEagain) {
			return true, nil
		}
		return true, fmt.Errorf("video decoding: receiving frame failed: %w", err)
	}

	defer vd.decodedFrame.Unref()

	i, err := vd.decodedFrame.Data().GuessImageFormat()
	if err != nil {
		return false, fmt.Errorf("video decoding: guessing frame data failed: %w", err)
	}

	err = vd.decodedFrame.Data().ToImage(i)
	if err != nil {
		return false, fmt.Errorf("video decoding: getting frame data failed: %w", err)
	}

	frame := &FrameData{
		img: i,
		pts: vd.decodedFrame.Pts(),
	}

	vd.queue.Add(frame)

	// Log
	return false, nil
}
