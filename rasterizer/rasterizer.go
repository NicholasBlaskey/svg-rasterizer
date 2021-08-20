package main

import (
	"bytes"
	"io/ioutil"
	"math/rand"
	"net/http"
	"strings"
	"syscall/js"

	"fmt"

	"encoding/xml"

	"github.com/nicholasblaskey/svg-rasterizer/board"
)

type rasterizer struct {
	board       *board.Board
	svg         *Svg
	drawingInfo drawInfo
	pixels      []byte
}

// TODO improve this
type drawInfo struct {
	width  int
	height int
}

type Svg struct {
	XMLName xml.Name
	Width   string `xml:"width,attr"`
	Height  string `xml:"height,attr"`
	ViewBox string `xml:"viewBox,attr"`
	Rects   []Rect `xml:"rect"`
}

type Rect struct {
	X      float32 `xml:"x,attr"`
	Y      float32 `xml:"y,attr"`
	Fill   string  `xml:"fill,attr"`
	Width  float32 `xml:"width,attr"`
	Height float32 `xml:"height,attr"`
}

func (s *Rect) rasterize(r *rasterizer) {
	r.pixels[0] = 255
}

/*
type Node struct {
	XMLName xml.Name
	Content []byte `xml:",innerxml"`
	Nodes   []Node `xml:",any"`
}
*/

func New(canvas js.Value, filePath string) (*rasterizer, error) {
	r := &rasterizer{}

	// Get xml file and parse it.
	fileString := getFile(filePath)

	buf := bytes.NewBuffer([]byte(fileString))
	dec := xml.NewDecoder(buf)
	var svg Svg
	if err := dec.Decode(&svg); err != nil {
		return nil, err
	}

	r.svg = &svg

	// TODO Calculate needed drawing info.
	r.drawingInfo = drawInfo{
		width:  600, // TODO Ensure multiple for byte alignment
		height: 600,
	}

	// Create board.
	canvas.Set("height", r.drawingInfo.width)
	canvas.Set("width", r.drawingInfo.height)
	b, err := board.New(canvas)
	if err != nil {
		panic(err)
	}
	b.EnablePixelInspector(true)

	r.board = b

	return r, nil
}

func (r *rasterizer) Draw() {
	r.pixels = make([]byte, 1*r.drawingInfo.width*r.drawingInfo.height)

	r.svg.Rects[0].rasterize(r)

	r.board.SetPixels(r.pixels)
}

func getFile(filePath string) string {
	loc := js.Global().Get("location")
	url := loc.Get("protocol").String() + "//" +
		loc.Get("hostname").String() + ":" +
		loc.Get("port").String()

	resp, err := http.Get(url + filePath)
	if err != nil {
		panic(err)
	}

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	s := string(b)
	return strings.ReplaceAll(s, "\r", "")
}

func main() {
	document := js.Global().Get("document")
	canvas := document.Call("getElementById", "webgl")

	r, err := New(canvas, "/svg/test1.svg")
	if err != nil {
		panic(err)
	}

	r.Draw()

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

	/*
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
	*/

	<-make(chan bool) // Prevent program from exiting
}
