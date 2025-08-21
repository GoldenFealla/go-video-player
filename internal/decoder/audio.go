package decoder

import (
	"errors"
	"fmt"
	"log"

	"github.com/asticode/go-astiav"
	"github.com/asticode/go-astikit"
)

var (
	DEFAULT_SAMPLE_RATE    = 44100
	DEFAULT_CHANNEL_LAYOUT = astiav.ChannelLayoutStereo
	DEFAULT_AUDIO_FORMAT   = astiav.SampleFormatFlt
	DEFAULT_NB_SAMPLES     = 1024
)

type AudioStream struct {
	st *astiav.Stream
	cc *astiav.CodecContext

	c        *ASConfig
	i        int
	timebase float64

	src   *astiav.SoftwareResampleContext
	afifo *astiav.AudioFifo

	df *astiav.Frame
	rf *astiav.Frame
	ff *astiav.Frame

	closer *astikit.Closer

	outputCallback func(*astiav.Frame)
}

type ASConfig struct {
	sampleRate    int
	channelLayout astiav.ChannelLayout
	sampleFormat  astiav.SampleFormat
	nbSamples     int
}

type ASOption func(*ASConfig)

func WithSampleRate(s int) ASOption {
	return func(a *ASConfig) {
		a.sampleRate = s
	}
}

func WithChannelLayout(cl astiav.ChannelLayout) ASOption {
	return func(a *ASConfig) {
		a.channelLayout = cl
	}
}

func WithAudioFormat(sf astiav.SampleFormat) ASOption {
	return func(a *ASConfig) {
		a.sampleFormat = sf
	}
}

func WithNBSamples(nbs int) ASOption {
	return func(a *ASConfig) {
		a.nbSamples = nbs
	}
}

func NewAudioStream(opts ...ASOption) *AudioStream {
	ast := &AudioStream{
		closer: astikit.NewCloser(),
	}

	ast.c = &ASConfig{
		sampleRate:    DEFAULT_SAMPLE_RATE,
		channelLayout: DEFAULT_CHANNEL_LAYOUT,
		sampleFormat:  DEFAULT_AUDIO_FORMAT,
		nbSamples:     DEFAULT_NB_SAMPLES,
	}

	for _, opt := range opts {
		opt(ast.c)
	}

	ast.df = astiav.AllocFrame()
	ast.closer.Add(ast.df.Free)

	ast.rf = astiav.AllocFrame()
	ast.closer.Add(ast.rf.Free)

	ast.rf.SetSampleRate(ast.c.sampleRate)
	ast.rf.SetChannelLayout(ast.c.channelLayout)
	ast.rf.SetSampleFormat(ast.c.sampleFormat)
	ast.rf.SetNbSamples(ast.c.nbSamples)
	if err := ast.rf.AllocBuffer(0); err != nil {
		log.Fatal(fmt.Errorf("audio stream: allocating rf buffer failed: %w", err))
	}

	ast.ff = astiav.AllocFrame()
	ast.closer.Add(ast.ff.Free)

	ast.ff.SetChannelLayout(ast.rf.ChannelLayout())
	ast.ff.SetNbSamples(ast.rf.NbSamples())
	ast.ff.SetSampleFormat(ast.rf.SampleFormat())
	ast.ff.SetSampleRate(ast.rf.SampleRate())
	if err := ast.ff.AllocBuffer(0); err != nil {
		log.Fatal(fmt.Errorf("audio stream: allocating ff buffer failed: %w", err))
	}

	ast.afifo = astiav.AllocAudioFifo(
		ast.ff.SampleFormat(),
		ast.ff.ChannelLayout().Channels(),
		ast.ff.NbSamples(),
	)
	ast.closer.Add(ast.afifo.Free)

	ast.src = astiav.AllocSoftwareResampleContext()
	ast.closer.Add(ast.src.Free)

	return ast
}

func (ast *AudioStream) Close() {
	ast.closer.Close()
}

func (ast *AudioStream) Index() int {
	return ast.i
}

func (ast *AudioStream) Timebase() float64 {
	return ast.timebase
}

func (ast *AudioStream) SampleFormat() astiav.SampleFormat {
	return ast.c.sampleFormat
}

func (ast *AudioStream) SampleRate() int {
	return ast.c.sampleRate
}

func (ast *AudioStream) Channel() int {
	return ast.c.channelLayout.Channels()
}

func (ast *AudioStream) ChannelLayout() astiav.ChannelLayout {
	return ast.c.channelLayout
}

func (ast *AudioStream) NbSamples() int {
	return ast.c.nbSamples
}

func (ast *AudioStream) SetOutputCallback(callback func(*astiav.Frame)) {
	ast.outputCallback = callback
}

func (ast *AudioStream) LoadInputContext(i *astiav.FormatContext) error {
	if i == nil {
		return ErrInputContextNil
	}

	for _, is := range i.Streams() {
		if is.CodecParameters().MediaType() != astiav.MediaTypeAudio {
			continue
		}

		ast.st = is
		ast.i = is.Index()
		ast.timebase = is.TimeBase().Float64()

		codec := astiav.FindDecoder(is.CodecParameters().CodecID())
		if codec == nil {
			return errors.New("finding audio codec: codec is nil")
		}

		if ast.cc = astiav.AllocCodecContext(codec); ast.cc == nil {
			return errors.New("finding audio codec: codec context is nil")
		}
		ast.closer.Add(ast.cc.Free)

		if err := is.CodecParameters().ToCodecContext(ast.cc); err != nil {
			return fmt.Errorf("finding audio codec: updating codec context failed: %w", err)
		}

		if err := ast.cc.Open(codec, nil); err != nil {
			return fmt.Errorf("finding audio codec: opening codec context failed: %w", err)
		}

		break
	}

	if ast.st == nil {
		return ErrNoAudio
	}

	return nil
}

func (ast *AudioStream) Decode(pkt *astiav.Packet) error {
	if err := ast.cc.SendPacket(pkt); err != nil {
		return fmt.Errorf("audio decode: sending packet to audio decoder failed: %w", err)
	}

	for {
		stop, err := ast.decode()
		if err != nil {
			return err
		}

		if stop {
			return nil
		}
	}
}

func (ast *AudioStream) decode() (bool, error) {
	if err := ast.cc.ReceiveFrame(ast.df); err != nil {
		if errors.Is(err, astiav.ErrEof) || errors.Is(err, astiav.ErrEagain) {
			return true, nil
		}
		return false, fmt.Errorf("audio decode: receiving frame failed: %w", err)
	}

	defer ast.df.Unref()

	if err := ast.src.ConvertFrame(ast.df, ast.rf); err != nil {
		return false, fmt.Errorf("audio decode: resampling decoded frame failed: %w", err)
	}

	if nbSamples := ast.rf.NbSamples(); nbSamples > 0 {
		ast.outputCallback(ast.rf.Clone())

		if err := ast.flushResampler(false); err != nil {
			return false, fmt.Errorf("audio decode: flushing software resample context failed: %w", err)
		}
		return false, nil
	}

	return false, nil
}

func (ast *AudioStream) flushResampler(finalFlush bool) error {
	for {
		if finalFlush || ast.src.Delay(
			int64(ast.rf.SampleRate())) >= int64(ast.rf.NbSamples()) {
			if err := ast.src.ConvertFrame(nil, ast.rf); err != nil {
				return fmt.Errorf("flush resampler: flushing resampler failed: %w", err)
			}

			if ast.rf.NbSamples() == 0 {
				break
			}

			ast.outputCallback(ast.rf.Clone())

			continue
		}
		break
	}

	return nil
}
