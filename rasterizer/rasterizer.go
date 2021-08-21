package main

import (
	"bytes"
	"io/ioutil"
	"math"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"syscall/js"

	"fmt"

	"encoding/xml"

	"github.com/nicholasblaskey/svg-rasterizer/board"
)

func parseColor(col string) (byte, byte, byte, byte) {
	r, _ := strconv.ParseInt(col[1:3], 16, 8)
	g, _ := strconv.ParseInt(col[3:5], 16, 8)
	b, _ := strconv.ParseInt(col[5:7], 16, 8)
	a := 255

	return byte(r), byte(g), byte(b), byte(a)
}

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
	Lines   []Line `xml:"line"`
}

type Rect struct {
	X      float32 `xml:"x,attr"`
	Y      float32 `xml:"y,attr"`
	Fill   string  `xml:"fill,attr"`
	Width  float32 `xml:"width,attr"`
	Height float32 `xml:"height,attr"`
}

func (s *Rect) rasterize(r *rasterizer) {
	red, g, b, a := parseColor(s.Fill)
	r.drawPoint(s.X, s.Y, red, g, b, a)
}

func (r *rasterizer) drawPoint(x, y float32, red, g, b, a byte) {
	// TODO is this width and height divide right?
	xCoord := int(x * float32(r.widthPixels) / r.width)
	yCoord := r.heightPixels - int(y*float32(r.heightPixels)/r.height)

	if xCoord < 0 || xCoord >= r.widthPixels ||
		yCoord < 0 || yCoord >= r.heightPixels {
		return
	}

	r.pixels[(xCoord+yCoord*r.widthPixels)*4] = red
	r.pixels[(xCoord+yCoord*r.widthPixels)*4+1] = g
	r.pixels[(xCoord+yCoord*r.widthPixels)*4+2] = b
	r.pixels[(xCoord+yCoord*r.widthPixels)*4+3] = a
}

type Line struct {
	X1   float32 `xml:"x1,attr"`
	Y1   float32 `xml:"y1,attr"`
	X2   float32 `xml:"x2,attr"`
	Y2   float32 `xml:"y2,attr"`
	Fill string  `xml:"stroke,attr"`
}

func (s *Line) rasterize(r *rasterizer) {
	red, g, b, a := parseColor(s.Fill)

	slope := (s.Y2 - s.Y1) / (s.X2 - s.X1)

	// Slope greater than one case
	if math.Abs(float64(slope)) > 1.0 {
		if s.Y1 < s.Y2 {
			r.bresenham(s.Y1, s.X1, s.Y2, 1.0/slope, red, g, b, a, true)
		} else { // Flip and y1 and y2
			r.bresenham(s.Y2, s.X2, s.Y1, 1.0/slope, red, g, b, a, true)
		}
		return
	}

	// Slope less than one case
	if s.X1 < s.X2 {
		r.bresenham(s.X1, s.Y1, s.X2, slope, red, g, b, a, false)
	} else { // Flip and x1 and x2
		r.bresenham(s.X2, s.Y2, s.X1, slope, red, g, b, a, false)
	}
}

func (r *rasterizer) bresenham(x1, y1, x2, slope float32, red, g, b, a byte, flipped bool) {
	direction := float32(1.0)
	if slope < 0 {
		direction = -1.0
	}

	epsilon := float32(0.0)
	y := y1
	for x := x1; x < x2; x++ {
		if flipped {
			r.drawPoint(y, x, red, g, b, a)
		} else {
			r.drawPoint(x, y, red, g, b, a)
		}

		if (slope >= 0 && epsilon+slope < 0.5) || (slope < 0 && epsilon+slope > -0.5) {
			epsilon += slope
		} else {
			y += direction
			epsilon += slope - direction
		}
	}
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
	// This is probaly very wrong. However keep going and revise
	// this to keep handling more test cases.
	viewBox := strings.Split(svg.ViewBox, " ")
	widthPixels, _ := strconv.ParseFloat(viewBox[2], 64)
	heightPixels, _ := strconv.ParseFloat(viewBox[3], 64)

	width, _ := strconv.ParseFloat(strings.Split(svg.Width, "px")[0], 64)
	height, _ := strconv.ParseFloat(strings.Split(svg.Height, "px")[0], 64)

	r.widthPixels = int(widthPixels)
	r.heightPixels = int(heightPixels)
	r.width = float32(width)
	r.height = float32(height)

	fmt.Println("FMT", widthPixels, heightPixels, width, height)

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
	for _, line := range r.svg.Lines {
		line.rasterize(r)
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

	//r, err := New(canvas, "/svg/test1.svg")
	r, err := New(canvas, "/svg/test2.svg")
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
