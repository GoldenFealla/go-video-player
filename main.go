package main

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"GoldenFealla/go-video-player/player"

	"github.com/asticode/go-astiav"
	"github.com/veandco/go-sdl2/sdl"
)

func init() {
	astiav.SetLogLevel(astiav.LogLevelError)
	astiav.SetLogCallback(func(c astiav.Classer, l astiav.LogLevel, fmt, msg string) {
		var cs string
		if c != nil {
			if cl := c.Class(); cl != nil {
				cs = " - class: " + cl.String()
			}
		}
		log.Printf("ffmpeg log: %s%s - level: %d\n", strings.TrimSpace(msg), cs, l)
	})
}

func main() {
	log.Printf("Starting program as PID: %d\n", os.Getpid())

	if len(os.Args) < 2 {
		log.Fatal("Usage: go run main.go <video_file>")
	}
	inputPath := os.Args[1]

	fileName := filepath.Base(inputPath)

	sdl.Init(sdl.INIT_VIDEO | sdl.INIT_AUDIO)
	defer sdl.Quit()

	spec := sdl.AudioSpec{
		Freq:     48000,
		Format:   sdl.AUDIO_F32,
		Channels: 2,
		Samples:  128,
	}
	deviceID, err := sdl.OpenAudioDevice("", false, &spec, nil, 0)
	if err != nil {
		panic(err)
	}
	defer sdl.CloseAudioDevice(deviceID)
	sdl.PauseAudioDevice(deviceID, false)

	window, _ := sdl.CreateWindow(
		fileName,
		sdl.WINDOWPOS_CENTERED,
		sdl.WINDOWPOS_CENTERED,
		800, 600,
		sdl.WINDOW_SHOWN|sdl.WINDOW_RESIZABLE,
	)
	renderer, _ := sdl.CreateRenderer(window, -1, 0)

	var texture *sdl.Texture

	p := player.NewPlayer()
	p.Clock.DeviceID = deviceID

	go p.Play(inputPath)

	for {
		for event := sdl.PollEvent(); event != nil; event = sdl.PollEvent() {
			switch event.(type) {
			case *sdl.QuitEvent:
				return
			}
		}

		select {
		case vf := <-p.VideoOut:
			if texture == nil {
				texture, _ = renderer.CreateTexture(
					uint32(sdl.PIXELFORMAT_IYUV),
					sdl.TEXTUREACCESS_STREAMING,
					int32(vf.W),
					int32(vf.H),
				)

				renderer.SetLogicalSize(int32(vf.W), int32(vf.H))
			}

			texture.UpdateYUV(nil, vf.Y, vf.W, vf.U, vf.W/2, vf.V, vf.W/2)
			renderer.Copy(texture, nil, nil)
			renderer.Present()

		case af := <-p.AudioOut:
			sdl.QueueAudio(deviceID, af.Samples)
			p.Clock.UpdateAudio(af.NbSamples)

		default:
			time.Sleep(time.Millisecond)
		}
	}
}
