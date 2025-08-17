package decoder

import (
	"errors"
	"fmt"
	"io"

	"github.com/asticode/go-astiav"
	"github.com/asticode/go-astikit"
)

var (
	audioStream       *astiav.Stream
	audioCodecCtx     *astiav.CodecContext
	audioResamplerCtx *astiav.SoftwareResampleContext

	audioFifo *astiav.AudioFifo

	audioDecoded   *astiav.Frame
	audioResampled *astiav.Frame
	audioFiltered  *astiav.Frame
	audioCompleted *astiav.Frame

	audioFilterGraph   *astiav.FilterGraph
	audioFilterInputs  *astiav.FilterInOut
	audioFilterOutputs *astiav.FilterInOut

	audioBuffersinkCtx *astiav.BuffersinkFilterContext
	audioBuffersrcCtx  *astiav.BuffersrcFilterContext

	aw  io.Writer
	Cur float64
)

func InitAudio(s *astiav.Stream, cc *astiav.CodecContext, dst io.Writer) error {
	var err error

	audioStream = s
	audioCodecCtx = cc
	aw = dst

	closer = astikit.NewCloser()

	audioDecoded = astiav.AllocFrame()
	closer.Add(audioDecoded.Free)

	audioResampled = astiav.AllocFrame()
	closer.Add(audioResampled.Free)

	audioResampled.SetChannelLayout(CHANNEL_LAYOUT)
	audioResampled.SetSampleFormat(FORMAT_TYPE)
	audioResampled.SetSampleRate(SAMPLE_RATE)
	audioResampled.SetNbSamples(NB_SAMPLES)

	audioFiltered = astiav.AllocFrame()
	closer.Add(audioFiltered.Free)

	audioFiltered.SetChannelLayout(audioResampled.ChannelLayout())
	audioFiltered.SetSampleFormat(audioResampled.SampleFormat())
	audioFiltered.SetSampleRate(audioResampled.SampleRate())
	audioFiltered.SetNbSamples(audioResampled.NbSamples())

	audioCompleted = astiav.AllocFrame()
	closer.Add(audioCompleted.Free)
	audioCompleted.SetChannelLayout(audioFiltered.ChannelLayout())
	audioCompleted.SetNbSamples(audioFiltered.NbSamples())
	audioCompleted.SetSampleFormat(audioFiltered.SampleFormat())
	audioCompleted.SetSampleRate(audioFiltered.SampleRate())
	if err := audioCompleted.AllocBuffer(0); err != nil {
		return fmt.Errorf("create audio decoder: allocating final frame buffer failed: %w", err)
	}
	audioFifo = astiav.AllocAudioFifo(
		audioCompleted.SampleFormat(),
		audioCompleted.ChannelLayout().Channels(),
		audioCompleted.NbSamples(),
	)
	closer.Add(audioFifo.Free)

	audioResamplerCtx = astiav.AllocSoftwareResampleContext()
	closer.Add(audioResamplerCtx.Free)

	if audioFilterGraph = astiav.AllocFilterGraph(); audioFilterGraph == nil {
		return errors.New("create audio decoder: graph is nil")
	}
	closer.Add(audioFilterGraph.Free)

	// Filter
	buffersrc := astiav.FindFilterByName("abuffer")
	if buffersrc == nil {
		return errors.New("create audio decoder: buffersrc is nil")
	}

	buffersink := astiav.FindFilterByName("abuffersink")
	if buffersink == nil {
		return errors.New("create audio decoder: buffersink is nil")
	}

	if audioBuffersrcCtx, err = audioFilterGraph.NewBuffersrcFilterContext(buffersrc, "in"); err != nil {
		return fmt.Errorf("create audio decoder: creating buffersrc context failed: %w", err)
	}

	if audioBuffersinkCtx, err = audioFilterGraph.NewBuffersinkFilterContext(buffersink, "in"); err != nil {
		return fmt.Errorf("create audio decoder: creating buffersink context failed: %w", err)
	}

	audioFilterOutputs = astiav.AllocFilterInOut()
	if audioFilterOutputs == nil {
		return errors.New("create audio decoder: outputs is nil")
	}
	closer.Add(audioFilterOutputs.Free)

	audioFilterInputs = astiav.AllocFilterInOut()
	if audioFilterInputs == nil {
		return errors.New("create audio decoder: inputs is nil")
	}
	closer.Add(audioFilterInputs.Free)

	return nil
}

func UpdateAudioFilter() error {
	buffersrcContextParameters := astiav.AllocBuffersrcFilterContextParameters()
	defer buffersrcContextParameters.Free()
	buffersrcContextParameters.SetSampleFormat(audioResampled.SampleFormat())
	buffersrcContextParameters.SetChannelLayout(audioResampled.ChannelLayout())
	buffersrcContextParameters.SetSampleRate(audioResampled.SampleRate())
	buffersrcContextParameters.SetTimeBase(audioStream.TimeBase())

	if err := audioBuffersrcCtx.SetParameters(buffersrcContextParameters); err != nil {
		return fmt.Errorf("update audio filter: setting buffersrc context parameters failed: %w", err)
	}

	if err := audioBuffersrcCtx.Initialize(nil); err != nil {
		return fmt.Errorf("update audio filter: initializing buffersrc context failed: %w", err)
	}

	audioFilterOutputs.SetName("in")
	audioFilterOutputs.SetFilterContext(audioBuffersrcCtx.FilterContext())
	audioFilterOutputs.SetPadIdx(0)
	audioFilterOutputs.SetNext(nil)

	audioFilterInputs.SetName("out")
	audioFilterInputs.SetFilterContext(audioBuffersinkCtx.FilterContext())
	audioFilterInputs.SetPadIdx(0)
	audioFilterInputs.SetNext(nil)

	if err := audioFilterGraph.Parse("loudnorm,aresample=44100,asetnsamples=n=1024,aformat=sample_fmts=flt:sample_rates=44100:channel_layouts=stereo", audioFilterInputs, audioFilterOutputs); err != nil {
		return fmt.Errorf("update audio filter: parsing filter failed: %w", err)
	}

	if err := audioFilterGraph.Configure(); err != nil {
		return fmt.Errorf("update audio filter: configuring filter failed: %w", err)
	}

	return nil
}

func DecodeAudio(pkt *astiav.Packet) error {
	if err := audioCodecCtx.SendPacket(pkt); err != nil {
		return fmt.Errorf("audio decode: sending packet to audio decoder failed: %w", err)
	}

	for {
		stop, err := decodeAudio()
		if err != nil {
			return err
		}

		if stop {
			break
		}
	}

	return nil
}

func decodeAudio() (bool, error) {
	if err := audioCodecCtx.ReceiveFrame(audioDecoded); err != nil {
		if errors.Is(err, astiav.ErrEof) || errors.Is(err, astiav.ErrEagain) {
			return true, nil
		}
		return false, fmt.Errorf("decode: receiving frame failed: %w", err)
	}

	defer audioDecoded.Unref()

	if err := audioResamplerCtx.ConvertFrame(audioDecoded, audioResampled); err != nil {
		return false, fmt.Errorf("decode: resampling decoded frame failed: %w", err)
	}

	if nbSamples := audioResampled.NbSamples(); nbSamples > 0 {
		if err := filterFrame(audioResampled); err != nil {
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
		if finalFlush || audioResamplerCtx.Delay(int64(audioResampled.SampleRate())) >= int64(audioResampled.NbSamples()) {
			if err := audioResamplerCtx.ConvertFrame(nil, audioResampled); err != nil {
				return fmt.Errorf("flush resampler: flushing resampler failed: %w", err)
			}

			if err := filterFrame(audioResampled); err != nil {
				return fmt.Errorf("flush resampler: adding resampled frame to filter failed: %w", err)
			}

			if finalFlush && audioResampled.NbSamples() == 0 {
				break
			}
			continue
		}
		break
	}

	return nil
}

func filterFrame(f *astiav.Frame) (err error) {
	if err = audioBuffersrcCtx.AddFrame(f, astiav.NewBuffersrcFlags(astiav.BuffersrcFlagKeepRef)); err != nil {
		return fmt.Errorf("filter audio: adding frame failed: %w", err)
	}

	for {
		if stop, err := func() (bool, error) {
			if err := audioBuffersinkCtx.GetFrame(audioFiltered, astiav.NewBuffersinkFlags()); err != nil {
				if errors.Is(err, astiav.ErrEof) || errors.Is(err, astiav.ErrEagain) {
					return true, nil
				}
				return false, fmt.Errorf("filter audio: getting frame failed: %w", err)
			}

			defer audioFiltered.Unref()

			if err := processFifo(false); err != nil {
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
	if audioFiltered.NbSamples() > 0 {
		if _, err := audioFifo.Write(audioFiltered); err != nil {
			return fmt.Errorf("process fifo: writing failed: %w", err)
		}
	}

	for {
		if (isFlush && audioFifo.Size() > 0) || (!isFlush && audioFifo.Size() >= audioCompleted.NbSamples()) {
			n, err := audioFifo.Read(audioCompleted)
			if err != nil {
				return fmt.Errorf("process fifo: reading failed: %w", err)
			}

			audioCompleted.SetNbSamples(n)

			Cur += float64(n) / float64(SAMPLE_RATE)

			data, err := audioCompleted.Data().Bytes(1)
			if err != nil {
				return err
			}
			_, err = aw.Write(data)
			if err != nil {
				return err
			}

			continue
		}
		break
	}
	return nil
}

func AudioIndex() int {
	return audioStream.Index()
}
