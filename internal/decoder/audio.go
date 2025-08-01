package decoder

import (
	"errors"
	"fmt"

	"github.com/asticode/go-astiav"
)

var (
	CHANNEL_LAYOUT = astiav.ChannelLayoutStereo
	FORMAT_TYPE    = astiav.SampleFormatFlt
	SAMPLE_RATE    = 44100
	NB_SAMPLES     = 1024
)

type AudioData struct {
	b []byte
	n int
}

type AudioDecoder struct {
	stream       *astiav.Stream
	codecCtx     *astiav.CodecContext
	ResamplerCtx *astiav.SoftwareResampleContext
	fifo         *astiav.AudioFifo

	decodedFrame   *astiav.Frame
	resampledFrame *astiav.Frame
	finalFrame     *astiav.Frame

	written chan AudioData
}

func NewAudioDecoder() (*AudioDecoder, error) {
	d := &AudioDecoder{}

	d.written = make(chan AudioData)

	d.decodedFrame = astiav.AllocFrame()

	d.resampledFrame = astiav.AllocFrame()
	d.resampledFrame.SetChannelLayout(CHANNEL_LAYOUT)
	d.resampledFrame.SetSampleFormat(FORMAT_TYPE)
	d.resampledFrame.SetSampleRate(SAMPLE_RATE)
	d.resampledFrame.SetNbSamples(NB_SAMPLES)
	if err := d.resampledFrame.AllocBuffer(0); err != nil {
		return nil, fmt.Errorf("create audio decoder: allocating resampled frame buffer failed: %w", err)
	}

	d.finalFrame = astiav.AllocFrame()
	d.finalFrame.SetChannelLayout(d.resampledFrame.ChannelLayout())
	d.finalFrame.SetNbSamples(d.resampledFrame.NbSamples())
	d.finalFrame.SetSampleFormat(d.resampledFrame.SampleFormat())
	d.finalFrame.SetSampleRate(d.resampledFrame.SampleRate())
	if err := d.finalFrame.AllocBuffer(0); err != nil {
		return nil, fmt.Errorf("create audio decoder: allocating final frame buffer failed: %w", err)
	}
	d.fifo = astiav.AllocAudioFifo(
		d.finalFrame.SampleFormat(),
		d.finalFrame.ChannelLayout().Channels(),
		d.finalFrame.NbSamples(),
	)

	d.ResamplerCtx = astiav.AllocSoftwareResampleContext()

	return d, nil
}

func (ad *AudioDecoder) SetStream(s *astiav.Stream) {
	ad.stream = s
}

func (ad *AudioDecoder) SetCodecContext(cc *astiav.CodecContext) {
	ad.codecCtx = cc
}

func (ad *AudioDecoder) Decode(pkt *astiav.Packet) error {
	if err := ad.codecCtx.SendPacket(pkt); err != nil {
		return fmt.Errorf("audio decode: sending packet to audio decoder failed: %w", err)
	}

	for {
		stop, err := ad.decode()
		if err != nil {
			return err
		}

		if stop {
			break
		}
	}

	return nil
}

func (ad *AudioDecoder) decode() (bool, error) {
	if err := ad.codecCtx.ReceiveFrame(ad.decodedFrame); err != nil {
		if errors.Is(err, astiav.ErrEof) || errors.Is(err, astiav.ErrEagain) {
			return true, nil
		}
		return false, fmt.Errorf("decode: receiving frame failed: %w", err)
	}

	defer ad.decodedFrame.Unref()

	if err := ad.ResamplerCtx.ConvertFrame(ad.decodedFrame, ad.resampledFrame); err != nil {
		return false, fmt.Errorf("decode: resampling decoded frame failed: %w", err)
	}

	if nbSamples := ad.resampledFrame.NbSamples(); nbSamples > 0 {
		if err := ad.processFifo(false); err != nil {
			return false, fmt.Errorf("decode: adding resampled frame to audio fifo failed: %w", err)
		}

		if err := ad.flushResampler(false); err != nil {
			return false, fmt.Errorf("decode: flushing software resample context failed: %w", err)
		}
	}

	return false, nil
}

func (ad *AudioDecoder) flushResampler(isFinalFlush bool) error {
	for {
		if !(isFinalFlush && ad.ResamplerCtx.Delay(int64(ad.resampledFrame.SampleRate())) >= int64(ad.resampledFrame.NbSamples())) {
			break
		}

		if err := ad.ResamplerCtx.ConvertFrame(nil, ad.resampledFrame); err != nil {
			return fmt.Errorf("flush resampler: flushing resampler failed: %w", err)
		}

		if err := ad.processFifo(isFinalFlush); err != nil {
			return fmt.Errorf("flush resampler: adding resampled frame to audio fifo failed: %w", err)
		}

		if isFinalFlush && ad.resampledFrame.NbSamples() == 0 {
			break
		}
	}
	return nil
}

func (ad *AudioDecoder) processFifo(isFlush bool) error {
	if ad.resampledFrame.NbSamples() > 0 {
		if _, err := ad.fifo.Write(ad.resampledFrame); err != nil {
			return fmt.Errorf("process fifo: writing failed: %w", err)
		}
	}

	for {
		if (isFlush && ad.fifo.Size() > 0) || (!isFlush && ad.fifo.Size() >= ad.finalFrame.NbSamples()) {
			n, err := ad.fifo.Read(ad.finalFrame)
			if err != nil {
				return fmt.Errorf("process fifo: reading failed: %w", err)
			}

			ad.finalFrame.SetNbSamples(n)

			b, err := ad.finalFrame.Data().Bytes(1)
			if err != nil {
				return fmt.Errorf("process fifo: get data failed: %w", err)
			}

			ad.written <- AudioData{
				b: b,
				n: n,
			}

			continue
		}
		break
	}
	return nil
}

func (ad *AudioDecoder) Free() {
	ad.codecCtx.Free()
	ad.fifo.Free()
	ad.ResamplerCtx.Free()

	ad.decodedFrame.Free()
	ad.resampledFrame.Free()
	ad.finalFrame.Free()
}
