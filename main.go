package main

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"
	"time"

	"GoldenFealla/go-video-player/player"
	"GoldenFealla/go-video-player/shader"

	"github.com/AllenDang/cimgui-go/imgui"
	"github.com/AllenDang/cimgui-go/impl/opengl3"
	"github.com/asticode/go-astiav"
	"github.com/go-gl/gl/v4.6-compatibility/gl"
	"github.com/veandco/go-sdl2/sdl"
)

var p *player.Player

var (
	DefaultSampleRate = 48000
	DefaultChannels   = 2
	DefaultBufferSize = 1024

	DefaultTargetTime = 25 //in milisecond

	BytePerSecond   = DefaultSampleRate * DefaultChannels * 2
	TargetTimeBytes = uint32(DefaultSampleRate*DefaultChannels*2*DefaultTargetTime) / 1000
)

func init() {
	runtime.LockOSThread()
	fmt.Println(TargetTimeBytes)

	log.Printf("Starting program as PID: %d\n", os.Getpid())
	sdl.Init(sdl.INIT_AUDIO | sdl.INIT_VIDEO)

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
	defer sdl.Quit()
	// ==== AUDIO =====
	log.Printf("Use audio driver: %v\n", sdl.GetCurrentAudioDriver())
	spec := sdl.AudioSpec{
		Freq:     int32(DefaultSampleRate),
		Format:   sdl.AUDIO_S16,
		Channels: uint8(DefaultChannels),
		Samples:  uint16(DefaultBufferSize),
	}
	audiodeviceid, err := sdl.OpenAudioDevice("", false, &spec, nil, 0)
	if err != nil {
		panic(err)
	}
	log.Printf("opened audio device id: %v\n", audiodeviceid)
	defer sdl.CloseAudioDevice(audiodeviceid)
	sdl.PauseAudioDevice(audiodeviceid, false)

	// ====== GUI ======
	window, err := sdl.CreateWindow(
		"Player",
		sdl.WINDOWPOS_CENTERED,
		sdl.WINDOWPOS_CENTERED,
		1280, 720,
		sdl.WINDOW_OPENGL|sdl.WINDOW_RESIZABLE,
	)
	if err != nil {
		panic(err)
	}

	glContext, err := window.GLCreateContext()
	if err != nil {
		panic(err)
	}
	err = window.GLMakeCurrent(glContext)
	if err != nil {
		panic(err)
	}

	if err := gl.Init(); err != nil {
		panic(err)
	}

	shader.Init()

	gl.PixelStorei(gl.UNPACK_ALIGNMENT, 1)

	log.Println("GL version:", gl.GoStr(gl.GetString(gl.VERSION)))

	imgui.CreateContext()

	io := imgui.CurrentIO()
	io.SetBackendPlatformName("custom-sdl")
	io.SetBackendRendererName("opengl")

	opengl3.InitV("#version 460 compatibility")
	opengl3.CreateDeviceObjects()
	defer opengl3.DestroyDeviceObjects()

	p = player.NewPlayer()
	go p.Play("test_video_1.mp4")

	var latestFrame player.VideoData
	var volume float32 = 0.5

	go func() {
		var sent int64 = 0
		for audiodata := range p.AudioOutput() {
			for sdl.GetQueuedAudioSize(audiodeviceid) > TargetTimeBytes {
				time.Sleep(time.Millisecond)
			}

			data := applyVolume(audiodata.Samples, volume)
			sdl.QueueAudio(audiodeviceid, data)

			sent += int64(len(data))
			queued := int64(sdl.GetQueuedAudioSize(audiodeviceid))
			p.SetCurrent(sent - queued)
		}
	}()

	for {
		for event := sdl.PollEvent(); event != nil; event = sdl.PollEvent() {
			switch e := event.(type) {

			case *sdl.MouseMotionEvent:
				io.AddMousePosEvent(float32(e.X), float32(e.Y))

			case *sdl.MouseButtonEvent:
				io.AddMouseButtonEvent(int32(e.Button-1), e.State == sdl.PRESSED)

			case *sdl.MouseWheelEvent:
				io.AddMouseWheelEvent(float32(e.X), float32(e.Y))

			case *sdl.TextInputEvent:
				io.AddInputCharactersUTF8(string(e.Text[:]))

			case *sdl.QuitEvent:
				return
			}
		}

		select {
		case videodata := <-p.VideoOutput():
			latestFrame = videodata
		default:
		}

		w, h := window.GetSize()
		io.SetDisplaySize(
			imgui.Vec2{
				X: float32(w),
				Y: float32(h),
			},
		)

		// ==== frame ====
		imgui.NewFrame()

		// ----- gui -----
		imgui.Begin("Control")
		imgui.SliderFloat("volume", &volume, 0, 1)
		imgui.Text(fmt.Sprintf("%.2fs", p.GetSecond()))
		imgui.End()

		imgui.Render()
		// --- render ---
		gl.Viewport(0, 0, int32(w), int32(h))
		gl.Clear(gl.COLOR_BUFFER_BIT)

		if len(latestFrame.Data) > 0 {
			shader.RenderYUV(latestFrame)
		}

		opengl3.RenderDrawData(imgui.CurrentDrawData())
		window.GLSwap()
	}
}

func applyVolume(data []byte, vol float32) []byte {
	// data is S16 LE (2 bytes per sample)
	out := make([]byte, len(data))
	for i := 0; i+1 < len(data); i += 2 {
		// - - - - - - - - b b b b b b b b
		// b b b b b b b b - - - - - - - -
		// b b b b b b b b b b b b b b b b
		// scale then return
		sample := int16(data[i]) | int16(data[i+1])<<8
		scaled := float32(sample) * vol
		scaled = min(scaled, 32767)
		scaled = max(scaled, -32768)
		s := int16(scaled)
		out[i] = byte(s)
		out[i+1] = byte(s >> 8)
	}
	return out
}
