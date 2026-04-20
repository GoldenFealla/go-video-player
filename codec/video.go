package codec

import (
	"errors"
	"fmt"
	"log"

	"github.com/asticode/go-astiav"
	"github.com/asticode/go-astikit"
)

type videodecoder struct {
	closer *astikit.Closer

	ctx *astiav.CodecContext

	has      bool
	timebase astiav.Rational
}

func newvideodecoder() *videodecoder {
	vd := &videodecoder{}

	vd.closer = astikit.NewCloser()

	return vd
}

func (vd *videodecoder) close() {
	vd.closer.Close()
}

func (vd *videodecoder) load(stream *astiav.Stream) error {
	codec := astiav.FindDecoder(stream.CodecParameters().CodecID())
	if codec == nil {
		return errors.New("video decoder: codec is nil")
	}

	vd.ctx = astiav.AllocCodecContext(codec)
	if vd.ctx == nil {
		return errors.New("video decoder: codec context is nil")
	}
	// vd.closer.Add(vd.ctx.Free)

	err := stream.CodecParameters().ToCodecContext(vd.ctx)
	if err != nil {
		return fmt.Errorf("video decoder: updating codec context failed: %w", err)
	}

	err = vd.ctx.Open(codec, nil)
	if err != nil {
		return fmt.Errorf("video decoder: opening codec context failed: %w", err)
	}

	vd.timebase = stream.TimeBase()
	log.Printf("video timebase %v\n", vd.timebase)
	vd.has = true
	return nil
}

func (vd *videodecoder) decode(pkt *astiav.Packet, vBuffer *VideoBuffer) error {
	if vd.ctx == nil {
		return errors.New("decoder context is nil")
	}

	f := astiav.AllocFrame()
	defer f.Free()

	err := vd.ctx.SendPacket(pkt)
	if err != nil {
		if errors.Is(err, astiav.ErrEof) || errors.Is(err, astiav.ErrEagain) {
			return nil
		}
		log.Println(fmt.Errorf("video decode: sending packet failed: %w", err))
		return err
	}

	for {
		if stop := func() bool {
			if err := vd.ctx.ReceiveFrame(f); err != nil {
				if errors.Is(err, astiav.ErrEagain) {
					// log.Println(fmt.Errorf("video decode eagain: %w", err))
				} else if errors.Is(err, astiav.ErrEof) {
					log.Println(fmt.Errorf("video decode eof: %w", err))
				} else {
					log.Println(fmt.Errorf("video decode: receiving frame failed: %w", err))
				}

				return true
			}

			defer f.Unref()

			pts := float64(f.Pts()) * vd.timebase.Float64()
			buf, _ := f.Data().Bytes(1)
			vBuffer.Push(VideoData{
				PTS:  pts,
				W:    f.Width(),
				H:    f.Height(),
				Data: buf,
			})

			return false
		}(); stop {
			break
		}
	}

	return nil
}
