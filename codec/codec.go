package codec

import (
	"errors"
	"fmt"
	"log"

	"github.com/asticode/go-astiav"
)

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

func (c *Codec) Load(path string) error {
	if err := c.ic.OpenInput(path, nil, nil); err != nil {
		return err
	}

	if err := c.ic.FindStreamInfo(nil); err != nil {
		return err
	}

	var err error
	for _, s := range c.ic.Streams() {
		switch s.CodecParameters().MediaType() {
		case astiav.MediaTypeVideo:
			c.videoidx = s.Index()
			err = c.video.load(s)
		case astiav.MediaTypeAudio:
			c.audioidx = s.Index()
			err = c.audio.load(s)
		}
	}

	return err
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

func (c *Codec) HasAudio() bool {
	return c.audio.hasaudio
}

func (c *Codec) AudioTimebase() astiav.Rational {
	return c.audio.tb
}
