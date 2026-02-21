package player

import (
	"errors"
	"fmt"
	"log"

	"github.com/asticode/go-astiav"
)

type Player struct {
	Clock *Clock

	vDecoder *VideoDecoder
	aDecoder *AudioDecoder

	VideoIn chan VideoFrame
	AudioIn chan AudioFrame

	VideoOut chan VideoFrame
	AudioOut chan AudioFrame
}

func NewPlayer() *Player {
	return &Player{
		Clock: &Clock{},

		vDecoder: NewVideoDecoder(),
		aDecoder: NewAudioDecoder(),

		VideoIn:  make(chan VideoFrame, 8),
		VideoOut: make(chan VideoFrame, 8),
		AudioOut: make(chan AudioFrame, 1024),
	}
}

func (p *Player) Play(path string) error {
	ic := astiav.AllocFormatContext()
	if ic == nil {
		return fmt.Errorf("alloc format context failed")
	}
	defer ic.Free()

	if err := ic.OpenInput(path, nil, nil); err != nil {
		return err
	}
	defer ic.CloseInput()

	if err := ic.FindStreamInfo(nil); err != nil {
		return err
	}

	// Find Codec
	for _, s := range ic.Streams() {
		switch s.CodecParameters().MediaType() {
		case astiav.MediaTypeVideo:
			p.vDecoder.Load(s)
		case astiav.MediaTypeAudio:
			p.aDecoder.Load(s)
		}
	}

	// Stream packet
	pkt := astiav.AllocPacket()
	defer pkt.Free()

	go VideoSync(p.Clock, p.VideoIn, p.VideoOut)

	for {
		if stop := func() bool {
			if err := ic.ReadFrame(pkt); err != nil {
				if !errors.Is(err, astiav.ErrEof) {
					log.Println(fmt.Errorf("demux: reading frame failed: %w", err))
				}
				// Return Eof
			}

			defer pkt.Unref()

			switch idx := pkt.StreamIndex(); idx {
			case p.vDecoder.stream.Index():
				p.vDecoder.Decode(pkt, p.VideoIn)
			case p.aDecoder.stream.Index():
				p.aDecoder.Decode(pkt, p.AudioOut)
			default:
			}

			return false
		}(); stop {
			break
		}
	}

	return nil
}
