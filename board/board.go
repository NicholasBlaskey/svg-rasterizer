package main

import (
	"fmt"

	"github.com/nicholasblaskey/webgl/webgl"
	"syscall/js"

	mgl "github.com/go-gl/mathgl/mgl32"
	"github.com/nicholasblaskey/webgl-utils/util"
)

type board struct {
	Width         int
	Height        int
	gl            *webgl.Gl
	colorsInd     []float32 // Color indicator 0 => background, 1 => foreground
	positions     []float32
	program       *webgl.Program
	posBuff       *util.Buffer
	colorsIndBuff *util.Buffer
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
	b.initPositions()
	b.initColorInd()

	b.SetColors(mgl.Vec4{6 / 255.0, 35 / 255.0, 41 / 255.0, 1.0},
		mgl.Vec4{140 / 255.0, 222 / 255.0, 148 / 255.0})
	b.gl.ClearColor(0.3, 0.5, 0.3, 1.0)
	b.draw()

	return b, nil
}

func (b *board) initShaders() error {
	vertShader := `
			attribute vec2 a_position;
			attribute float a_colorInd;	
			uniform mat4 m;
			varying float colorInd;
			void main() {
				gl_Position = vec4(0.0, 0.0, 0.0, 1.0); //vec4(a_position, 0.0, 1.0);
				gl_PointSize = 1.0;
				colorInd = a_colorInd;
			}`

	fragShader := `
			precision mediump float;
			uniform vec4 background;
			uniform vec4 foreground;
			varying float colorInd;
			void main() {
				gl_FragColor = vec4(1.0, 1.0, 1.0, 1.0);
			}`

	program, err := util.CreateProgram(b.gl, vertShader, fragShader)
	if err != nil {
		return err
	}

	b.program = program
	return nil
}

func (b *board) initPositions() {
	// Generate positions data
	b.positions = []float32{}
	for x := float32(0); x < float32(b.Width); x++ {
		for y := float32(0); y < float32(b.Height); y++ {
			b.positions = append(b.positions, x/float32(b.Width), y/float32(b.Height))
			break
		}
		break
	}
	fmt.Println(b.positions)

	// Initialize position buffer
	b.posBuff = util.NewBufferVec2(b.gl)
	b.posBuff.BindData(b.gl, b.positions)
	b.posBuff.BindToAttrib(b.gl, b.program, "a_position")

	// Set point size
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
	fmt.Println(b.posBuff.VertexCount)

	b.gl.Clear(webgl.COLOR_BUFFER_BIT)
	b.gl.DrawArrays(webgl.POINTS, 0, b.posBuff.VertexCount)
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
