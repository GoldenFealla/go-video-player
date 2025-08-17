package main

import (
	"log"
	"os"

	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/op"
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
	Input string = "./test.mp4"
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

	err = media.Open(Input)
	if err != nil {
		log.Fatalln(err)
	}

	vs, vcc, err := media.FindCodec(media.InputFormatContext, astiav.MediaTypeVideo)
	if err != nil {
		log.Fatalln(err)
	}

	err = decoder.InitVideo(vs, vcc, media.RecieveChan)
	if err != nil {
		log.Fatalln(err)
	}

	as, acc, err := media.FindCodec(media.InputFormatContext, astiav.MediaTypeAudio)
	if err != nil {
		log.Fatalln(err)
	}
	err = decoder.InitAudio(as, acc, media.SyncAudioWriter)
	if err != nil {
		log.Fatalln(err)
	}
	err = decoder.UpdateAudioFilter()
	if err != nil {
		log.Fatalln(err)
	}

	otoCtx, readyChan, err := oto.NewContext(OtoOption)
	if err != nil {
		panic("oto.NewContext failed: " + err.Error())
	}
	<-readyChan
	AudioPlayer = otoCtx.NewPlayer(media.SyncAudioReader)
	AudioPlayer.SetVolume(0.5)
}

func main() {
	defer func() {
		media.Free()
		decoder.Free()
		AudioPlayer.Close()
	}()

	go media.ReadPacket()
	go media.RunVideoReceiver()
	go media.RunVideoSync()

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

			layout.Flex{}.Layout(
				gtx,
				layout.Flexed(1, widget.VideoFrame),
				//layout.Flexed(1, widget.Controller),
			)

			typ.Frame(gtx.Ops)
		case app.DestroyEvent:
			return typ.Err
		}
	}
}
