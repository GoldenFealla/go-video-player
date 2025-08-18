package decoder

import (
	"errors"
	"fmt"

	"github.com/asticode/go-astiav"
	"github.com/asticode/go-astikit"
)

type VideoStream struct {
	st *astiav.Stream
	cc *astiav.CodecContext

	df *astiav.Frame

	closer *astikit.Closer

	outputCallback func(*astiav.Frame)
}

func NewVideoStream() *VideoStream {
	vst := &VideoStream{
		closer: astikit.NewCloser(),
	}

	vst.df = astiav.AllocFrame()
	vst.closer.Add(vst.df.Free)

	return vst
}

func (vst *VideoStream) Close() {
	vst.closer.Close()
}

func (vst *VideoStream) Index() int {
	return vst.st.Index()
}

func (vst *VideoStream) SetOutoutCallback(callback func(*astiav.Frame)) {
	vst.outputCallback = callback
}

func (vst *VideoStream) LoadInputContext(i *astiav.FormatContext) error {
	if i == nil {
		return ErrInputContextNil
	}

	for _, is := range i.Streams() {
		if is.CodecParameters().MediaType() != astiav.MediaTypeVideo {
			continue
		}

		vst.st = is

		codec := astiav.FindDecoder(is.CodecParameters().CodecID())
		if codec == nil {
			return errors.New("finding video codec: codec is nil")
		}

		if vst.cc = astiav.AllocCodecContext(codec); vst.cc == nil {
			return errors.New("finding video codec: codec context is nil")
		}
		vst.closer.Add(vst.cc.Free)

		if err := is.CodecParameters().ToCodecContext(vst.cc); err != nil {
			return fmt.Errorf("finding video codec: updating codec context failed: %w", err)
		}

		if err := vst.cc.Open(codec, nil); err != nil {
			return fmt.Errorf("finding video codec: opening codec context failed: %w", err)
		}

		break
	}

	if vst.st == nil {
		return ErrNoVideo
	}

	return nil
}

func (vst *VideoStream) Decode(pkt *astiav.Packet) error {
	if err := vst.cc.SendPacket(pkt); err != nil {
		return fmt.Errorf("video decode: sending packet to audio decoder failed: %w", err)
	}

	for {
		stop, err := vst.decode()
		if err != nil {
			return err
		}

		if stop {
			return nil
		}
	}
}

func (vst *VideoStream) decode() (bool, error) {
	if err := vst.cc.ReceiveFrame(vst.df); err != nil {
		if errors.Is(err, astiav.ErrEof) || errors.Is(err, astiav.ErrEagain) {
			return true, nil
		}
		return true, fmt.Errorf("video decoding: receiving frame failed: %w", err)
	}

	defer vst.df.Unref()

	vst.outputCallback(vst.df.Clone())

	return false, nil
}
