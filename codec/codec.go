package codec

import (
	"errors"
	"fmt"
	"log"

	"github.com/asticode/go-astiav"
)

type AudioMetadata struct {
	Freq     int
	Timebase astiav.Rational
}

type VideoMetadata struct {
	W        int
	H        int
	Timebase astiav.Rational
}

// ====== CODEC ======
type Codec struct {
	ic *astiav.FormatContext

	audio *audiodecoder
	video *videodecoder

	audioidx int
	videoidx int

	AudioBuffer *AudioBuffer
	VideoBuffer *VideoBuffer
}

func NewCodec() *Codec {
	return &Codec{
		ic:          astiav.AllocFormatContext(),
		audio:       newaudiodecoder(),
		video:       newvideodecoder(),
		AudioBuffer: NewAudioBuffer(8),
		VideoBuffer: NewVideoBuffer(2),
	}
}

func (c *Codec) Load(path string) (*VideoMetadata, *AudioMetadata, error) {
	if err := c.ic.OpenInput(path, nil, nil); err != nil {
		return nil, nil, err
	}

	if err := c.ic.FindStreamInfo(nil); err != nil {
		return nil, nil, err
	}

	var err error
	var am *AudioMetadata = nil
	var vm *VideoMetadata = nil

	for _, s := range c.ic.Streams() {
		switch s.CodecParameters().MediaType() {
		case astiav.MediaTypeVideo:
			c.videoidx = s.Index()
			err = c.video.load(s)

			vm = &VideoMetadata{}
			vm.H = s.CodecParameters().Height()
			vm.W = s.CodecParameters().Width()
			vm.Timebase = s.TimeBase()
		case astiav.MediaTypeAudio:
			c.audioidx = s.Index()
			err = c.audio.load(s)

			am = &AudioMetadata{}
			am.Freq = s.CodecParameters().SampleRate()
			am.Timebase = s.TimeBase()
		}
	}

	return vm, am, err
}

func (c *Codec) Parse() {
	pkt := astiav.AllocPacket()
	defer pkt.Free()

	for {
		if stop := func() bool {
			if err := c.ic.ReadFrame(pkt); err != nil {
				if !errors.Is(err, astiav.ErrEof) {
					log.Println(fmt.Errorf("demux: reading frame failed: %w", err))
				}
			}

			defer pkt.Unref()

			switch idx := pkt.StreamIndex(); idx {
			case c.videoidx:
				c.video.decode(pkt, c.VideoBuffer)
			case c.audioidx:
				c.audio.decode(pkt, c.AudioBuffer)
			default:
			}

			return false
		}(); stop {
			break
		}
	}
}

func (c *Codec) Duration() int64 {
	return c.ic.Duration()
}

func (c *Codec) Seek() {
	c.ic.SeekFrame(c.videoidx, 0, astiav.NewSeekFlags(astiav.SeekFlagBackward))
}
