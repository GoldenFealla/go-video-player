package codec

import (
	"errors"
	"fmt"
	"log"

	"github.com/asticode/go-astiav"
	"github.com/asticode/go-astikit"
)

type audiodecoder struct {
	closer *astikit.Closer
	ctx    *astiav.CodecContext
	src    *astiav.SoftwareResampleContext

	f *astiav.Frame
	r *astiav.Frame

	hasaudio bool
	tb       astiav.Rational
}

func newaudiodecoder() *audiodecoder {
	ad := &audiodecoder{}
	ad.closer = astikit.NewCloser()

	ad.src = astiav.AllocSoftwareResampleContext()
	ad.closer.Add(ad.src.Free)

	return ad
}

func (ad *audiodecoder) close() {
	ad.closer.Close()
}

func (ad *audiodecoder) load(stream *astiav.Stream) error {
	codec := astiav.FindDecoder(stream.CodecParameters().CodecID())
	if codec == nil {
		return errors.New("audio decoder: codec is nil")
	}

	ad.ctx = astiav.AllocCodecContext(codec)
	if ad.ctx == nil {
		return errors.New("audio decoder: codec context is nil")
	}
	ad.closer.Add(ad.ctx.Free)

	err := stream.CodecParameters().ToCodecContext(ad.ctx)
	if err != nil {
		return fmt.Errorf("audio decoder: updating codec context failed: %w", err)
	}

	err = ad.ctx.Open(codec, nil)
	if err != nil {
		return fmt.Errorf("audio decoder: opening codec context failed: %w", err)
	}

	ad.f = astiav.AllocFrame()
	ad.closer.Add(ad.f.Free)

	ad.r = astiav.AllocFrame()
	ad.closer.Add(ad.r.Free)

	ad.hasaudio = true
	ad.tb = stream.TimeBase()
	log.Printf("audio timebase %v\n", ad.tb)

	return nil
}

func (ad *audiodecoder) decode(pkt *astiav.Packet, aBuffer *AudioBuffer) error {
	ad.r.SetSampleFormat(astiav.SampleFormatS16)
	ad.r.SetChannelLayout(astiav.ChannelLayoutStereo)
	ad.r.SetSampleRate(44100)

	if err := ad.ctx.SendPacket(pkt); err != nil {
		log.Println(fmt.Errorf("audio decode: sending packet failed: %w", err))
	}
	// ad.decodeloop(aBuffer)
	for {
		if stop := func() bool {
			if err := ad.ctx.ReceiveFrame(ad.f); err != nil {
				if !errors.Is(err, astiav.ErrEof) && !errors.Is(err, astiav.ErrEagain) {
					log.Println(fmt.Errorf("audio decode: receiving frame failed: %w", err))
				}
				return false
			}

			defer ad.f.Unref()
			// defer ad.r.Unref()

			if err := ad.src.ConvertFrame(ad.f, ad.r); err != nil {
				log.Println(fmt.Errorf("audio decode: resampling decoded frame failed: %w", err))
				return false
			}

			if nbSamples := ad.r.NbSamples(); nbSamples > 0 {
				src, _ := ad.r.Data().Bytes(1)

				aBuffer.Push(AudioData{
					PTS:     float64(ad.f.Pts()) * ad.tb.Float64(),
					Samples: src,
				})
			}

			return true
		}(); stop {
			break
		}
	}

	return nil
}
