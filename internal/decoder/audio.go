package decoder

import (
	"errors"
	"fmt"

	"github.com/asticode/go-astiav"
	"github.com/asticode/go-astikit"
)

var (
	CHANNEL_LAYOUT = astiav.ChannelLayoutStereo
	FORMAT_TYPE    = astiav.SampleFormatFlt
	SAMPLE_RATE    = 44100
	NB_SAMPLES     = 4096
)

type AudioData struct {
	b []byte
	n int
}

type AudioDecoder struct {
	closer *astikit.Closer

	stream       *astiav.Stream
	codecCtx     *astiav.CodecContext
	ResamplerCtx *astiav.SoftwareResampleContext
	fifo         *astiav.AudioFifo

	decodedFrame   *astiav.Frame
	resampledFrame *astiav.Frame
	filteredFrame  *astiav.Frame
	finalFrame     *astiav.Frame

	filterGraph   *astiav.FilterGraph
	filterInputs  *astiav.FilterInOut
	filterOutputs *astiav.FilterInOut
	buffersinkCtx *astiav.BuffersinkFilterContext
	buffersrcCtx  *astiav.BuffersrcFilterContext

	written chan AudioData
}

func NewAudioDecoder() (*AudioDecoder, error) {
	var err error
	d := &AudioDecoder{}

	d.written = make(chan AudioData)

	d.closer = astikit.NewCloser()

	d.decodedFrame = astiav.AllocFrame()
	d.closer.Add(d.decodedFrame.Free)

	d.resampledFrame = astiav.AllocFrame()
	d.closer.Add(d.resampledFrame.Free)

	d.resampledFrame.SetChannelLayout(CHANNEL_LAYOUT)
	d.resampledFrame.SetSampleFormat(FORMAT_TYPE)
	d.resampledFrame.SetSampleRate(SAMPLE_RATE)
	d.resampledFrame.SetNbSamples(NB_SAMPLES)

	d.filteredFrame = astiav.AllocFrame()
	d.closer.Add(d.filteredFrame.Free)

	d.filteredFrame.SetChannelLayout(d.resampledFrame.ChannelLayout())
	d.filteredFrame.SetSampleFormat(d.resampledFrame.SampleFormat())
	d.filteredFrame.SetSampleRate(d.resampledFrame.SampleRate())
	d.filteredFrame.SetNbSamples(d.resampledFrame.NbSamples())

	d.finalFrame = astiav.AllocFrame()
	d.closer.Add(d.finalFrame.Free)
	d.finalFrame.SetChannelLayout(d.filteredFrame.ChannelLayout())
	d.finalFrame.SetNbSamples(d.filteredFrame.NbSamples())
	d.finalFrame.SetSampleFormat(d.filteredFrame.SampleFormat())
	d.finalFrame.SetSampleRate(d.filteredFrame.SampleRate())
	if err := d.finalFrame.AllocBuffer(0); err != nil {
		return nil, fmt.Errorf("create audio decoder: allocating final frame buffer failed: %w", err)
	}
	d.fifo = astiav.AllocAudioFifo(
		d.finalFrame.SampleFormat(),
		d.finalFrame.ChannelLayout().Channels(),
		d.finalFrame.NbSamples(),
	)
	d.closer.Add(d.fifo.Free)

	d.ResamplerCtx = astiav.AllocSoftwareResampleContext()
	d.closer.Add(d.ResamplerCtx.Free)

	if d.filterGraph = astiav.AllocFilterGraph(); d.filterGraph == nil {
		return nil, errors.New("create audio decoder: graph is nil")
	}
	d.closer.Add(d.filterGraph.Free)

	// Filter
	buffersrc := astiav.FindFilterByName("abuffer")
	if buffersrc == nil {
		return nil, errors.New("create audio decoder: buffersrc is nil")
	}

	buffersink := astiav.FindFilterByName("abuffersink")
	if buffersink == nil {
		return nil, errors.New("create audio decoder: buffersink is nil")
	}

	if d.buffersrcCtx, err = d.filterGraph.NewBuffersrcFilterContext(buffersrc, "in"); err != nil {
		return nil, fmt.Errorf("create audio decoder: creating buffersrc context failed: %w", err)
	}

	if d.buffersinkCtx, err = d.filterGraph.NewBuffersinkFilterContext(buffersink, "in"); err != nil {
		return nil, fmt.Errorf("create audio decoder: creating buffersink context failed: %w", err)
	}

	d.filterOutputs = astiav.AllocFilterInOut()
	if d.filterOutputs == nil {
		return nil, errors.New("create audio decoder: outputs is nil")
	}
	d.closer.Add(d.filterOutputs.Free)

	d.filterInputs = astiav.AllocFilterInOut()
	if d.filterInputs == nil {
		return nil, errors.New("create audio decoder: inputs is nil")
	}
	d.closer.Add(d.filterInputs.Free)

	return d, nil
}

func (ad *AudioDecoder) SetStream(s *astiav.Stream) {
	ad.stream = s
}

func (ad *AudioDecoder) SetCodecContext(cc *astiav.CodecContext) {
	ad.codecCtx = cc
	ad.closer.Add(ad.codecCtx.Free)

	ad.decodedFrame.SetChannelLayout(cc.ChannelLayout())
	ad.decodedFrame.SetSampleFormat(cc.SampleFormat())
	ad.decodedFrame.SetSampleRate(cc.SampleRate())
}

func (ad *AudioDecoder) UpdateFilter() error {
	buffersrcContextParameters := astiav.AllocBuffersrcFilterContextParameters()
	defer buffersrcContextParameters.Free()
	buffersrcContextParameters.SetSampleFormat(ad.resampledFrame.SampleFormat())
	buffersrcContextParameters.SetChannelLayout(ad.resampledFrame.ChannelLayout())
	buffersrcContextParameters.SetSampleRate(ad.resampledFrame.SampleRate())
	buffersrcContextParameters.SetTimeBase(ad.stream.TimeBase())

	if err := ad.buffersrcCtx.SetParameters(buffersrcContextParameters); err != nil {
		err = fmt.Errorf("update audio filter: setting buffersrc context parameters failed: %w", err)
	}

	if err := ad.buffersrcCtx.Initialize(nil); err != nil {
		err = fmt.Errorf("update audio filter: initializing buffersrc context failed: %w", err)
	}

	ad.filterOutputs.SetName("in")
	ad.filterOutputs.SetFilterContext(ad.buffersrcCtx.FilterContext())
	ad.filterOutputs.SetPadIdx(0)
	ad.filterOutputs.SetNext(nil)

	ad.filterInputs.SetName("out")
	ad.filterInputs.SetFilterContext(ad.buffersinkCtx.FilterContext())
	ad.filterInputs.SetPadIdx(0)
	ad.filterInputs.SetNext(nil)

	if err := ad.filterGraph.Parse("loudnorm,aresample=44100,asetnsamples=n=1024,aformat=sample_fmts=flt:sample_rates=44100:channel_layouts=stereo", ad.filterInputs, ad.filterOutputs); err != nil {
		return fmt.Errorf("update audio filter: parsing filter failed: %w", err)
	}

	if err := ad.filterGraph.Configure(); err != nil {
		return fmt.Errorf("update audio filter: configuring filter failed: %w", err)
	}

	return nil
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
		if err := ad.filterFrame(ad.resampledFrame); err != nil {
			return false, err
		}

		if err := ad.flushResampler(false); err != nil {
			return false, fmt.Errorf("decode: flushing software resample context failed: %w", err)
		}
	}

	return false, nil
}

func (ad *AudioDecoder) flushResampler(finalFlush bool) error {
	for {
		if finalFlush || ad.ResamplerCtx.Delay(int64(ad.resampledFrame.SampleRate())) >= int64(ad.resampledFrame.NbSamples()) {
			if err := ad.ResamplerCtx.ConvertFrame(nil, ad.resampledFrame); err != nil {
				return fmt.Errorf("flush resampler: flushing resampler failed: %w", err)
			}

			if err := ad.filterFrame(ad.resampledFrame); err != nil {
				return fmt.Errorf("flush resampler: adding resampled frame to filter failed: %w", err)
			}

			if finalFlush && ad.resampledFrame.NbSamples() == 0 {
				break
			}
			continue
		}
		break
	}

	return nil
}

func (ad *AudioDecoder) filterFrame(f *astiav.Frame) (err error) {
	if err = ad.buffersrcCtx.AddFrame(f, astiav.NewBuffersrcFlags(astiav.BuffersrcFlagKeepRef)); err != nil {
		return fmt.Errorf("filter audio: adding frame failed: %w", err)
	}

	for {
		if stop, err := func() (bool, error) {
			if err := ad.buffersinkCtx.GetFrame(ad.filteredFrame, astiav.NewBuffersinkFlags()); err != nil {
				if errors.Is(err, astiav.ErrEof) || errors.Is(err, astiav.ErrEagain) {
					return true, nil
				}
				return false, fmt.Errorf("filter audio: getting frame failed: %w", err)
			}

			defer ad.filteredFrame.Unref()

			if err := ad.processFifo(false); err != nil {
				return false, err
			}

			return false, nil
		}(); err != nil {
			return err
		} else if stop {
			break
		}

	}
	return
}

func (ad *AudioDecoder) processFifo(isFlush bool) error {
	if ad.filteredFrame.NbSamples() > 0 {
		if _, err := ad.fifo.Write(ad.filteredFrame); err != nil {
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

			b, err := ad.finalFrame.Data().Bytes(0)
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
	ad.closer.Close()
}
