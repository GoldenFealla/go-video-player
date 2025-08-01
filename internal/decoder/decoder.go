package decoder

import (
	"errors"
	"fmt"

	"github.com/asticode/go-astiav"
)

type MediaDecoder interface {
	SetStream(s *astiav.Stream)
	SetCodecContext(cc *astiav.CodecContext)
}

func FindCodec(i *astiav.FormatContext, t astiav.MediaType, dst MediaDecoder) error {
	var s *astiav.Stream
	var cc *astiav.CodecContext

	for _, is := range i.Streams() {
		if is.CodecParameters().MediaType() != t {
			continue
		}

		s = is

		c := astiav.FindDecoder(is.CodecParameters().CodecID())
		if c == nil {
			return errors.New("finding codec: codec is nil")
		}

		if cc = astiav.AllocCodecContext(c); cc == nil {
			return errors.New("finding codec: codec context is nil")
		}

		if err := is.CodecParameters().ToCodecContext(cc); err != nil {
			return fmt.Errorf("finding codec: updating codec context failed: %w", err)
		}

		if err := cc.Open(c, nil); err != nil {
			return fmt.Errorf("finding codec: opening codec context failed: %w", err)
		}

		break
	}

	if s == nil {
		return errors.New("finding codec: no stream found")
	}

	dst.SetStream(s)
	dst.SetCodecContext(cc)

	return nil
}

type Decoder struct {
	inputFormatCtx *astiav.FormatContext

	// vd *VideoDecoder
	ad *AudioDecoder
}

func NewDecoder(ad *AudioDecoder) (*Decoder, error) {
	d := &Decoder{
		ad: ad,
	}

	d.inputFormatCtx = astiav.AllocFormatContext()
	if d.inputFormatCtx == nil {
		return nil, errors.New("decoder: input format context is nil")
	}

	return d, nil
}

func (d *Decoder) Open(input string) error {
	if err := d.inputFormatCtx.OpenInput(input, nil, nil); err != nil {
		return fmt.Errorf("decoder: opening input failed: %w", err)
	}
	// defer d.inputFormatCtx.CloseInput()

	if err := d.inputFormatCtx.FindStreamInfo(nil); err != nil {
		return fmt.Errorf("decoder: finding stream info failed: %w", err)
	}

	if err := FindCodec(d.inputFormatCtx, astiav.MediaTypeAudio, d.ad); err != nil {
		return fmt.Errorf("decoder: finding audio codec failed: %w", err)
	}

	return nil
}

func (d *Decoder) Decode() error {
	pkt := astiav.AllocPacket()
	defer pkt.Free()

	for {
		stop, err := d.decode(pkt)
		if err != nil {
			return err
		}

		if stop {
			break
		}
	}

	return nil
}

func (d *Decoder) decode(pkt *astiav.Packet) (bool, error) {
	if err := d.inputFormatCtx.ReadFrame(pkt); err != nil {
		if errors.Is(err, astiav.ErrEof) {
			return true, nil
		}
		return true, fmt.Errorf("decode: reading frame failed: %w", err)
	}

	defer pkt.Unref()

	if pkt.StreamIndex() == d.ad.stream.Index() {
		if err := d.ad.Decode(pkt); err != nil {
			return false, err
		}
	}

	return false, nil
}

func (d *Decoder) Free() {
	d.inputFormatCtx.Free()
}
