package decoder

import (
	"errors"
	"fmt"
	"io"
	"log"

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

type AudioDecoder struct {
	stream              *astiav.Stream
	codecCtx            *astiav.CodecContext
	softwareResampleCtx *astiav.SoftwareResampleContext
	fifo                *astiav.AudioFifo

	decodedFrame   *astiav.Frame
	resampledFrame *astiav.Frame
	finalFrame     *astiav.Frame

	read chan bool
}

func NewAudioDecoder() {
	//TODO: implementation
}

func (ad *AudioDecoder) SetStream(s *astiav.Stream) {
	ad.stream = s
}

func (ad *AudioDecoder) SetCodecContext(cc *astiav.CodecContext) {
	ad.codecCtx = cc
}

func (ad *AudioDecoder) Decode() {
	for {
		if stop := func() bool {
			if err := ad.codecCtx.ReceiveFrame(ad.decodedFrame); err != nil {
				if errors.Is(err, astiav.ErrEof) || errors.Is(err, astiav.ErrEagain) {
					return true
				}
				log.Fatal(fmt.Errorf("main: receiving frame failed: %w", err))
			}

			defer ad.decodedFrame.Unref()

			if err := ad.softwareResampleCtx.ConvertFrame(ad.decodedFrame, ad.resampledFrame); err != nil {
				log.Fatal(fmt.Errorf("main: resampling decoded frame failed: %w", err))
			}

			if nbSamples := ad.resampledFrame.NbSamples(); nbSamples > 0 {
				// Add resampled frame to audio fifo
				// if err := addResampledFrameToAudioFIFO(false); err != nil {
				// 	log.Fatal(fmt.Errorf("main: adding resampled frame to audio fifo failed: %w", err))
				// }

				// Flush software resample context
				// if err := flushSoftwareResampleContext(false); err != nil {
				// 	log.Fatal(fmt.Errorf("main: flushing software resample context failed: %w", err))
				// }
			}
			return false
		}(); stop {
			break
		}
	}
}

func (ad *AudioDecoder) flushSoftwareResampleContext(finalFlush bool) error {
	// Loop
	for {
		// We're making the final flush or there's enough data to flush the resampler
		if finalFlush || ad.softwareResampleCtx.Delay(int64(ad.resampledFrame.SampleRate())) >= int64(ad.resampledFrame.NbSamples()) {
			// Flush resampler
			if err := ad.softwareResampleCtx.ConvertFrame(nil, ad.resampledFrame); err != nil {
				log.Fatal(fmt.Errorf("main: flushing resampler failed: %w", err))
			}

			// Log
			if ad.resampledFrame.NbSamples() > 0 {
				log.Printf("new resampled frame: nb samples: %d", ad.resampledFrame.NbSamples())
			}

			// Add resampled frame to audio fifo
			if err := ad.addResampledFrameToAudioFIFO(finalFlush); err != nil {
				log.Fatal(fmt.Errorf("main: adding resampled frame to audio fifo failed: %w", err))
			}

			// Final flush is done
			if finalFlush && ad.resampledFrame.NbSamples() == 0 {
				break
			}
			continue
		}
		break
	}
	return nil
}

func (ad *AudioDecoder) addResampledFrameToAudioFIFO(flush bool) error {
	if ad.resampledFrame.NbSamples() > 0 {
		if _, err := ad.fifo.Write(ad.resampledFrame); err != nil {
			return fmt.Errorf("main: writing failed: %w", err)
		}
	}

	// Loop
	for {
		// We're flushing or there's enough data to read
		if (flush && ad.fifo.Size() > 0) || (!flush && ad.fifo.Size() >= ad.finalFrame.NbSamples()) {
			// Read
			n, err := ad.fifo.Read(ad.finalFrame)
			if err != nil {
				return fmt.Errorf("main: reading failed: %w", err)
			}
			ad.finalFrame.SetNbSamples(n)

			// Log
			log.Printf("new final frame: nb samples: %d", ad.finalFrame.NbSamples())
			continue
		}
		break
	}
	return nil
}

func (ad *AudioDecoder) Free() {
	ad.codecCtx.Free()
	ad.fifo.Free()

	ad.decodedFrame.Free()
	ad.resampledFrame.Free()
}

type VideoDecoder struct {
	cr chan bool
}

func NewVideoDecoder() {
	//TODO: implementation
}

type Synchronizer struct {
	ad *AudioDecoder
	vd *VideoDecoder

	vw io.Writer
	aw io.Writer
}

func NewSynchronizer(vw io.Writer, aw io.Writer) *Synchronizer {
	return &Synchronizer{
		vw: vw,
		aw: aw,
	}
}

func (s *Synchronizer) Run(callback func()) {
	for {
		_, ok := <-s.ad.read
		if !ok {
			break
		}
		// Do something
	}
}
