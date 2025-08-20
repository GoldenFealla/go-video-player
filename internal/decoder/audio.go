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
	st    *astiav.Stream
	cc    *astiav.CodecContext
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
	audioFormat   astiav.SampleFormat
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
		a.audioFormat = sf
	}
}

func WithNBSamples(nbs int) ASOption {
	return func(a *ASConfig) {
		a.nbSamples = nbs
	}
}

func NewAudioStream(opts ...ASOption) *AudioStream {
	c := ASConfig{
		sampleRate:    DEFAULT_SAMPLE_RATE,
		channelLayout: DEFAULT_CHANNEL_LAYOUT,
		audioFormat:   DEFAULT_AUDIO_FORMAT,
		nbSamples:     DEFAULT_NB_SAMPLES,
	}

	ast := &AudioStream{
		closer: astikit.NewCloser(),
	}

	for _, opt := range opts {
		opt(&c)
	}

	ast.df = astiav.AllocFrame()
	ast.closer.Add(ast.df.Free)

	ast.rf = astiav.AllocFrame()
	ast.closer.Add(ast.rf.Free)

	ast.rf.SetSampleRate(44100)
	ast.rf.SetChannelLayout(astiav.ChannelLayoutStereo)
	ast.rf.SetSampleFormat(astiav.SampleFormatFlt)
	ast.rf.SetNbSamples(1024)
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
	return ast.st.Index()
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

	ast.rf.SetPts(ast.df.Pts())

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

			if finalFlush && ast.rf.NbSamples() == 0 {
				break
			}

			continue
		}
		break
	}

	return nil
}
