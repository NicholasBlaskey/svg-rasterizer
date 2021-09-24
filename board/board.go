package board

import (
	"github.com/nicholasblaskey/webgl/webgl"
	"syscall/js"

	mgl "github.com/go-gl/mathgl/mgl32"
	"github.com/nicholasblaskey/webgl-utils/util"
)

type Board struct {
	Width  int
	Height int
	gl     *webgl.Gl
	canvas js.Value
	//
	ZoomFactor       float32
	zoomValue        float32
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
	mouseX                float32
	mouseY                float32
	numSquares            int       // numSquares x numSquares squares in pixel inspector
	offsets               []float32 // Amount to add to the texture coordinate to get to the pixel from the center.
	offsetsBuff           *util.Buffer
	//
	texture *webgl.Texture
	program *webgl.Program
}

func New(canvas js.Value) (*Board, error) {
	gl, err := webgl.FromCanvas(canvas)
	if err != nil {
		return nil, err
	}

	// TODO ensure (width * height) % 4 == 0
	b := &Board{gl: gl, canvas: canvas, ZoomFactor: 0.05, TranslationSpeed: 0.003,
		numSquares: 15,
		//Width:      12, Height: 12,
		Width: canvas.Get("width").Int(), Height: canvas.Get("height").Int(),
	}

	err = b.initShaders()
	if err != nil {
		return nil, err
	}
	b.initTexCoords()
	b.initPositions()
	b.initTexture()

	b.initZoomListener()
	b.initTranslationListener()
	b.initPixelInspector()
	b.initPixelInspectorOffsets()

	b.gl.ClearColor(0.3, 0.5, 0.3, 1.0)

	b.Draw()

	return b, nil
}

func (b *Board) SetWidthHeight(w, h int) {
	b.Width, b.Height = w, h
	b.initTexCoords()
	b.initPixelInspectorOffsets()
}

func (b *Board) EnablePixelInspector(shouldTurnOn bool) {
	b.pixelInspectorOn = shouldTurnOn
	b.Draw()
}

func (b *Board) initPixelInspector() {
	// Always have the pixel inspector on and listening
	b.canvas.Call("addEventListener", "mousemove",
		js.FuncOf(func(this js.Value, args []js.Value) interface{} {
			texelSizeX := 1.0 / float32(b.Width)
			texelSizeY := 1.0 / float32(b.Height)

			x, y := getXAndYFromEvent(args[0])
			x = (x / float32(b.canvas.Get("width").Float())) * float32(b.Width)
			y = (y / float32(b.canvas.Get("height").Float())) * float32(b.Height)

			//b.mouseX, b.mouseY = x, y

			// Add in a small value to ensure we are near the center of a pixel
			// not on the edge of a pixel.
			b.mouseX = (x*texelSizeX + texelSizeX/2 - 0.5) * 2.0
			b.mouseY = (1.0 - y*texelSizeY + texelSizeY/2 - 0.5) * 2.0

			if b.pixelInspectorOn {
				b.Draw()
			}

			return nil
		}))
}

func (b *Board) initPixelInspectorOffsets() {
	if b.offsetsBuff != nil {
		b.gl.DeleteBuffer(b.offsetsBuff.WebGlBuffer)
	}

	b.offsets = []float32{}
	texelSizeX, texelSizeY := 1.0/float32(b.Width), 1.0/float32(b.Height)
	centerX, centerY := b.numSquares/2, b.numSquares/2

	for y := 0; y < b.numSquares; y++ {
		for x := 0; x < b.numSquares; x++ {
			xOffset := float32(x-centerX) * texelSizeX
			yOffset := float32(y-centerY) * texelSizeY

			// Repeat 4 times since each quad has 6 vertices.
			b.offsets = append(b.offsets,
				xOffset, yOffset,
				xOffset, yOffset,
				xOffset, yOffset,
				xOffset, yOffset,
				xOffset, yOffset,
				xOffset, yOffset,
			)
		}
	}

	b.gl.UseProgram(b.pixelInspectorProgram)
	b.offsetsBuff = util.NewBufferVec2(b.gl)
	b.offsetsBuff.BindData(b.gl, b.offsets)
}

func getXAndYFromEvent(e js.Value) (float32, float32) {
	x := float32(e.Get("offsetX").Float())
	y := float32(e.Get("offsetY").Float())

	return x, y
}

func (b *Board) initTranslationListener() {
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

func (b *Board) setTranslation() {
	b.gl.UseProgram(b.program)
	util.SetVec2(b.gl, b.program, "translation", b.translation)

	b.gl.UseProgram(b.pixelInspectorProgram)
	util.SetVec2(b.gl, b.pixelInspectorProgram, "translation", b.translation)
}

func (b *Board) applyTranslation(xStart, yStart, x, y float32) {
	b.translation = b.translation.Add(mgl.Vec2{x - xStart, yStart - y}.Mul(b.TranslationSpeed))
	b.setTranslation()

	b.Draw()
}

func (b *Board) ResetView() {
	b.translation = mgl.Vec2{}
	b.setTranslation()

	b.zoomValue = 1.0
	b.setZoom()
}

func (b *Board) initZoomListener() {
	b.zoomValue = float32(1.0)
	b.setZoom()

	eventFunc := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		val := args[0].Get("deltaY").Float()
		if val > 0 {
			b.zoomValue -= b.ZoomFactor
			if b.zoomValue < 0 {
				b.zoomValue = 0
			}
		} else {
			b.zoomValue += b.ZoomFactor
		}
		b.setZoom()
		b.Draw()
		return nil
	})

	js.Global().Call("addEventListener", "wheel", eventFunc)
}

func (b *Board) setZoom() {
	b.gl.UseProgram(b.program)
	util.SetFloat(b.gl, b.program, "zoomFactor", b.zoomValue)

	b.gl.UseProgram(b.pixelInspectorProgram)
	util.SetFloat(b.gl, b.pixelInspectorProgram, "zoomFactor", b.zoomValue)
}

func (b *Board) initShaders() error {
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
			//uniform vec4 foreground;
			//uniform vec4 background;
			void main() {
				gl_FragColor = texture2D(t, texCoord);
				//float alpha = texture2D(t, texCoord).a;
				//gl_FragColor = alpha * foreground + (1.0 - alpha) * background;
			}`

	program, err := util.CreateProgram(b.gl, vertShader, fragShader)
	if err != nil {
		return err
	}
	b.program = program

	vertShader = `
			attribute vec2 a_position;
			attribute vec2 a_offset;
			varying vec2 offset;
			void main() {
				vec2 pos = a_position;
				gl_Position = vec4(pos, 0.0, 1.0);
				offset = a_offset;
			}`

	fragShader = `
			precision mediump float;
			uniform sampler2D t;
			uniform vec2 mousePos;
			//uniform vec4 foreground;
			//uniform vec4 background;
			uniform vec2 translation;
			uniform float zoomFactor;
			varying vec2 offset;
			void main() {
				vec2 texCoord = (mousePos - translation) / zoomFactor; 

				// Convert to texture coodinates of [0, 1] from [-1, 1]
				texCoord = (texCoord / 2.0) + 0.5 + offset;
				if (texCoord.x >= 0.0 && texCoord.x <= 1.0 &&
					texCoord.y >= 0.0 && texCoord.y <= 1.0) {

					gl_FragColor = texture2D(t, texCoord);
					//float alpha = texture2D(t, texCoord).a;
					//gl_FragColor = alpha * foreground + (1.0 - alpha) * background;
				} else {
					gl_FragColor = vec4(1.0, 1.0, 1.0, 1.0);
				}
			}`
	pixelProgram, err := util.CreateProgram(b.gl, vertShader, fragShader)
	if err != nil {
		return err
	}
	b.pixelInspectorProgram = pixelProgram

	return nil
}

func (b *Board) initTexture() {
	if b.texture != nil {
		b.gl.DeleteTexture(b.texture)
	}

	b.texture = b.gl.CreateTexture()
	b.gl.ActiveTexture(webgl.TEXTURE0)
	b.gl.BindTexture(webgl.TEXTURE_2D, b.texture)

	data := make([]byte, b.Width*b.Height*4)
	b.setTextureData(data)

	b.gl.TexParameteri(webgl.TEXTURE_2D, webgl.TEXTURE_WRAP_S, webgl.CLAMP_TO_EDGE)
	b.gl.TexParameteri(webgl.TEXTURE_2D, webgl.TEXTURE_WRAP_T, webgl.CLAMP_TO_EDGE)
	b.gl.TexParameteri(webgl.TEXTURE_2D, webgl.TEXTURE_MIN_FILTER, webgl.NEAREST)
	b.gl.TexParameteri(webgl.TEXTURE_2D, webgl.TEXTURE_MAG_FILTER, webgl.NEAREST)

	b.gl.UseProgram(b.program)
	util.SetInt(b.gl, b.program, "t", 0)
	b.gl.UseProgram(b.pixelInspectorProgram)
	util.SetInt(b.gl, b.pixelInspectorProgram, "t", 0)
}

func (b *Board) setTextureData(data []byte) {
	b.gl.TexImage2DArray(webgl.TEXTURE_2D, 0, webgl.RGBA, b.Width, b.Height, 0,
		webgl.RGBA, webgl.UNSIGNED_BYTE, data)
}

func (b *Board) SetPixels(data []byte) {
	b.setTextureData(data)
	b.Draw()
}

func (b *Board) initTexCoords() {
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

func (b *Board) initPositions() {
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

	b.pixelPos = []float32{}
	translations := []mgl.Vec2{}
	xOffset := 1.0 / float32(b.numSquares)
	yOffset := 1.0 / float32(b.numSquares)

	borderAmount := float32(0.005)
	quad := []float32{
		-xOffset + borderAmount, +yOffset - borderAmount,
		-xOffset + borderAmount, -yOffset + borderAmount,
		+xOffset - borderAmount, -yOffset + borderAmount,
		-xOffset + borderAmount, +yOffset - borderAmount,
		+xOffset - borderAmount, -yOffset + borderAmount,
		+xOffset - borderAmount, +yOffset - borderAmount,
	}

	for y := -b.numSquares; y < b.numSquares; y += 2 {
		for x := -b.numSquares; x < b.numSquares; x += 2 {
			translations = append(translations,
				mgl.Vec2{float32(x)/float32(b.numSquares) + xOffset,
					float32(y)/float32(b.numSquares) + yOffset})
		}
	}
	for _, trans := range translations {
		for i := 0; i < len(quad); i += 2 {
			b.pixelPos = append(b.pixelPos, quad[i]+trans[0], quad[i+1]+trans[1])
		}
	}

	b.gl.UseProgram(b.pixelInspectorProgram)
	b.pixelPosBuff = util.NewBufferVec2(b.gl)
	b.pixelPosBuff.BindData(b.gl, b.pixelPos)
}

func (b *Board) Draw() {
	// Draw the texture.
	w, h := b.canvas.Get("width").Int(), b.canvas.Get("height").Int()
	b.gl.Viewport(0.0, 0.0, w, h)

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
	inspectorWidth, inspectorHeight := w/3, h/3
	if w < h { // Correct for aspect ratio to ensure out pixel inspector is a square
		inspectorWidth = inspectorHeight
	} else {
		inspectorHeight = inspectorWidth
	}

	b.gl.Viewport(w-inspectorWidth, h-inspectorHeight, inspectorWidth, inspectorHeight)

	// Draw black box around viewport to be used for pixel borders.
	b.gl.Enable(webgl.SCISSOR_TEST)
	b.gl.Scissor(w-inspectorWidth, h-inspectorWidth, inspectorWidth, inspectorWidth)
	b.gl.ClearColor(0.0, 0, 0.0, 1.0)
	b.gl.Clear(webgl.COLOR_BUFFER_BIT)
	b.gl.Disable(webgl.SCISSOR_TEST)

	b.gl.UseProgram(b.pixelInspectorProgram)

	mousePos := mgl.Vec2{b.mouseX, b.mouseY}
	util.SetVec2(b.gl, b.pixelInspectorProgram, "mousePos", mousePos)

	b.offsetsBuff.BindToAttrib(b.gl, b.pixelInspectorProgram, "a_offset")
	b.pixelPosBuff.BindToAttrib(b.gl, b.pixelInspectorProgram, "a_position")
	b.gl.DrawArrays(webgl.TRIANGLES, 0, b.pixelPosBuff.VertexCount)
}
