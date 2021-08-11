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
	canvas js.Value
	//
	ZoomFactor       float32
	TranslationSpeed float32
	translation      mgl.Vec2
	//
	positions     []float32
	positionsBuff *util.Buffer
	//
	texCoords     []float32
	texCoordsBuff *util.Buffer
	//
	pixelPos              []float32
	pixelPosBuff          *util.Buffer
	pixelInspectorProgram *webgl.Program
	pixelInspectorOn      bool
	mouseX                int
	mouseY                int
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

	// TODO ensure (width * height) % 4 == 0
	//b := &board{Width: canvas.Get("height").Int(), Height: canvas.Get("width").Int(),
	//
	b := &board{Width: 12, Height: 12,
		gl: gl, canvas: canvas, ZoomFactor: 0.05, TranslationSpeed: 0.003,
	}

	err = b.initShaders()
	if err != nil {
		return nil, err
	}
	b.initTexCoords()
	b.initPositions()
	//b.initColorInd()
	b.initTexture()

	b.initZoomListener()
	b.initTranslationListener()
	b.initPixelInspector()

	b.SetColors(mgl.Vec4{6 / 255.0, 35 / 255.0, 41 / 255.0, 1.0},
		mgl.Vec4{140 / 255.0, 222 / 255.0, 148 / 255.0, 1.0})
	b.gl.ClearColor(0.3, 0.5, 0.3, 1.0)
	b.draw()

	return b, nil
}

func (b *board) EnablePixelInspector(shouldTurnOn bool) {
	b.pixelInspectorOn = shouldTurnOn
	b.draw()
}

func (b *board) initPixelInspector() {
	// Always have the pixel inspector on and listening
	js.Global().Call("addEventListener", "mousemove",
		js.FuncOf(func(this js.Value, args []js.Value) interface{} {
			x, y := getXAndYFromEvent(args[0])
			b.mouseX, b.mouseY = int(x), int(y)

			if b.pixelInspectorOn {
				b.draw()
			}

			return nil
		}))
}

func getXAndYFromEvent(e js.Value) (float32, float32) {
	x := float32(e.Get("offsetX").Float())
	y := float32(e.Get("offsetY").Float())

	return x, y
}

func (b *board) initTranslationListener() {
	isDown := false
	xStart, yStart := float32(0.0), float32(0.0)
	b.canvas.Call("addEventListener", "mousedown",
		js.FuncOf(func(this js.Value, args []js.Value) interface{} {
			isDown = true
			xStart, yStart = getXAndYFromEvent(args[0])

			return nil
		}))

	js.Global().Call("addEventListener", "mouseup",
		js.FuncOf(func(this js.Value, args []js.Value) interface{} {
			if isDown {
				xNew, yNew := getXAndYFromEvent(args[0])
				b.applyTranslation(xStart, yStart, xNew, yNew)
				xStart, yStart = 0, 0
				isDown = false
			}

			return nil
		}))

	js.Global().Call("addEventListener", "mousemove",
		js.FuncOf(func(this js.Value, args []js.Value) interface{} {
			if isDown {
				xNew, yNew := getXAndYFromEvent(args[0])
				b.applyTranslation(xStart, yStart, xNew, yNew)
				xStart, yStart = xNew, yNew
			}

			return nil
		}))
}

func (b *board) applyTranslation(xStart, yStart, x, y float32) {
	b.translation = b.translation.Add(mgl.Vec2{x - xStart, yStart - y}.Mul(b.TranslationSpeed))

	b.gl.UseProgram(b.program)
	util.SetVec2(b.gl, b.program, "translation", b.translation)
	b.draw()
}

func (b *board) initZoomListener() {
	zoomValue := float32(1.0)
	b.setZoom(zoomValue)

	eventFunc := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		val := args[0].Get("deltaY").Float()
		if val > 0 {
			zoomValue -= b.ZoomFactor
			if zoomValue < 0 {
				zoomValue = 0
			}
		} else {
			zoomValue += b.ZoomFactor
		}
		b.setZoom(zoomValue)
		b.draw()
		return nil
	})

	js.Global().Call("addEventListener", "wheel", eventFunc)
}

func (b *board) setZoom(zoom float32) {
	b.gl.UseProgram(b.program)
	util.SetFloat(b.gl, b.program, "zoomFactor", zoom)
}

func (b *board) initShaders() error {
	vertShader := `
			attribute vec2 a_position;
			attribute vec2 a_texCoord;			
			varying vec2 texCoord;

			uniform vec2 translation;
			uniform float zoomFactor;
			void main() {
				gl_Position = vec4(a_position * zoomFactor + translation, 0.0, 1.0);
				texCoord = a_texCoord;
			}`

	fragShader := `
			precision mediump float;
			uniform sampler2D t;
			varying vec2 texCoord;
			uniform vec4 foreground;
			uniform vec4 background;
			void main() {
				float alpha = texture2D(t, texCoord).a;
				gl_FragColor = alpha * foreground + (1.0 - alpha) * background;
			}`

	program, err := util.CreateProgram(b.gl, vertShader, fragShader)
	if err != nil {
		return err
	}
	b.program = program

	vertShader = `
			attribute vec2 a_position;

			void main() {
				gl_Position = vec4(a_position, 0.0, 1.0);
			}`

	fragShader = `
			precision mediump float;
			void main() {
				gl_FragColor = vec4(1.0, 0.0, 1.0, 1.0); 
			}`
	pixelProgram, err := util.CreateProgram(b.gl, vertShader, fragShader)
	if err != nil {
		return err
	}
	b.pixelInspectorProgram = pixelProgram

	return nil
}

func (b *board) initTexture() {
	b.texture = b.gl.CreateTexture()
	b.gl.ActiveTexture(webgl.TEXTURE0)
	b.gl.BindTexture(webgl.TEXTURE_2D, b.texture)

	data := make([]byte, b.Width*b.Height)
	b.setTextureData(data)

	b.gl.TexParameteri(webgl.TEXTURE_2D, webgl.TEXTURE_WRAP_S, webgl.CLAMP_TO_EDGE)
	b.gl.TexParameteri(webgl.TEXTURE_2D, webgl.TEXTURE_WRAP_T, webgl.CLAMP_TO_EDGE)
	b.gl.TexParameteri(webgl.TEXTURE_2D, webgl.TEXTURE_MIN_FILTER, webgl.NEAREST)
	b.gl.TexParameteri(webgl.TEXTURE_2D, webgl.TEXTURE_MAG_FILTER, webgl.NEAREST)

	b.gl.UseProgram(b.program)
	util.SetInt(b.gl, b.program, "t", 0)
}

func (b *board) setTextureData(data []byte) {
	b.gl.TexImage2DArray(webgl.TEXTURE_2D, 0, webgl.ALPHA, b.Width, b.Height, 0,
		webgl.ALPHA, webgl.UNSIGNED_BYTE, data)
}

func (b *board) SetPixels(data []byte) {
	b.setTextureData(data)
	b.draw()
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

	b.gl.UseProgram(b.program)
	b.texCoordsBuff = util.NewBufferVec2(b.gl)
	b.texCoordsBuff.BindData(b.gl, b.texCoords)
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

	b.gl.UseProgram(b.program)
	b.positionsBuff = util.NewBufferVec2(b.gl)
	b.positionsBuff.BindData(b.gl, b.positions)

	b.pixelPos = []float32{
		-0.99, +0.99,
		-0.99, -0.99,
		+0.99, -0.99,
		-0.99, +0.99,
		+0.99, -0.99,
		+0.99, +0.99,
	}
	b.gl.UseProgram(b.pixelInspectorProgram)
	b.pixelPosBuff = util.NewBufferVec2(b.gl)
	b.pixelPosBuff.BindData(b.gl, b.pixelPos)
}

func (b *board) SetColors(background, foreground mgl.Vec4) {
	b.gl.UseProgram(b.program)
	util.SetVec4(b.gl, b.program, "background", background)
	util.SetVec4(b.gl, b.program, "foreground", foreground)
}

func (b *board) draw() {
	w, h := b.canvas.Get("width").Int(), b.canvas.Get("height").Int()

	// Draw the texture.
	b.gl.Viewport(0.0, 0.0, w, h) // TODO change when we change the canvas size???

	b.gl.UseProgram(b.program)
	b.positionsBuff.BindToAttrib(b.gl, b.program, "a_position")
	b.texCoordsBuff.BindToAttrib(b.gl, b.program, "a_texCoord")

	b.gl.ClearColor(1.0, 1.0, 1.0, 1.0)
	b.gl.Clear(webgl.COLOR_BUFFER_BIT)
	b.gl.DrawArrays(webgl.TRIANGLES, 0, b.positionsBuff.VertexCount)

	if !b.pixelInspectorOn {
		return
	}

	// Draw the pixel inpector
	b.gl.Viewport(w/2, h/2, w/2, h/2)

	// Draw black box around viewport to be used for pixel borders.
	b.gl.Enable(webgl.SCISSOR_TEST)
	b.gl.Scissor(w/2, h/2, w/2, h/2)
	b.gl.ClearColor(0.0, 0, 0.0, 1.0)
	b.gl.Clear(webgl.COLOR_BUFFER_BIT)
	b.gl.Disable(webgl.SCISSOR_TEST)

	b.gl.UseProgram(b.pixelInspectorProgram)
	b.pixelPosBuff.BindToAttrib(b.gl, b.pixelInspectorProgram, "a_position")
	b.gl.DrawArrays(webgl.TRIANGLES, 0, b.pixelPosBuff.VertexCount)

	// Read the pixels from the texture.

	// Draw
}

func main() {
	document := js.Global().Get("document")
	canvas := document.Call("getElementById", "webgl")
	canvas.Set("height", 800)
	canvas.Set("width", 800)
	b, err := New(canvas)
	if err != nil {
		panic(err)
	}
	b.EnablePixelInspector(true)

	fmt.Println("starting")

	data := []byte{}
	white := false
	for i := 0; i < b.Width; i++ {
		white = i%2 == 0
		for j := 0; j < b.Height; j++ {
			if white {
				data = append(data, 255)
			} else {
				data = append(data, 0)
			}
			white = !white
		}
	}
	b.SetPixels(data)

	<-make(chan bool) // Prevent program from exiting
}
