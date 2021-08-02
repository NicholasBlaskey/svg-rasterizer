package main

import (
	"syscall/js"

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

	/*
			vertShader := `
		void main() {
			gl_Position = vec4(0.0, 0.0, 0.0, 1.0);
			gl_PointSize = 10.0;
		}`
			fragShader := `
		void main() {
			gl_FragColor = vec4(1.0, 0.0, 0.0, 1.0);
		}`
			if !util.InitShaders(gl, vertShader, fragShader) {
				fmt.Println("Failed to init shaders")
			}
	*/

	//gl.Call("clearColor", 0.0, 0.0, 0.0, 1.0)
	//gl.Call("clear", gl.Get("COLOR_BUFFER_BIT"))
	//gl.Call("drawArrays", gl.Get("POINTS"), 0, 1)

}
