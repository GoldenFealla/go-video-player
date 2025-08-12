package media

import (
	"errors"
	"fmt"

	"github.com/GoldenFealla/VideoPlayerGo/internal/media/synchronizer"
	"github.com/asticode/go-astiav"
	"github.com/asticode/go-astikit"
)

func FindCodec(i *astiav.FormatContext, t astiav.MediaType) (*astiav.Stream, *astiav.CodecContext, error) {
	var s *astiav.Stream
	var cc *astiav.CodecContext

	for _, is := range i.Streams() {
		if is.CodecParameters().MediaType() != t {
			continue
		}

		s = is

		c := astiav.FindDecoder(is.CodecParameters().CodecID())
		if c == nil {
			return nil, nil, errors.New("finding codec: codec is nil")
		}

		if cc = astiav.AllocCodecContext(c); cc == nil {
			return nil, nil, errors.New("finding codec: codec context is nil")
		}

		if err := is.CodecParameters().ToCodecContext(cc); err != nil {
			return nil, nil, fmt.Errorf("finding codec: updating codec context failed: %w", err)
		}

		if err := cc.Open(c, nil); err != nil {
			return nil, nil, fmt.Errorf("finding codec: opening codec context failed: %w", err)
		}

		break
	}

	if s == nil {
		return nil, nil, errors.New("finding codec: no stream found")
	}

	return s, cc, nil
}

var (
	closer             *astikit.Closer       = astikit.NewCloser()
	InputFormatContext *astiav.FormatContext = astiav.AllocFormatContext()
)

func Open(input string) error {
	if InputFormatContext == nil {
		InputFormatContext = astiav.AllocFormatContext()
	}

	InputFormatContext.Free()
	if err := InputFormatContext.OpenInput(input, nil, nil); err != nil {
		return fmt.Errorf("decoder: opening input failed: %w", err)
	}
	closer.Add(InputFormatContext.CloseInput)

	if err := InputFormatContext.FindStreamInfo(nil); err != nil {
		return fmt.Errorf("decoder: finding stream info failed: %w", err)
	}

	return nil
}

func Free() {
	closer.Close()
}

func ReadPacket() error {
	pkt := astiav.AllocPacket()
	defer pkt.Free()

	for {
		stop, err := read(pkt)
		if err != nil {
			return err
		}

		if stop {
			break
		}
	}

	return nil
}

func read(pkt *astiav.Packet) (bool, error) {
	if err := InputFormatContext.ReadFrame(pkt); err != nil {
		if errors.Is(err, astiav.ErrEof) {
			return true, nil
		}
		return false, fmt.Errorf("decode: reading frame failed: %w", err)
	}

	defer pkt.Unref()

	synchronizer.SendToDecoder(pkt.Clone())

	return false, nil
}
