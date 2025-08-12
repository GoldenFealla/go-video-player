package audiodecoder

import (
	"errors"
	"fmt"
	"io"

	"github.com/asticode/go-astiav"
	"github.com/asticode/go-astikit"
)

var (
	CHANNEL_LAYOUT = astiav.ChannelLayoutStereo
	FORMAT_TYPE    = astiav.SampleFormatFlt
	SAMPLE_RATE    = 44100
	NB_SAMPLES     = 4096
)

var (
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

	w io.Writer
)

func Init(s *astiav.Stream, cc *astiav.CodecContext, dst io.Writer) error {
	var err error

	stream = s
	codecCtx = cc
	w = dst

	closer = astikit.NewCloser()

	decodedFrame = astiav.AllocFrame()
	closer.Add(decodedFrame.Free)

	resampledFrame = astiav.AllocFrame()
	closer.Add(resampledFrame.Free)

	resampledFrame.SetChannelLayout(CHANNEL_LAYOUT)
	resampledFrame.SetSampleFormat(FORMAT_TYPE)
	resampledFrame.SetSampleRate(SAMPLE_RATE)
	resampledFrame.SetNbSamples(NB_SAMPLES)

	filteredFrame = astiav.AllocFrame()
	closer.Add(filteredFrame.Free)

	filteredFrame.SetChannelLayout(resampledFrame.ChannelLayout())
	filteredFrame.SetSampleFormat(resampledFrame.SampleFormat())
	filteredFrame.SetSampleRate(resampledFrame.SampleRate())
	filteredFrame.SetNbSamples(resampledFrame.NbSamples())

	finalFrame = astiav.AllocFrame()
	closer.Add(finalFrame.Free)
	finalFrame.SetChannelLayout(filteredFrame.ChannelLayout())
	finalFrame.SetNbSamples(filteredFrame.NbSamples())
	finalFrame.SetSampleFormat(filteredFrame.SampleFormat())
	finalFrame.SetSampleRate(filteredFrame.SampleRate())
	if err := finalFrame.AllocBuffer(0); err != nil {
		return fmt.Errorf("create audio decoder: allocating final frame buffer failed: %w", err)
	}
	fifo = astiav.AllocAudioFifo(
		finalFrame.SampleFormat(),
		finalFrame.ChannelLayout().Channels(),
		finalFrame.NbSamples(),
	)
	closer.Add(fifo.Free)

	ResamplerCtx = astiav.AllocSoftwareResampleContext()
	closer.Add(ResamplerCtx.Free)

	if filterGraph = astiav.AllocFilterGraph(); filterGraph == nil {
		return errors.New("create audio decoder: graph is nil")
	}
	closer.Add(filterGraph.Free)

	// Filter
	buffersrc := astiav.FindFilterByName("abuffer")
	if buffersrc == nil {
		return errors.New("create audio decoder: buffersrc is nil")
	}

	buffersink := astiav.FindFilterByName("abuffersink")
	if buffersink == nil {
		return errors.New("create audio decoder: buffersink is nil")
	}

	if buffersrcCtx, err = filterGraph.NewBuffersrcFilterContext(buffersrc, "in"); err != nil {
		return fmt.Errorf("create audio decoder: creating buffersrc context failed: %w", err)
	}

	if buffersinkCtx, err = filterGraph.NewBuffersinkFilterContext(buffersink, "in"); err != nil {
		return fmt.Errorf("create audio decoder: creating buffersink context failed: %w", err)
	}

	filterOutputs = astiav.AllocFilterInOut()
	if filterOutputs == nil {
		return errors.New("create audio decoder: outputs is nil")
	}
	closer.Add(filterOutputs.Free)

	filterInputs = astiav.AllocFilterInOut()
	if filterInputs == nil {
		return errors.New("create audio decoder: inputs is nil")
	}
	closer.Add(filterInputs.Free)

	return nil
}

func UpdateFilter() error {
	buffersrcContextParameters := astiav.AllocBuffersrcFilterContextParameters()
	defer buffersrcContextParameters.Free()
	buffersrcContextParameters.SetSampleFormat(resampledFrame.SampleFormat())
	buffersrcContextParameters.SetChannelLayout(resampledFrame.ChannelLayout())
	buffersrcContextParameters.SetSampleRate(resampledFrame.SampleRate())
	buffersrcContextParameters.SetTimeBase(stream.TimeBase())

	if err := buffersrcCtx.SetParameters(buffersrcContextParameters); err != nil {
		return fmt.Errorf("update audio filter: setting buffersrc context parameters failed: %w", err)
	}

	if err := buffersrcCtx.Initialize(nil); err != nil {
		return fmt.Errorf("update audio filter: initializing buffersrc context failed: %w", err)
	}

	filterOutputs.SetName("in")
	filterOutputs.SetFilterContext(buffersrcCtx.FilterContext())
	filterOutputs.SetPadIdx(0)
	filterOutputs.SetNext(nil)

	filterInputs.SetName("out")
	filterInputs.SetFilterContext(buffersinkCtx.FilterContext())
	filterInputs.SetPadIdx(0)
	filterInputs.SetNext(nil)

	if err := filterGraph.Parse("loudnorm,aresample=44100,asetnsamples=n=1024,aformat=sample_fmts=flt:sample_rates=44100:channel_layouts=stereo", filterInputs, filterOutputs); err != nil {
		return fmt.Errorf("update audio filter: parsing filter failed: %w", err)
	}

	if err := filterGraph.Configure(); err != nil {
		return fmt.Errorf("update audio filter: configuring filter failed: %w", err)
	}

	return nil
}

func Decode(pkt *astiav.Packet) error {
	if err := codecCtx.SendPacket(pkt); err != nil {
		return fmt.Errorf("audio decode: sending packet to audio decoder failed: %w", err)
	}

	for {
		stop, err := decode()
		if err != nil {
			return err
		}

		if stop {
			break
		}
	}

	return nil
}

func decode() (bool, error) {
	if err := codecCtx.ReceiveFrame(decodedFrame); err != nil {
		if errors.Is(err, astiav.ErrEof) || errors.Is(err, astiav.ErrEagain) {
			return true, nil
		}
		return false, fmt.Errorf("decode: receiving frame failed: %w", err)
	}

	defer decodedFrame.Unref()

	if err := ResamplerCtx.ConvertFrame(decodedFrame, resampledFrame); err != nil {
		return false, fmt.Errorf("decode: resampling decoded frame failed: %w", err)
	}

	if nbSamples := resampledFrame.NbSamples(); nbSamples > 0 {
		if err := filterFrame(resampledFrame); err != nil {
			return false, err
		}

		if err := flushResampler(false); err != nil {
			return false, fmt.Errorf("decode: flushing software resample context failed: %w", err)
		}
	}

	return false, nil
}

func flushResampler(finalFlush bool) error {
	for {
		if finalFlush || ResamplerCtx.Delay(int64(resampledFrame.SampleRate())) >= int64(resampledFrame.NbSamples()) {
			if err := ResamplerCtx.ConvertFrame(nil, resampledFrame); err != nil {
				return fmt.Errorf("flush resampler: flushing resampler failed: %w", err)
			}

			if err := filterFrame(resampledFrame); err != nil {
				return fmt.Errorf("flush resampler: adding resampled frame to filter failed: %w", err)
			}

			if finalFlush && resampledFrame.NbSamples() == 0 {
				break
			}
			continue
		}
		break
	}

	return nil
}

func filterFrame(f *astiav.Frame) (err error) {
	if err = buffersrcCtx.AddFrame(f, astiav.NewBuffersrcFlags(astiav.BuffersrcFlagKeepRef)); err != nil {
		return fmt.Errorf("filter audio: adding frame failed: %w", err)
	}

	for {
		if stop, err := func() (bool, error) {
			if err := buffersinkCtx.GetFrame(filteredFrame, astiav.NewBuffersinkFlags()); err != nil {
				if errors.Is(err, astiav.ErrEof) || errors.Is(err, astiav.ErrEagain) {
					return true, nil
				}
				return false, fmt.Errorf("filter audio: getting frame failed: %w", err)
			}

			defer filteredFrame.Unref()

			if err := processFifo(false); err != nil {
				return false, err
			}

			data, err := finalFrame.Data().Bytes(1)
			if err != nil {
				return false, err
			}
			_, err = w.Write(data)
			if err != nil {
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

func processFifo(isFlush bool) error {
	if filteredFrame.NbSamples() > 0 {
		if _, err := fifo.Write(filteredFrame); err != nil {
			return fmt.Errorf("process fifo: writing failed: %w", err)
		}
	}

	for {
		if (isFlush && fifo.Size() > 0) || (!isFlush && fifo.Size() >= finalFrame.NbSamples()) {
			n, err := fifo.Read(finalFrame)
			if err != nil {
				return fmt.Errorf("process fifo: reading failed: %w", err)
			}

			finalFrame.SetNbSamples(n)

			_, err = finalFrame.Data().Bytes(0)
			if err != nil {
				return fmt.Errorf("process fifo: get data failed: %w", err)
			}

			continue
		}
		break
	}
	return nil
}

func Index() int {
	return stream.Index()
}

func Free() {
	closer.Close()
}
