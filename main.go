package main

import (
	"fmt"
	"log"
	"math"
	"os"
	"runtime"
	"strings"

	"GoldenFealla/go-video-player/codec"
	"GoldenFealla/go-video-player/player"
	"GoldenFealla/go-video-player/shader"

	"github.com/AllenDang/cimgui-go/imgui"
	"github.com/AllenDang/cimgui-go/impl/opengl3"
	"github.com/asticode/go-astiav"
	"github.com/go-gl/gl/v4.6-compatibility/gl"
	"github.com/veandco/go-sdl2/sdl"
)

var p *player.Player

// var (
// 	DefaultSampleRate = 48000
// 	DefaultChannels   = 2
// 	DefaultBufferSize = 1024

// 	DefaultTargetTime = 25 //in milisecond

// 	BytePerSecond   = DefaultSampleRate * DefaultChannels * 2
// 	TargetTimeBytes = uint32(DefaultSampleRate*DefaultChannels*2*DefaultTargetTime) / 1000
// )

func init() {
	runtime.LockOSThread()

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

	if err := sdl.GLSetSwapInterval(1); err != nil {
		log.Println("Failed to enable vsync:", err)
	}

	if err := gl.Init(); err != nil {
		panic(err)
	}

	shader.Init()
	gl.PixelStorei(gl.UNPACK_ALIGNMENT, 1)
	log.Println("GL version:", gl.GoStr(gl.GetString(gl.VERSION)))

	imgui.CreateContext()

	// IO
	io := imgui.CurrentIO()
	io.SetBackendPlatformName("custom-sdl")
	io.SetBackendRendererName("opengl")

	// OpenGL Impl from cimgui-go package
	opengl3.InitV("#version 460 compatibility")
	opengl3.CreateDeviceObjects()
	defer opengl3.DestroyDeviceObjects()

	p = player.NewPlayer()
	p.Load("test_video_1.mp4")

	go p.Play()
	go p.Clock()

	var latestFrame codec.VideoData

	// ====== LOOP =====
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

		f := p.LatestFrame()
		if len(f.Data) > 0 {
			latestFrame = f
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
		barHeight := float32(40)

		imgui.SetNextWindowPos(imgui.Vec2{
			X: 0,
			Y: float32(h) - barHeight,
		})

		imgui.SetNextWindowSize(imgui.Vec2{
			X: float32(w),
			Y: barHeight,
		})

		flags := imgui.WindowFlagsNoTitleBar |
			imgui.WindowFlagsNoResize |
			imgui.WindowFlagsNoMove |
			imgui.WindowFlagsNoCollapse

		imgui.BeginV("Control", nil, flags)
		avail := imgui.ContentRegionAvail().X

		imgui.PushItemWidth(avail * 0.7)
		imgui.SliderFloat("##second", &p.Second, 0, p.Duration)
		imgui.PopItemWidth()

		imgui.SameLine()

		imgui.PushItemWidth(avail * 0.2)
		imgui.SliderFloat("##volume", &p.Volume, 0, 1)
		imgui.PopItemWidth()

		imgui.SameLine()

		imgui.PushItemWidth(avail * 0.1)
		imgui.Text(fmt.Sprintf("%s/%s", formatDuration(p.Second), formatDuration(p.Duration)))
		imgui.PopItemWidth()

		imgui.End()
		imgui.Render()

		// --- render ---
		gl.Viewport(0, 0, int32(w), int32(h))
		gl.Clear(gl.COLOR_BUFFER_BIT)

		if len(latestFrame.Data) > 0 {
			shader.RenderYUV(latestFrame, int(w), int(h))
		}

		opengl3.RenderDrawData(imgui.CurrentDrawData())
		window.GLSwap()
	}
}

func formatDuration(sec float32) string {
	totalSeconds := int(math.Round(float64(sec)))

	m := totalSeconds / 60
	s := totalSeconds % 60

	return fmt.Sprintf("%02d:%02d", m, s)
}
