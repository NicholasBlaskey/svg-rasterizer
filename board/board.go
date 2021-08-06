package main

import (
	"fmt"

	"github.com/nicholasblaskey/webgl/webgl"
	"syscall/js"

	"github.com/nicholasblaskey/webgl-utils/util"
	//mgl "github.com/go-gl/mathgl/mgl32"
)

type board struct {
	Width     int
	Height    int
	gl        *webgl.Gl
	data      []float32
	positions []float32

	program *webgl.Program
	buff    *util.Buffer
}

func New(canvas js.Value) (*board, error) {
	gl, err := webgl.FromCanvas(canvas)
	if err != nil {
		return nil, err
	}

	b := &board{Width: 400, Height: 400, gl: gl}

	err = b.initShaders()
	if err != nil {
		return nil, err
	}

	b.initPositions()

	b.gl.ClearColor(0.3, 0.5, 0.3, 1.0)
	b.draw()

	return b, nil
}

func (b *board) initShaders() error {
	vertShader := `
			attribute vec2 position;
			uniform mat4 m;
			void main() {
				gl_Position = vec4(position, 0, 1.0);
				gl_PointSize = 1.0;
			}`

	fragShader := `
			precision mediump float;
			//uniform vec4 color;
			void main() {
				gl_FragColor = vec4(0.3, 0.4, 0.8, 1.0); //vec4(color);
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
	for x := float32(0); x < 400.0; x++ {
		for y := float32(0); y < 400.0; y++ {
			b.positions = append(b.positions, x/400.0, y/400.0)
		}
	}

	// Initialize position buffer
	b.buff = util.NewBufferVec2(b.gl)
	b.buff.BindData(b.gl, b.positions)
	b.buff.BindToAttrib(b.gl, b.program, "position")

	// Set point size
}

func (b *board) draw() {
	//fmt.Println(b.positions)
	fmt.Println(b.buff.VertexCount)

	b.gl.Clear(webgl.COLOR_BUFFER_BIT)
	b.gl.DrawArrays(webgl.POINTS, 0, b.buff.VertexCount)
}

func main() {
	document := js.Global().Get("document")
	canvas := document.Call("getElementById", "webgl")
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
