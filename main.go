package main

import (
	"image"
	"log"
	"os"

	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"github.com/GoldenFealla/VideoPlayerGo/internal/media"
	"github.com/GoldenFealla/VideoPlayerGo/internal/media/audiodecoder"
	"github.com/GoldenFealla/VideoPlayerGo/internal/media/synchronizer"
	"github.com/GoldenFealla/VideoPlayerGo/internal/media/videodecoder"
	"github.com/asticode/go-astiav"
	"github.com/ebitengine/oto/v3"
)

type C = layout.Context
type D = layout.Dimensions

var (
	Input string = "./The Lost Beyond.mp4"
)

var (
	WIDTH  = unit.Dp(800)
	HEIGHT = unit.Dp(450)
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

	err = videodecoder.Init(vs, vcc, synchronizer.RecieveChan)
	if err != nil {
		log.Fatalln(err)
	}

	as, acc, err := media.FindCodec(media.InputFormatContext, astiav.MediaTypeAudio)
	if err != nil {
		log.Fatalln(err)
	}
	err = audiodecoder.Init(as, acc, synchronizer.SyncAudioWriter)
	if err != nil {
		log.Fatalln(err)
	}
	err = audiodecoder.UpdateFilter()
	if err != nil {
		log.Fatalln(err)
	}

	otoCtx, readyChan, err := oto.NewContext(OtoOption)
	if err != nil {
		panic("oto.NewContext failed: " + err.Error())
	}
	<-readyChan
	AudioPlayer = otoCtx.NewPlayer(synchronizer.SyncAudioReader)
	AudioPlayer.SetVolume(0)
}

func main() {
	defer func() {
		media.Free()

		videodecoder.Free()
		audiodecoder.Free()

		AudioPlayer.Close()
	}()

	go media.ReadPacket()
	go synchronizer.RunVideoSync()
	go synchronizer.RunAudioSync()

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
				layout.Rigid(VideoFrame),
			)

			typ.Frame(gtx.Ops)
		case app.DestroyEvent:
			return typ.Err
		}
	}
}

func VideoFrame(gtx C) D {
	gtx.Source.Execute(op.InvalidateCmd{})

	img, ok := <-synchronizer.OutputChan
	if !ok {
		return layout.Dimensions{
			Size: image.Pt(0, 0),
		}
	}

	imageOp := paint.NewImageOp(img)
	imageOp.Filter = paint.FilterNearest
	imageOp.Add(gtx.Ops)

	paint.PaintOp{}.Add(gtx.Ops)

	return layout.Dimensions{
		Size: image.Black.Bounds().Size(),
	}
}
