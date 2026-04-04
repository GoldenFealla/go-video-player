package shader

import (
	"GoldenFealla/go-video-player/player"

	"github.com/go-gl/gl/v4.6-compatibility/gl"
)

var programyuv uint32

func Init() {
	initYUVTextures()
	programyuv = createYUVShader()
	initQuad()
}

// Vertex Array Object
var vao uint32

// Vertex Buffer Object
var vbo uint32

func initQuad() {
	verts := []float32{
		-1, 1, 0, 0,
		1, 1, 1, 0,
		1, -1, 1, 1,
		-1, -1, 0, 1,
	}

	// Create VAO, VBO handles
	gl.GenVertexArrays(1, &vao)
	gl.GenBuffers(1, &vbo)
	gl.BindVertexArray(vao)
	gl.BindBuffer(gl.ARRAY_BUFFER, vbo)
	gl.BufferData(gl.ARRAY_BUFFER, len(verts)*4, gl.Ptr(verts), gl.STATIC_DRAW)

	posLoc := uint32(gl.GetAttribLocation(programyuv, gl.Str("position\x00")))
	uvLoc := uint32(gl.GetAttribLocation(programyuv, gl.Str("texCoord\x00")))

	gl.EnableVertexAttribArray(posLoc)
	gl.VertexAttribPointerWithOffset(posLoc, 2, gl.FLOAT, false, 4*4, 0)

	gl.EnableVertexAttribArray(uvLoc)
	gl.VertexAttribPointerWithOffset(uvLoc, 2, gl.FLOAT, false, 4*4, 2*4)

	gl.BindVertexArray(0)
}

var texY, texU, texV uint32

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

func compile(source string, shaderType uint32) uint32 {
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

	vs := compile(vertexShaderSource, gl.VERTEX_SHADER)
	fs := compile(fragmentShaderSource, gl.FRAGMENT_SHADER)

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

var lastW, lastH int

const (
	FullSize int32 = 0
)

const (
	ZeroOffsetX = 0
	ZeroOffsetY = 0
	ZeroBorder  = 0
)

func RenderYUV(frame player.VideoData) {
	ySize := frame.W * frame.H
	uvSize := ySize / 4
	y := frame.Data[:ySize]
	u := frame.Data[ySize : ySize+uvSize]
	v := frame.Data[ySize+uvSize:]

	upload := func(tex uint32, data []byte, w, h int) {
		gl.BindTexture(gl.TEXTURE_2D, tex)
		if frame.W != lastW || frame.H != lastH {
			gl.TexImage2D(
				gl.TEXTURE_2D,
				FullSize,
				gl.R8,
				int32(w),
				int32(h),
				ZeroBorder,
				gl.RED,
				gl.UNSIGNED_BYTE,
				gl.Ptr(data),
			)
		} else {
			gl.TexSubImage2D(
				gl.TEXTURE_2D,
				FullSize,
				ZeroOffsetX,
				ZeroOffsetY,
				int32(w),
				int32(h),
				gl.RED,
				gl.UNSIGNED_BYTE,
				gl.Ptr(data),
			)
		}
	}

	upload(texY, y, frame.W, frame.H)
	upload(texU, u, frame.W/2, frame.H/2)
	upload(texV, v, frame.W/2, frame.H/2)

	lastW, lastH = frame.W, frame.H

	gl.UseProgram(programyuv)
	gl.Uniform1i(gl.GetUniformLocation(programyuv, gl.Str("texY\x00")), 0)
	gl.Uniform1i(gl.GetUniformLocation(programyuv, gl.Str("texU\x00")), 1)
	gl.Uniform1i(gl.GetUniformLocation(programyuv, gl.Str("texV\x00")), 2)

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
