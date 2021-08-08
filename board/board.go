package main

import (
	"fmt"

	"github.com/nicholasblaskey/webgl/webgl"
	"syscall/js"

	mgl "github.com/go-gl/mathgl/mgl32"
	"github.com/nicholasblaskey/webgl-utils/util"
)

type board struct {
	Width  int
	Height int
	gl     *webgl.Gl
	//
	positions     []float32
	positionsBuff *util.Buffer
	//
	texCoords     []float32
	texCoordsBuff *util.Buffer
	//
	texture       *webgl.Texture
	program       *webgl.Program
	colorsIndBuff *util.Buffer
	colorsInd     []float32 // Color indicator 0 => background, 1 => foreground
}

func New(canvas js.Value) (*board, error) {
	gl, err := webgl.FromCanvas(canvas)
	if err != nil {
		return nil, err
	}

	b := &board{Width: 50, Height: 50, gl: gl}

	err = b.initShaders()
	if err != nil {
		return nil, err
	}
	b.initTexCoords()
	b.initPositions()
	//b.initColorInd()
	b.initTexture()

	b.SetColors(mgl.Vec4{6 / 255.0, 35 / 255.0, 41 / 255.0, 1.0},
		mgl.Vec4{140 / 255.0, 222 / 255.0, 148 / 255.0})
	b.gl.ClearColor(0.3, 0.5, 0.3, 1.0)
	b.draw()

	return b, nil
}

func (b *board) initShaders() error {
	vertShader := `
			attribute vec2 a_position;
			attribute vec2 a_texCoord;			
			varying vec2 texCoord;
			void main() {
				gl_Position = vec4(a_position, 0.0, 1.0);
				texCoord = a_texCoord;
			}`

	fragShader := `
			precision mediump float;
			uniform sampler2D t;
			varying vec2 texCoord;
			void main() {
				gl_FragColor = texture2D(t, texCoord);
			}`

	program, err := util.CreateProgram(b.gl, vertShader, fragShader)
	if err != nil {
		return err
	}

	b.program = program
	return nil
}

func (b *board) initTexture() {
	b.texture = b.gl.CreateTexture()
	b.gl.ActiveTexture(0x84C0) // webgl.TEXTURE0 (TODO code gen this in)
	b.gl.BindTexture(webgl.TEXTURE_2D, b.texture)

	data := []byte{
		0, 0, 255, 255,
		255, 0, 0, 255,
		0, 255, 0, 255,
		255, 255, 255, 255,
	}
	b.gl.TexImage2DArray(webgl.TEXTURE_2D, 0, webgl.RGBA, 2, 2, 0,
		webgl.RGBA, webgl.UNSIGNED_BYTE, data)

	b.gl.TexParameteri(webgl.TEXTURE_2D, webgl.TEXTURE_WRAP_S, webgl.CLAMP_TO_EDGE)
	b.gl.TexParameteri(webgl.TEXTURE_2D, webgl.TEXTURE_WRAP_T, webgl.CLAMP_TO_EDGE)
	b.gl.TexParameteri(webgl.TEXTURE_2D, webgl.TEXTURE_MIN_FILTER, webgl.LINEAR)

	util.SetInt(b.gl, b.program, "t", 0)
}

func (b *board) initTexCoords() {
	b.texCoords = []float32{
		0.0, 1.0,
		0.0, 0.0,
		1.0, 0.0,
		0.0, 1.0,
		1.0, 0.0,
		1.0, 1.0,
	}

	b.texCoordsBuff = util.NewBufferVec2(b.gl)
	b.texCoordsBuff.BindData(b.gl, b.texCoords)
	b.texCoordsBuff.BindToAttrib(b.gl, b.program, "a_texCoord")
}

func (b *board) initPositions() {
	b.positions = []float32{
		-1.0, +1.0,
		-1.0, -1.0,
		+1.0, -1.0,
		-1.0, +1.0,
		+1.0, -1.0,
		+1.0, +1.0,
	}

	b.positionsBuff = util.NewBufferVec2(b.gl)
	b.positionsBuff.BindData(b.gl, b.positions)
	b.positionsBuff.BindToAttrib(b.gl, b.program, "a_position")
}

func (b *board) initColorInd() {
	b.colorsInd = make([]float32, 1) //b.Width*b.Height)
	fmt.Println(b.colorsInd)
	for i := 0; i < len(b.colorsInd); i += 2 {
		b.colorsInd[i] = 1.0
		break
	}

	b.colorsIndBuff = util.NewBufferFloat(b.gl)
	b.colorsIndBuff.BindData(b.gl, b.colorsInd)
	b.colorsIndBuff.BindToAttrib(b.gl, b.program, "a_colorInd")
}

func (b *board) SetColors(background, foreground mgl.Vec4) {
	util.SetVec4(b.gl, b.program, "background", background)
	util.SetVec4(b.gl, b.program, "foreground", foreground)
}

func (b *board) draw() {
	//fmt.Println(b.positions)
	b.gl.Clear(webgl.COLOR_BUFFER_BIT)
	b.gl.DrawArrays(webgl.TRIANGLES, 0, b.positionsBuff.VertexCount)
}

func main() {
	document := js.Global().Get("document")
	canvas := document.Call("getElementById", "webgl")
	canvas.Set("height", 400)
	canvas.Set("width", 400)
	b, err := New(canvas)
	if err != nil {
		panic(err)
	}

	_ = b

	/*
		gl, err := webgl.FromCanvas(canvas)
		if err != nil {
			panic(err)
		}

		gl.ClearColor(0.3, 0.5, 0.3, 1.0)
		gl.Clear(webgl.COLOR_BUFFER_BIT)

		vertShader := `
			attribute vec4 position;
			uniform mat4 m;
			void main() {
				gl_Position = m * position;
				gl_PointSize = 10.0;
			}`
		fragShader := `
			precision mediump float;
			uniform vec4 color;
			void main() {
				gl_FragColor = vec4(color);
			}`
		program, err := util.CreateProgram(gl, vertShader, fragShader)
		if err != nil {
			panic(err)
		}

		vertices := []float32{
			+0.0, +0.5,
			-0.5, -0.5,
			+0.5, -0.5,
		}
		buff := util.NewBufferVec2(gl)
		buff.BindData(gl, vertices)
		buff.BindToAttrib(gl, program, "position")

		util.SetVec4(gl, program, "color", mgl.Vec4{0.3, 0.9, 0.4, 1.0})
		util.SetMat4(gl, program, "m", mgl.Scale3D(0.5, 0.5, 0.9))

		gl.DrawArrays(webgl.TRIANGLES, 0, buff.VertexCount)
	*/
}
