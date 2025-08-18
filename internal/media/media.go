package media

import (
	"errors"
	"fmt"

	"github.com/GoldenFealla/VideoPlayerGo/internal/decoder"
	"github.com/asticode/go-astiav"
	"github.com/asticode/go-astikit"
)

type Media struct {
	closer *astikit.Closer

	iformat *astiav.FormatContext

	videoStream *decoder.VideoStream
	audioStream *decoder.AudioStream
}

func NewMedia(
	videoStream *decoder.VideoStream,
	audioStream *decoder.AudioStream,
) *Media {
	return &Media{
		iformat: astiav.AllocFormatContext(),
	}
}

func (m *Media) Close() {
	m.closer.Close()
}

func (m *Media) Load(input string) error {
	if m.iformat == nil {
		m.iformat = astiav.AllocFormatContext()
	}

	m.iformat.Free()
	if err := m.iformat.OpenInput(input, nil, nil); err != nil {
		return fmt.Errorf("format context: opening input failed: %w", err)
	}
	m.closer.Add(m.iformat.CloseInput)

	if err := m.iformat.FindStreamInfo(nil); err != nil {
		return fmt.Errorf("format context: finding stream info failed: %w", err)
	}

	if err := m.videoStream.LoadInputContext(m.iformat); !errors.Is(err, decoder.ErrNoVideo) {
		return fmt.Errorf("loaded video stream failed: %w", err)
	}
	m.videoStream.SetOutoutCallback(m.EnqueueVideoFrame)

	if err := m.audioStream.LoadInputContext(m.iformat); !errors.Is(err, decoder.ErrNoAudio) {
		return fmt.Errorf("loaded audio stream failed: %w", err)
	}
	m.audioStream.SetOutoutCallback(m.EnqueueAudioFrame)

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
		m.videoStream.Decode(pkt)
	} else if pkt.StreamIndex() == m.audioStream.Index() {
		m.audioStream.Decode(pkt)
	}

	return false, nil
}

func (m *Media) EnqueueVideoFrame(f *astiav.Frame) {

}

func (m *Media) EnqueueAudioFrame(f *astiav.Frame) {

}
