package main

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"
	"time"

	"GoldenFealla/go-video-player/player"

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

	initYUVTextures()
	shaderProgram = createYUVShader()
	initQuad()
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
	var totalBytesSent uint32 = 0

	go func() {
		for audiodata := range p.AudioOutput() {
			for sdl.GetQueuedAudioSize(audiodeviceid) > TargetTimeBytes {
				time.Sleep(time.Millisecond)
			}

			data := applyVolume(audiodata.Samples, volume)
			sdl.QueueAudio(audiodeviceid, data)

			// total := p.GetClock()
			totalBytesSent += uint32(len(data))
			queuedBytes := sdl.GetQueuedAudioSize(audiodeviceid)
			playedBytes := totalBytesSent - queuedBytes
			p.SetClock(playedBytes)
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
		imgui.Text(fmt.Sprintf("%.2fs", float32(p.GetClock())/float32(BytePerSecond)))
		imgui.End()

		imgui.Render()
		// --- render ---
		gl.Viewport(0, 0, int32(w), int32(h))
		gl.Clear(gl.COLOR_BUFFER_BIT)

		if len(latestFrame.Data) > 0 {
			renderYUV(latestFrame)
		}

		opengl3.RenderDrawData(imgui.CurrentDrawData())
		window.GLSwap()
	}
}

func applyVolume(data []byte, vol float32) []byte {
	// data is S16 LE (2 bytes per sample)
	out := make([]byte, len(data))
	for i := 0; i+1 < len(data); i += 2 {
		sample := int16(data[i]) | int16(data[i+1])<<8
		scaled := float32(sample) * vol
		// clamp
		if scaled > 32767 {
			scaled = 32767
		}
		if scaled < -32768 {
			scaled = -32768
		}
		s := int16(scaled)
		out[i] = byte(s)
		out[i+1] = byte(s >> 8)
	}
	return out
}

var shaderProgram uint32

func compileShader(source string, shaderType uint32) uint32 {
	shader := gl.CreateShader(shaderType)

	csource, free := gl.Strs(source + "\x00")
	gl.ShaderSource(shader, 1, csource, nil)
	free()

	gl.CompileShader(shader)

	var status int32
	gl.GetShaderiv(shader, gl.COMPILE_STATUS, &status)
	if status == gl.FALSE {
		var logLength int32
		gl.GetShaderiv(shader, gl.INFO_LOG_LENGTH, &logLength)

		log := string(make([]byte, logLength+1))
		gl.GetShaderInfoLog(shader, logLength, nil, gl.Str(log))

		panic("shader compile error: " + log)
	}

	return shader
}

func createYUVShader() uint32 {
	vertexShaderSource := `
		#version 130
		attribute vec2 position;
		attribute vec2 texCoord;
		varying vec2 vTexCoord;

		void main() {
			vTexCoord = texCoord;
			gl_Position = vec4(position, 0.0, 1.0);
		}
	`

	fragmentShaderSource := `
		#version 130
		varying vec2 vTexCoord;

		uniform sampler2D texY;
		uniform sampler2D texU;
		uniform sampler2D texV;

		void main() {
			float y = texture2D(texY, vTexCoord).r;
			float u = texture2D(texU, vTexCoord).r - 0.5;
			float v = texture2D(texV, vTexCoord).r - 0.5;

			float r = y + 1.402 * v;
			float g = y - 0.344 * u - 0.714 * v;
			float b = y + 1.772 * u;

			gl_FragColor = vec4(r, g, b, 1.0);
		}
	`

	vs := compileShader(vertexShaderSource, gl.VERTEX_SHADER)
	fs := compileShader(fragmentShaderSource, gl.FRAGMENT_SHADER)

	program := gl.CreateProgram()
	gl.AttachShader(program, vs)
	gl.AttachShader(program, fs)
	gl.LinkProgram(program)

	var status int32
	gl.GetProgramiv(program, gl.LINK_STATUS, &status)
	if status == gl.FALSE {
		panic("shader link failed")
	}

	gl.DeleteShader(vs)
	gl.DeleteShader(fs)

	return program
}

var texY, texU, texV uint32
var vao, vbo uint32

func initYUVTextures() {
	gl.GenTextures(1, &texY)
	gl.GenTextures(1, &texU)
	gl.GenTextures(1, &texV)

	setup := func(tex uint32) {
		gl.BindTexture(gl.TEXTURE_2D, tex)
		gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
		gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
		gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
		gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
	}

	setup(texY)
	setup(texU)
	setup(texV)
}

func initQuad() {
	verts := []float32{
		-1, 1, 0, 0,
		1, 1, 1, 0,
		1, -1, 1, 1,
		-1, -1, 0, 1,
	}
	gl.GenVertexArrays(1, &vao)
	gl.GenBuffers(1, &vbo)
	gl.BindVertexArray(vao)
	gl.BindBuffer(gl.ARRAY_BUFFER, vbo)
	gl.BufferData(gl.ARRAY_BUFFER, len(verts)*4, gl.Ptr(verts), gl.STATIC_DRAW)

	posLoc := uint32(gl.GetAttribLocation(shaderProgram, gl.Str("position\x00")))
	uvLoc := uint32(gl.GetAttribLocation(shaderProgram, gl.Str("texCoord\x00")))

	gl.EnableVertexAttribArray(posLoc)
	gl.VertexAttribPointer(posLoc, 2, gl.FLOAT, false, 4*4, gl.PtrOffset(0))
	gl.EnableVertexAttribArray(uvLoc)
	gl.VertexAttribPointer(uvLoc, 2, gl.FLOAT, false, 4*4, gl.PtrOffset(2*4))
	gl.BindVertexArray(0)
}

var lastW, lastH int

func renderYUV(frame player.VideoData) {
	ySize := frame.W * frame.H
	uvSize := (frame.W / 2) * (frame.H / 2)
	y := frame.Data[:ySize]
	u := frame.Data[ySize : ySize+uvSize]
	v := frame.Data[ySize+uvSize:]

	upload := func(tex uint32, data []byte, w, h int) {
		gl.BindTexture(gl.TEXTURE_2D, tex)
		if frame.W != lastW || frame.H != lastH {
			gl.TexImage2D(gl.TEXTURE_2D, 0, gl.R8,
				int32(w), int32(h), 0,
				gl.RED, gl.UNSIGNED_BYTE, gl.Ptr(data))
		} else {
			gl.TexSubImage2D(gl.TEXTURE_2D, 0, 0, 0,
				int32(w), int32(h),
				gl.RED, gl.UNSIGNED_BYTE, gl.Ptr(data))
		}
	}
	upload(texY, y, frame.W, frame.H)
	upload(texU, u, frame.W/2, frame.H/2)
	upload(texV, v, frame.W/2, frame.H/2)
	lastW, lastH = frame.W, frame.H

	gl.UseProgram(shaderProgram)
	gl.Uniform1i(gl.GetUniformLocation(shaderProgram, gl.Str("texY\x00")), 0)
	gl.Uniform1i(gl.GetUniformLocation(shaderProgram, gl.Str("texU\x00")), 1)
	gl.Uniform1i(gl.GetUniformLocation(shaderProgram, gl.Str("texV\x00")), 2)

	gl.ActiveTexture(gl.TEXTURE0)
	gl.BindTexture(gl.TEXTURE_2D, texY)
	gl.ActiveTexture(gl.TEXTURE1)
	gl.BindTexture(gl.TEXTURE_2D, texU)
	gl.ActiveTexture(gl.TEXTURE2)
	gl.BindTexture(gl.TEXTURE_2D, texV)

	gl.BindVertexArray(vao)
	gl.DrawArrays(gl.TRIANGLE_FAN, 0, 4)
	gl.BindVertexArray(0)
}
