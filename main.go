// package main

// import (
// 	"image"
// 	"io"
// 	"log"

// 	"golang.org/x/sync/errgroup"

// 	"fyne.io/fyne/v2"
// 	"fyne.io/fyne/v2/app"
// 	"fyne.io/fyne/v2/canvas"
// 	"github.com/ebitengine/oto/v3"

// 	"github.com/GoldenFealla/VideoPlayerGo/internal/decoder"
// )

// var (
// 	Input string = "./The Lost Beyond.mp4"
// )

// var (
// 	Decoder *decoder.Decoder

// 	VideoDecoder *decoder.VideoDecoder
// 	AudioDecoder *decoder.AudioDecoder
// 	Synchronizer *decoder.Synchronizer

// 	AudioReader io.Reader
// 	AudioWriter io.Writer
// 	VideoReader io.Reader
// 	VideoWriter io.Writer
// )

// var (
// 	OtoOption = &oto.NewContextOptions{
// 		SampleRate:   44100,
// 		ChannelCount: 2,
// 		Format:       oto.FormatFloat32LE,
// 	}
// 	AudioPlayer *oto.Player
// )

// func init() {
// 	var err error
// 	VideoReader, VideoWriter = io.Pipe()
// 	AudioReader, AudioWriter = io.Pipe()

// 	VideoDecoder, err = decoder.NewVideoDecoder()
// 	if err != nil {
// 		log.Fatalln(err)
// 	}

// 	AudioDecoder, err = decoder.NewAudioDecoder()
// 	if err != nil {
// 		log.Fatalln(err)
// 	}

// 	Decoder, err = decoder.NewDecoder(VideoDecoder, AudioDecoder)
// 	if err != nil {
// 		log.Fatalln(err)
// 	}

// 	Synchronizer = decoder.NewSynchronizer(VideoDecoder, AudioDecoder, VideoWriter, AudioWriter)

// 	otoCtx, readyChan, err := oto.NewContext(OtoOption)
// 	if err != nil {
// 		panic("oto.NewContext failed: " + err.Error())
// 	}
// 	<-readyChan
// 	AudioPlayer = otoCtx.NewPlayer(AudioReader)
// 	AudioPlayer.SetVolume(1)

// 	if err := Decoder.Open(Input); err != nil {
// 		log.Fatalln(err)
// 	}
// }

// func main() {
// 	defer func() {
// 		AudioDecoder.Free()
// 		Decoder.Free()

// 		AudioPlayer.Close()
// 	}()

// 	myApp := app.New()
// 	w := myApp.NewWindow("Video Player")
// 	w.Resize(fyne.Size{Width: 1280, Height: 720})

// 	upLeft := image.Point{0, 0}
// 	lowRight := image.Point{1280, 720}
// 	imgRect := image.Rectangle{upLeft, lowRight}

// 	// Create a new RGBA image with the specified bounds
// 	img := image.NewRGBA(imgRect)

// 	frame := canvas.NewImageFromImage(img)
// 	frame.FillMode = canvas.ImageFillContain
// 	w.SetContent(frame)

// 	go Synchronizer.Run(func(cur float64, i image.Image) {
// 		if i == nil {
// 			return
// 		}
// 		frame.Image = i
// 		frame.Refresh()
// 	})

// 	go func() {
// 		errsgroup := new(errgroup.Group)

// 		errsgroup.Go(Decoder.Decode)

// 		if err := errsgroup.Wait(); err == nil {
// 			log.Fatalln(err)
// 		}
// 	}()

// 	AudioPlayer.Play()

//		w.ShowAndRun()
//	}
package main

import (
	"os"

	"gioui.org/app"
	"gioui.org/op"
	"gioui.org/unit"
)

func main() {
	go func() {
		w := new(app.Window)
		w.Option(
			app.Title("Video player"),
			app.Size(unit.Dp(800), unit.Dp(450)),
		)

		var ops *op.Ops

		for {
			evt := w.Event()

			switch typ := evt.(type) {

			case app.FrameEvent:
				ops = new(op.Ops)

				typ.Frame(ops)
			case app.DestroyEvent:
				os.Exit(0)
			}
		}
	}()
	app.Main()
}
