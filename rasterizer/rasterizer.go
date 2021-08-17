package main

import (
	"math/rand"
	"syscall/js"

	"fmt"

	"github.com/nicholasblaskey/svg-rasterizer/board"
)

func main() {
	document := js.Global().Get("document")
	canvas := document.Call("getElementById", "webgl")
	canvas.Set("height", 800)
	canvas.Set("width", 800)
	b, err := board.New(canvas)
	if err != nil {
		panic(err)
	}
	b.EnablePixelInspector(true)

	fmt.Println("starting", rand.Int31n(256))

	/*
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
	*/

	data := []byte{}
	for i := 0; i < b.Width/10; i++ {
		col1, col2 := byte(rand.Int31n(256)), byte(rand.Int31n(256))

		for k := 0; k < 10; k++ {

			for j := 0; j < b.Height/2; j++ {
				data = append(data, col1)
			}
			for j := b.Height / 2; j < b.Height; j++ {
				data = append(data, col2)
			}
		}
	}
	b.SetPixels(data)

	<-make(chan bool) // Prevent program from exiting
}
