package main

import (
	"io"
	"log"
	"os"

	"gioui.org/app"
	"gioui.org/op"
	"gioui.org/unit"
	"github.com/GoldenFealla/VideoPlayerGo/internal/decoder"
	"github.com/GoldenFealla/VideoPlayerGo/internal/media"
	"github.com/asticode/go-astiav"
	"github.com/ebitengine/oto/v3"
)

var (
	WIDTH  = unit.Dp(800)
	HEIGHT = unit.Dp(450)
)

var (
	Input string = "./test.mp4"
)

var (
	audioStream  *decoder.AudioStream
	videoStream  *decoder.VideoStream
	synchronizer *media.Media
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
	ar, _ := io.Pipe()

	videoStream = decoder.NewVideoStream()
	audioStream = decoder.NewAudioStream(
		decoder.WithAudioFormat(astiav.SampleFormatFlt),
		decoder.WithSampleRate(44100),
		decoder.WithChannelLayout(astiav.ChannelLayoutStereo),
		decoder.WithNBSamples(1024),
	)

	synchronizer = media.NewMedia(videoStream, audioStream)
	if err := synchronizer.Load(Input); err != nil {
		log.Fatal(err)
	}

	otoCtx, readyChan, err := oto.NewContext(OtoOption)
	if err != nil {
		panic("oto.NewContext failed: " + err.Error())
	}
	<-readyChan
	AudioPlayer = otoCtx.NewPlayer(ar)
	AudioPlayer.SetVolume(0.5)
}

func main() {
	defer func() {
		videoStream.Close()
		audioStream.Close()
		synchronizer.Close()

		AudioPlayer.Close()
	}()

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

			// layout.Flex{}.Layout(
			// 	gtx,
			// 	layout.Flexed(1, widget.VideoFrame),
			// )

			typ.Frame(gtx.Ops)
		case app.DestroyEvent:
			return typ.Err
		}
	}
}
