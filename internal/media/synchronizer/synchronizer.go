package synchronizer

import (
	"image"
	"io"
	"log"

	"github.com/GoldenFealla/VideoPlayerGo/internal/media/audiodecoder"
	"github.com/GoldenFealla/VideoPlayerGo/internal/media/videodecoder"
	"github.com/asticode/go-astiav"
)

var (
	SyncAudioReader, SyncAudioWriter     = io.Pipe()
	OutputAudioReader, OutputAudioWriter = io.Pipe()

	RecieveChan = make(chan image.Image)
	OutputChan  = make(chan image.Image)

	ab = make([]byte, 1024)
)

func RunVideoSync() {
	for {
		img, ok := <-RecieveChan
		if !ok {
			continue
		}
		OutputChan <- img
	}
}

func RunAudioSync() {
	for {
		_, err := SyncAudioReader.Read(ab)
		if err != nil {
			log.Println(err)
		} else {
			OutputAudioWriter.Write(ab)
		}
	}
}

func SendToDecoder(pkt *astiav.Packet) {
	if pkt.StreamIndex() == videodecoder.Index() {
		videodecoder.Decode(pkt)
	} else if pkt.StreamIndex() == audiodecoder.Index() {
		audiodecoder.Decode(pkt)
	}
	pkt.Free()
}
