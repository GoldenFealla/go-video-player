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

	timebase astiav.Rational
	Stopped  bool
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

	c.timebase = vm.Timebase

	return vm, am, err
}

var video_decode_counter int = 0
var audio_decode_counter int = 0

func (c *Codec) Parse(quit chan struct{}) {
	pkt := astiav.AllocPacket()
	defer pkt.Free()

	c.Stopped = false

	for {
		if stop := func() bool {
			if err := c.ic.ReadFrame(pkt); err != nil {
				if !errors.Is(err, astiav.ErrEof) {
					log.Println(fmt.Errorf("demux: reading frame failed: %w", err))
				} else {
					log.Println("End of file")
				}

				return true
			}

			defer pkt.Unref()

			switch idx := pkt.StreamIndex(); idx {
			case c.videoidx:
				c.video.decode(pkt, c.VideoBuffer)
				video_decode_counter += 1
			case c.audioidx:
				c.audio.decode(pkt, c.AudioBuffer)
				audio_decode_counter += 1
			default:
			}

			return false
		}(); stop {
			break
		}
	}

	c.Stopped = true
	quit <- struct{}{}
}

func (c *Codec) Duration() int64 {
	return c.ic.Duration()
}

func (c *Codec) SeekSecond(second float32) {
	c.AudioBuffer.Clear()
	c.VideoBuffer.Clear()

	timestamp := int64(second / float32(c.timebase.Float64()))
	err := c.ic.SeekFrame(c.videoidx, timestamp, astiav.NewSeekFlags(astiav.SeekFlagBackward))

	if err != nil {
		log.Println(err)
	}
}
