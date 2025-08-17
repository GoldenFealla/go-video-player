package decoder

import (
	"errors"
	"fmt"
	"image"

	"github.com/asticode/go-astiav"
)

type VideoData struct {
	Img  image.Image
	Time float64
}

var (
	videoStream   *astiav.Stream
	videoCodecCtx *astiav.CodecContext

	videoDecoded *astiav.Frame

	outputChan chan VideoData
)

func InitVideo(s *astiav.Stream, cc *astiav.CodecContext, dst chan VideoData) error {
	videoStream = s
	videoCodecCtx = cc
	outputChan = dst

	videoDecoded = astiav.AllocFrame()
	closer.Add(videoDecoded.Free)

	return nil
}

func UpdateVideoFilter() error {
	return nil
}

func DecodeVideo(pkt *astiav.Packet) error {
	if err := videoCodecCtx.SendPacket(pkt); err != nil {
		return fmt.Errorf("video decode: sending packet to audio decoder failed: %w", err)
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
	if err := videoCodecCtx.ReceiveFrame(videoDecoded); err != nil {
		if errors.Is(err, astiav.ErrEof) || errors.Is(err, astiav.ErrEagain) {
			return true, nil
		}
		return true, fmt.Errorf("video decoding: receiving frame failed: %w", err)
	}

	defer videoDecoded.Unref()

	i, err := videoDecoded.Data().GuessImageFormat()
	if err != nil {
		return false, err
	}

	err = videoDecoded.Data().ToImage(i)
	if err != nil {
		return false, err
	}

	outputChan <- VideoData{
		Img:  i,
		Time: float64(videoDecoded.Pts()) * videoStream.TimeBase().Float64(),
	}
	// select {
	// case outputChan <- VideoData{
	// 	img: i,
	// 	t:   float64(videoDecoded.Pts()) * videoStream.TimeBase().Float64(),
	// }:
	// default:
	// }

	return false, nil
}

func VideoIndex() int {
	return videoStream.Index()
}

func VideoTimeBase() float64 {
	return videoStream.TimeBase().Float64()
}
