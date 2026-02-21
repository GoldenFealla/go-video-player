package player

import (
	"errors"
	"fmt"
	"log"

	"github.com/asticode/go-astiav"
	"github.com/asticode/go-astikit"
)

// ==================== VIDEO ====================

type VideoDecoder struct {
	closer *astikit.Closer

	stream *astiav.Stream
	codec  *astiav.Codec
	ctx    *astiav.CodecContext

	timebase float64
}

func NewVideoDecoder() *VideoDecoder {
	return &VideoDecoder{
		closer: astikit.NewCloser(),
	}
}

func (vd *VideoDecoder) Load(inputStream *astiav.Stream) error {
	vd.stream = inputStream

	if vd.codec = astiav.FindDecoder(inputStream.CodecParameters().CodecID()); vd.codec == nil {
		return errors.New("video decoder: codec is nil")
	}

	if vd.ctx = astiav.AllocCodecContext(vd.codec); vd.ctx == nil {
		return errors.New("video decoder: codec context is nil")
	}
	vd.closer.Add(vd.ctx.Free)

	if err := inputStream.CodecParameters().ToCodecContext(vd.ctx); err != nil {
		return fmt.Errorf("video decoder: updating codec context failed: %w", err)
	}

	if err := vd.ctx.Open(vd.codec, nil); err != nil {
		return fmt.Errorf("video decoder: opening codec context failed: %w", err)
	}

	vd.timebase = inputStream.TimeBase().Float64()

	return nil
}

func (vd *VideoDecoder) Decode(pkt *astiav.Packet, out chan<- VideoFrame) {
	f := astiav.AllocFrame()
	defer f.Free()

	if err := vd.ctx.SendPacket(pkt); err != nil {
		log.Println(fmt.Errorf("video decode: sending packet failed: %w", err))
	}

	for {
		if stop := func() bool {
			if err := vd.ctx.ReceiveFrame(f); err != nil {
				if !errors.Is(err, astiav.ErrEof) && !errors.Is(err, astiav.ErrEagain) {
					log.Println(fmt.Errorf("video decode: receiving frame failed: %w", err))
				}
				return true
			}

			defer f.Unref()

			pts := float64(f.Pts()) * vd.timebase

			fd := f.Data()
			buf, _ := fd.Bytes(1)

			w := f.Width()
			h := f.Height()

			ls := f.Linesize()
			ls0 := ls[0]
			ls1 := ls[1]
			ls2 := ls[2]

			ySize := ls0 * h
			uSize := ls1 * (h / 2)
			vSize := ls2 * (h / 2)

			ySrc := buf[0:ySize]
			uSrc := buf[ySize : ySize+uSize]
			vSrc := buf[ySize+uSize : ySize+uSize+vSize]

			y := repackPlane(ySrc, ls0, w, h)
			u := repackPlane(uSrc, ls1, w/2, h/2)
			v := repackPlane(vSrc, ls2, w/2, h/2)

			out <- VideoFrame{
				PTS: pts,
				Y:   y,
				U:   u,
				V:   v,
				W:   w,
				H:   h,
			}

			return false
		}(); stop {
			break
		}
	}
}

func repackPlane(src []byte, linesize, width, height int) []byte {
	dst := make([]byte, width*height)
	for y := range height {
		copy(dst[y*width:(y+1)*width], src[y*linesize:y*linesize+width])
	}
	return dst
}

func (vd *VideoDecoder) Free() {
	vd.closer.Close()
}

// ==================== AUDIO ====================

type AudioDecoder struct {
	closer *astikit.Closer

	stream *astiav.Stream
	codec  *astiav.Codec
	ctx    *astiav.CodecContext

	src *astiav.SoftwareResampleContext

	timebase float64
}

func NewAudioDecoder() *AudioDecoder {
	ad := &AudioDecoder{
		closer: astikit.NewCloser(),
		src:    astiav.AllocSoftwareResampleContext(),
	}

	ad.closer.Add(ad.src.Free)

	return ad
}

func (ad *AudioDecoder) Load(inputStream *astiav.Stream) error {
	ad.stream = inputStream

	if ad.codec = astiav.FindDecoder(inputStream.CodecParameters().CodecID()); ad.codec == nil {
		return errors.New("audio decoder: codec is nil")
	}

	if ad.ctx = astiav.AllocCodecContext(ad.codec); ad.ctx == nil {
		return errors.New("audio decoder: codec context is nil")
	}
	ad.closer.Add(ad.ctx.Free)

	if err := inputStream.CodecParameters().ToCodecContext(ad.ctx); err != nil {
		return fmt.Errorf("audio decoder: updating codec context failed: %w", err)
	}

	if err := ad.ctx.Open(ad.codec, nil); err != nil {
		return fmt.Errorf("audio decoder: opening codec context failed: %w", err)
	}

	ad.timebase = inputStream.TimeBase().Float64()

	return nil
}

func (ad *AudioDecoder) Decode(pkt *astiav.Packet, out chan<- AudioFrame) {
	f := astiav.AllocFrame()
	defer f.Free()

	r := astiav.AllocFrame()
	defer r.Free()
	r.SetChannelLayout(astiav.ChannelLayoutStereo)
	r.SetSampleFormat(astiav.SampleFormatFlt)
	r.SetSampleRate(48000)
	if err := ad.ctx.SendPacket(pkt); err != nil {
		log.Println(fmt.Errorf("audio decode: sending packet failed: %w", err))
	}

	for {
		if stop := func() bool {
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
				fd := r.Data()
				buf, _ := fd.Bytes(1)

				pts := float64(f.Pts()) * ad.timebase

				out <- AudioFrame{
					PTS:       pts,
					NbSamples: uint32(len(buf)),
					Samples:   buf,
					Rate:      48000,
				}
			}

			return false
		}(); stop {
			break
		}
	}
}

func (ad *AudioDecoder) Free() {
	ad.closer.Close()
}
