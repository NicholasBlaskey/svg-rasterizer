package main

import (
	"syscall/js"

	"github.com/nicholasblaskey/webgl-utils/util"

	"github.com/nicholasblaskey/webgl/webgl"
)

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

	vertices := []float32{
		+0.0, +0.5,
		-0.5, -0.5,
		+0.5, -0.5,
	}
	buff := util.NewBufferVec2(gl)
	buff.BindData(gl, vertices)
	buff.BindToAttrib(gl, program, "position")

	gl.DrawArrays(webgl.TRIANGLES, 0, buff.VertexCount)
}
