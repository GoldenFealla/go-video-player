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

	iformat *astiav.FormatContext

	videoStream *decoder.VideoStream
	audioStream *decoder.AudioStream

	outputAudio io.Writer
	outputVideo chan image.Image

	lastVideoPts int64
	lastAudioPts int64
}

func NewMedia(
	videoStream *decoder.VideoStream,
	audioStream *decoder.AudioStream,
	outputVideo chan image.Image,
	outputAudio io.Writer,
) *Media {
	return &Media{
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
	i, err := f.Data().GuessImageFormat()
	if err != nil {
		log.Println("skip video frame")
	}

	err = f.Data().ToImage(i)
	if err != nil {
		log.Println("skip video frame")
	}

	m.outputVideo <- i

	m.lastVideoPts = f.Pts()
	fmt.Printf(
		"video: %10d audio: %10d a/v: %+2.3f\r",
		m.lastVideoPts,
		m.lastAudioPts,
		float64(m.lastAudioPts)*m.audioStream.Timebase()-
			float64(m.lastVideoPts)*m.videoStream.Timebase(),
	)
}

func (m *Media) EnqueueAudioFrame(f *astiav.Frame) {
	defer f.Free()

	d, err := f.Data().Bytes(1)
	if err != nil {
		log.Println("skip audio frame")
	}
	_, err = m.outputAudio.Write(d)
	if err != nil {
		log.Println("skip audio frame")
	}

	m.lastAudioPts = f.Pts()
	fmt.Printf(
		"video: %10d audio: %10d a/v: %+2.3f\r",
		m.lastVideoPts,
		m.lastAudioPts,
		float64(m.lastAudioPts)*m.audioStream.Timebase()-
			float64(m.lastVideoPts)*m.videoStream.Timebase(),
	)
}
