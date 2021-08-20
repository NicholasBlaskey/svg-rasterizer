package main

import (
	"bytes"
	"io/ioutil"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"syscall/js"

	"fmt"

	"encoding/xml"

	"github.com/nicholasblaskey/svg-rasterizer/board"
)

type rasterizer struct {
	board        *board.Board
	svg          *Svg
	pixels       []byte
	widthPixels  int
	heightPixels int
	width        float32
	height       float32
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

func parseColor(col string) (byte, byte, byte, byte) {

	//panic(col[1:2])

	r, _ := strconv.ParseInt(col[1:3], 16, 8)
	g, _ := strconv.ParseInt(col[3:5], 16, 8)
	b, _ := strconv.ParseInt(col[5:7], 16, 8)
	a := 255

	fmt.Println(col[1:3], col[3:5], col[5:7])

	return byte(r), byte(g), byte(b), byte(a)
}

func (s *Rect) rasterize(r *rasterizer) {
	xCoord := int(s.X * float32(r.widthPixels))
	yCoord := r.heightPixels - int(s.Y*float32(r.heightPixels))

	if xCoord < 0 || xCoord > r.widthPixels ||
		yCoord < 0 || yCoord > r.heightPixels {
		return
	}

	red, g, b, a := parseColor(s.Fill)
	r.pixels[(xCoord+yCoord*r.widthPixels)*4] = red
	r.pixels[(xCoord+yCoord*r.widthPixels)*4+1] = g
	r.pixels[(xCoord+yCoord*r.widthPixels)*4+2] = b
	r.pixels[(xCoord+yCoord*r.widthPixels)*4+3] = a
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
	r.widthPixels = 600 // TODO Ensure multiple for byte alignment
	r.heightPixels = 600
	r.width = 1.0
	r.height = 1.0

	// Create board.
	canvas.Set("height", r.widthPixels)
	canvas.Set("width", r.heightPixels)
	b, err := board.New(canvas)
	if err != nil {
		panic(err)
	}
	//b.EnablePixelInspector(true)

	r.board = b

	return r, nil
}

func (r *rasterizer) Draw() {
	r.pixels = make([]byte, 4*r.widthPixels*r.heightPixels)

	for i := 0; i < len(r.pixels); i++ {
		r.pixels[i] = 255
	}

	for _, rect := range r.svg.Rects {
		rect.rasterize(r)
	}

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

	/*
		canvas.Set("height", 900)
		canvas.Set("width", 900)
	*/

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
