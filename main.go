package main

import (
	"fmt"
	"image/color"
	"io"
	"log"

	"golang.org/x/sync/errgroup"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"github.com/ebitengine/oto/v3"

	"github.com/GoldenFealla/VideoPlayerGo/internal/decoder"
)

var (
	Input string = "./test.mp4"
)

var (
	Decoder *decoder.Decoder

	AudioDecoder *decoder.AudioDecoder

	Synchronizer *decoder.Synchronizer

	AudioReader io.Reader
	AudioWriter io.Writer
	VideoReader io.Reader
	VideoWriter io.Writer
)

var (
	OtoOption = &oto.NewContextOptions{
		SampleRate:   44100,
		ChannelCount: 2,
		Format:       oto.FormatFloat32LE,
	}
	AudioPlayer *oto.Player
)

func init() {
	var err error
	AudioReader, AudioWriter = io.Pipe()
	VideoReader, VideoWriter = io.Pipe()

	AudioDecoder, err = decoder.NewAudioDecoder()
	if err != nil {
		log.Fatalln(err)
	}

	Decoder, err = decoder.NewDecoder(AudioDecoder)
	if err != nil {
		log.Fatalln(err)
	}

	Synchronizer = decoder.NewSynchronizer(AudioDecoder, VideoWriter, AudioWriter)

	otoCtx, readyChan, err := oto.NewContext(OtoOption)
	if err != nil {
		panic("oto.NewContext failed: " + err.Error())
	}
	<-readyChan
	AudioPlayer = otoCtx.NewPlayer(AudioReader)
	AudioPlayer.SetVolume(1)

	if err := Decoder.Open(Input); err != nil {
		log.Fatalln(err)
	}
}

func main() {
	defer func() {
		AudioDecoder.Free()
		Decoder.Free()

		AudioPlayer.Close()
	}()

	myApp := app.New()
	w := myApp.NewWindow("Image")

	go Synchronizer.Run(func(cur float64) {
		s := fmt.Sprintf("%.3f", cur)
		text := canvas.NewText(s, color.White)
		text.Alignment = fyne.TextAlignTrailing
		text.TextStyle = fyne.TextStyle{Italic: true}
		w.SetContent(text)
	})

	go func() {
		errsgroup := new(errgroup.Group)

		errsgroup.Go(Decoder.Decode)

		if err := errsgroup.Wait(); err == nil {
			log.Fatalln(err)
		}
	}()

	AudioPlayer.Play()

	w.ShowAndRun()
}
