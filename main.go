package main

import (
	"image"
	"image/color"
	"io"
	"log"
	"os"

	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"github.com/GoldenFealla/VideoPlayerGo/internal/decoder"
	"github.com/GoldenFealla/VideoPlayerGo/internal/media"
	"github.com/GoldenFealla/VideoPlayerGo/internal/widget"
	"github.com/asticode/go-astiav"
	"github.com/ebitengine/oto/v3"
)

var (
	WIDTH  = unit.Dp(800)
	HEIGHT = unit.Dp(450)
)

var (
	Input string = "./Daisy Crown.mp4"
)

var (
	audioStream *decoder.AudioStream
	videoStream *decoder.VideoStream
	Media       *media.Media

	outputVideo chan image.Image
)

// Oto
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
	inputAudio, outputAudio := io.Pipe()
	outputVideo = make(chan image.Image, 3)

	videoStream = decoder.NewVideoStream()
	audioStream = decoder.NewAudioStream(
		decoder.WithAudioFormat(astiav.SampleFormatFlt),
		decoder.WithSampleRate(44100),
		decoder.WithChannelLayout(astiav.ChannelLayoutStereo),
		decoder.WithNBSamples(1024),
	)

	Media = media.NewMedia(videoStream, audioStream, outputVideo, outputAudio)
	if err := Media.Load(Input); err != nil {
		log.Fatal(err)
	}

	otoCtx, readyChan, err := oto.NewContext(OtoOption)
	if err != nil {
		panic("oto.NewContext failed: " + err.Error())
	}
	<-readyChan
	AudioPlayer = otoCtx.NewPlayer(inputAudio)
	AudioPlayer.SetVolume(0.25)
}

func main() {
	defer func() {
		videoStream.Close()
		audioStream.Close()
		Media.Close()

		AudioPlayer.Close()
	}()

	go Media.Decode()
	go func() {
		w := new(app.Window)
		w.Option(
			app.Title("Video player"),
			app.Size(WIDTH, HEIGHT),
		)
		if err := draw(w); err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}()

	AudioPlayer.Play()

	app.Main()
}

func draw(window *app.Window) error {
	var ops op.Ops

	for {
		evt := window.Event()

		switch typ := evt.(type) {

		case app.FrameEvent:
			gtx := app.NewContext(&ops, typ)

			paint.Fill(gtx.Ops, color.NRGBA{
				R: 0,
				G: 0,
				B: 0,
				A: 255,
			})

			layout.Flex{}.Layout(
				gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return widget.VideoFrame(gtx, outputVideo)
				}),
			)

			typ.Frame(gtx.Ops)
		case app.DestroyEvent:
			return typ.Err
		}
	}
}
