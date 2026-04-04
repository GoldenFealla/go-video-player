package player

import (
	"errors"
	"fmt"
	"log"

	"github.com/asticode/go-astiav"
	"github.com/asticode/go-astikit"
)

// ====== Audio ======
type audiodecoder struct {
	closer *astikit.Closer
	ctx    *astiav.CodecContext
	src    *astiav.SoftwareResampleContext

	has      bool
	timebase astiav.Rational
}

func newaudiodecoder() *audiodecoder {
	ad := &audiodecoder{}

	ad.closer = astikit.NewCloser()
	ad.src = astiav.AllocSoftwareResampleContext()

	return ad
}

func (ad *audiodecoder) close() {
	ad.closer.Close()
}

type AudioData struct {
	PTS     int64
	Samples []byte
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

	ad.timebase = stream.TimeBase()
	ad.has = true
	return nil
}

func (ad *audiodecoder) decode(pkt *astiav.Packet, input chan<- AudioData) error {
	f := astiav.AllocFrame()
	defer f.Free()

	r := astiav.AllocFrame()
	defer r.Free()

	r.SetChannelLayout(astiav.ChannelLayoutStereo)
	r.SetSampleFormat(astiav.SampleFormatS16)
	r.SetSampleRate(48000)
	if err := ad.ctx.SendPacket(pkt); err != nil {
		log.Println(fmt.Errorf("audio decode: sending packet failed: %w", err))
	}

	for {
		if ad.decodeloop(f, r, input) {
			break
		}
	}

	return nil
}

func (ad *audiodecoder) decodeloop(f, r *astiav.Frame, input chan<- AudioData) bool {
	if err := ad.ctx.ReceiveFrame(f); err != nil {
		if !errors.Is(err, astiav.ErrEof) && !errors.Is(err, astiav.ErrEagain) {
			log.Println(fmt.Errorf("audio decode: receiving frame failed: %w", err))
		}
		return true
	}

	defer f.Unref()

	if err := ad.src.ConvertFrame(f, r); err != nil {
		log.Println(fmt.Errorf("audio decode: resampling decoded frame failed: %w", err))
		return true
	}

	if nbSamples := r.NbSamples(); nbSamples > 0 {
		buf, _ := r.Data().Bytes(1)
		input <- AudioData{
			PTS:     f.Pts(),
			Samples: buf,
		}
	}

	return false
}

// ====== VIDEO ======
type videodecoder struct {
	closer *astikit.Closer

	ctx *astiav.CodecContext

	has      bool
	timebase astiav.Rational
}

func newvideodecoder() *videodecoder {
	vd := &videodecoder{}

	vd.closer = astikit.NewCloser()

	return vd
}

func (vd *videodecoder) close() {
	vd.closer.Close()
}

type VideoData struct {
	W    int
	H    int
	Data []byte
}

func (vd *videodecoder) load(stream *astiav.Stream) error {
	codec := astiav.FindDecoder(stream.CodecParameters().CodecID())
	if codec == nil {
		return errors.New("video decoder: codec is nil")
	}

	vd.ctx = astiav.AllocCodecContext(codec)
	if vd.ctx == nil {
		return errors.New("video decoder: codec context is nil")
	}
	vd.closer.Add(vd.ctx.Free)

	err := stream.CodecParameters().ToCodecContext(vd.ctx)
	if err != nil {
		return fmt.Errorf("video decoder: updating codec context failed: %w", err)
	}

	err = vd.ctx.Open(codec, nil)
	if err != nil {
		return fmt.Errorf("video decoder: opening codec context failed: %w", err)
	}

	vd.timebase = stream.TimeBase()
	vd.has = true
	return nil
}

func (vd *videodecoder) decode(pkt *astiav.Packet, input chan<- VideoData) error {
	f := astiav.AllocFrame()
	defer f.Free()

	if err := vd.ctx.SendPacket(pkt); err != nil {
		log.Println(fmt.Errorf("video decode: sending packet failed: %w", err))
	}

	for {
		if vd.decodeloop(f, input) {
			break
		}
	}

	return nil
}

func (ad *videodecoder) decodeloop(f *astiav.Frame, input chan<- VideoData) bool {
	if err := ad.ctx.ReceiveFrame(f); err != nil {
		if !errors.Is(err, astiav.ErrEof) && !errors.Is(err, astiav.ErrEagain) {
			log.Println(fmt.Errorf("audio decode: receiving frame failed: %w", err))
		}
		return true
	}

	defer f.Unref()

	// pts := float64(f.Pts()) * vd.timebase
	buf, _ := f.Data().Bytes(1)
	input <- VideoData{
		W:    f.Width(),
		H:    f.Height(),
		Data: buf,
	}

	return false
}

// ====== DEMUX ======
type demuxer struct {
	ic *astiav.FormatContext
}

func newdemuxer() *demuxer {
	return &demuxer{
		ic: astiav.AllocFormatContext(),
	}
}

func (d *demuxer) open(path string) error {
	if err := d.ic.OpenInput(path, nil, nil); err != nil {
		return err
	}

	if err := d.ic.FindStreamInfo(nil); err != nil {
		return err
	}

	return nil
}

func (d *demuxer) demux(o chan<- *astiav.Packet) error {
	pkt := astiav.AllocPacket()
	defer pkt.Free()

	for {
		if stop := func() bool {
			if err := d.ic.ReadFrame(pkt); err != nil {
				if !errors.Is(err, astiav.ErrEof) {
					log.Println(fmt.Errorf("demux: reading frame failed: %w", err))
				}
			}

			defer pkt.Unref()
			o <- pkt.Clone()

			return false
		}(); stop {
			break
		}
	}

	return nil
}

// ====== CODEC ======
type codec struct {
	demuxer *demuxer

	pkt chan *astiav.Packet

	audio    *audiodecoder
	audioidx int

	video    *videodecoder
	videoidx int
}

func newcodec() *codec {
	return &codec{
		demuxer: newdemuxer(),

		audio: newaudiodecoder(),
		video: newvideodecoder(),

		pkt: make(chan *astiav.Packet, 16),
	}
}

func (c *codec) load(path string) error {
	c.demuxer.open(path)

	var err error
	for _, s := range c.demuxer.ic.Streams() {
		switch s.CodecParameters().MediaType() {
		case astiav.MediaTypeVideo:
			c.videoidx = s.Index()
			err = c.video.load(s)
		case astiav.MediaTypeAudio:
			c.audioidx = s.Index()
			err = c.audio.load(s)
		}
	}

	return err
}

func (c *codec) parse(audiochan chan<- AudioData, videochan chan<- VideoData) {
	go c.demuxer.demux(c.pkt)

	for pkt := range c.pkt {
		switch idx := pkt.StreamIndex(); idx {
		case c.videoidx:
			c.video.decode(pkt, videochan)
		case c.audioidx:
			c.audio.decode(pkt, audiochan)
		default:
		}
	}
}

type Player struct {
	codec *codec
	clock *clock

	audiooutput chan AudioData
	videooutput chan VideoData
}

func NewPlayer() *Player {
	return &Player{
		codec: newcodec(),
		clock: &clock{d: 0},

		audiooutput: make(chan AudioData, 16),
		videooutput: make(chan VideoData, 16),
	}
}

func (p *Player) AudioOutput() <-chan AudioData {
	return p.audiooutput
}

func (p *Player) VideoOutput() <-chan VideoData {
	return p.videooutput
}

func (p *Player) SetCurrent(n int64) {
	p.clock.set(n)
}

func (p *Player) GetSecond() float64 {
	return p.clock.time()
}

func (p *Player) Seek() {

}

func (p *Player) Play(path string) error {
	err := p.codec.load(path)
	if err != nil {
		return err
	}

	fmt.Println(p.codec.audio.timebase)

	if p.codec.audio.has {
		p.clock.b = astiav.NewRational(1, 48000*4)
	}

	p.codec.parse(p.audiooutput, p.videooutput)

	return nil
}
