package media

import (
	"errors"
	"fmt"
	"image"
	"io"
	"log"

	"github.com/GoldenFealla/VideoPlayerGo/internal/decoder"
	"github.com/asticode/go-astiav"
	"github.com/asticode/go-astikit"
)

type Media struct {
	closer *astikit.Closer

	vfifo *FrameQueue
	afifo *astiav.AudioFifo

	iformat *astiav.FormatContext

	videoStream *decoder.VideoStream
	audioStream *decoder.AudioStream

	outputAudio io.Writer
	outputVideo chan image.Image

	lastVideoPts int64
	lastAudioPts int64

	cur float64
}

func NewMedia(
	videoStream *decoder.VideoStream,
	audioStream *decoder.AudioStream,
	outputVideo chan image.Image,
	outputAudio io.Writer,
) *Media {
	return &Media{
		vfifo:       NewFrameQueue(3),
		videoStream: videoStream,
		audioStream: audioStream,
		outputVideo: outputVideo,
		outputAudio: outputAudio,
		iformat:     astiav.AllocFormatContext(),
	}
}

func (m *Media) Close() {
	m.closer.Close()
}

func (m *Media) Load(input string) error {
	if m.iformat == nil {
		m.iformat = astiav.AllocFormatContext()
	}

	m.closer = astikit.NewCloser()
	m.iformat.Free()

	if err := m.iformat.OpenInput(input, nil, nil); err != nil {
		return fmt.Errorf("format context: opening input failed: %w", err)
	}
	m.closer.Add(m.iformat.CloseInput)

	if err := m.iformat.FindStreamInfo(nil); err != nil {
		return fmt.Errorf("format context: finding stream info failed: %w", err)
	}

	if err := m.videoStream.LoadInputContext(m.iformat); err != nil && !errors.Is(err, decoder.ErrNoVideo) {
		return fmt.Errorf("loaded video stream failed: %w", err)
	}
	m.videoStream.SetOutputCallback(m.EnqueueVideoFrame)

	if err := m.audioStream.LoadInputContext(m.iformat); err != nil && !errors.Is(err, decoder.ErrNoAudio) {
		return fmt.Errorf("loaded audio stream failed: %w", err)
	}
	m.audioStream.SetOutputCallback(m.EnqueueAudioFrame)

	m.afifo = astiav.AllocAudioFifo(
		m.audioStream.SampleFormat(),
		m.audioStream.Channel(),
		m.audioStream.NbSamples(),
	)
	m.closer.Add(m.afifo.Free)

	return nil
}

func (m *Media) Decode() error {
	pkt := astiav.AllocPacket()
	defer pkt.Free()

	for {
		stop, err := m.readPacket(pkt)
		if err != nil {
			return err
		}

		if stop {
			break
		}
	}

	return nil
}

func (m *Media) readPacket(pkt *astiav.Packet) (bool, error) {
	if err := m.iformat.ReadFrame(pkt); err != nil {
		if errors.Is(err, astiav.ErrEof) {
			return true, nil
		}
		return false, fmt.Errorf("decoding: reading packet failed: %w", err)
	}

	defer pkt.Unref()

	if pkt.StreamIndex() == m.videoStream.Index() {
		err := m.videoStream.Decode(pkt)
		if err != nil {
			log.Println(err)
		}
	} else if pkt.StreamIndex() == m.audioStream.Index() {
		err := m.audioStream.Decode(pkt)
		if err != nil {
			log.Println(err)
		}
	}

	return false, nil
}

func (m *Media) EnqueueVideoFrame(f *astiav.Frame) {
	defer f.Free()
	m.vfifo.Write(f)

}

func (m *Media) EnqueueAudioFrame(f *astiav.Frame) {
	defer f.Free()
	if _, err := m.afifo.Write(f); err != nil {
		log.Println(err)
	}
}

func (m *Media) Sync() {
	f := astiav.AllocFrame()
	defer f.Free()

	f.SetChannelLayout(m.audioStream.ChannelLayout())
	f.SetSampleRate(m.audioStream.SampleRate())
	f.SetSampleFormat(m.audioStream.SampleFormat())
	f.SetNbSamples(m.audioStream.NbSamples())
	if err := f.AllocBuffer(0); err != nil {
		log.Println(err)
	}

	for {
		if m.afifo.Size() >= f.NbSamples() {
			n, err := m.afifo.Read(f)
			if err != nil {
				log.Println(fmt.Errorf("sync: reading audio failed: %w", err))
			}
			f.SetNbSamples(n)

			d, err := f.Data().Bytes(1)
			if err != nil {
				log.Println(fmt.Errorf("sync: reading audio bytes failed: %w", err))
			}
			m.outputAudio.Write(d)
			m.lastAudioPts += int64(n)
			m.cur = float64(m.lastAudioPts) * m.audioStream.Timebase()

			if float64(m.vfifo.CurrentFramePTS())*m.videoStream.Timebase() < m.cur {
				vf := m.vfifo.Read()
				if vf != nil {
					i, err := vf.Data().GuessImageFormat()
					if err != nil {
						log.Println(fmt.Errorf("skip video frame: %w", err))
					}

					err = vf.Data().ToImage(i)
					if err != nil {
						log.Println(fmt.Errorf("skip video frame: %w", err))
					}

					m.outputVideo <- i
					m.lastVideoPts = f.Pts()
					vf.Free()
				}
			}
		}

	}

}
