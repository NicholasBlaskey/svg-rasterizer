package main

import (
	"github.com/nicholasblaskey/webgl/webgl"
	"syscall/js"

	"github.com/nicholasblaskey/webgl-utils/util"

	mgl "github.com/go-gl/mathgl/mgl32"
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
}
