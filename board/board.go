package main

import (
	"syscall/js"

	"github.com/nicholasblaskey/webgl-utils/util"

	"github.com/nicholasblaskey/webgl/webgl"
)

func initBuffer(gl *webgl.Gl, program *webgl.Program, attribute string, vertices []float32) {
	vertexBuffer := gl.CreateBuffer()
	if vertexBuffer == nil {
		return
	}

	gl.BindBuffer(webgl.ARRAY_BUFFER, vertexBuffer)
	gl.BufferData(webgl.ARRAY_BUFFER, vertices, webgl.STATIC_DRAW)

	attribLoc := gl.GetAttribLocation(program, attribute)
	if attribLoc < 0 {
		return
	}

	gl.VertexAttribPointer(attribLoc, 2, webgl.FLOAT, false, 0, 0)
	gl.EnableVertexAttribArray(attribLoc)
}

func main() {
	document := js.Global().Get("document")
	canvas := document.Call("getElementById", "webgl")
	gl, err := webgl.FromCanvas(canvas)
	if err != nil {
		panic(err)
	}

	gl.ClearColor(0.3, 0.5, 0.3, 1.0)
	gl.Clear(webgl.COLOR_BUFFER_BIT)

	vertShader := `
		attribute vec4 position;
		void main() {
			gl_Position = position;
			gl_PointSize = 10.0;
		}`
	fragShader := `
		void main() {
			gl_FragColor = vec4(1.0, 0.0, 0.0, 1.0);
		}`
	program, err := util.CreateProgram(gl, vertShader, fragShader)
	if err != nil {
		panic(err)
	}
	_ = program

	vertices := []float32{
		+0.0, +0.5,
		-0.5, -0.5,
		+0.5, -0.5,
	}
	initBuffer(gl, program, "position", vertices)

	//gl.Call("clearColor", 0.0, 0.0, 0.0, 1.0)
	//gl.Call("clear", gl.Get("COLOR_BUFFER_BIT"))
	gl.DrawArrays(webgl.TRIANGLES, 0, 3)

}
