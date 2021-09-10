package main

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"math"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"syscall/js"

	"encoding/base64"
	"image"
	"image/color"
	"image/png"

	mgl "github.com/go-gl/mathgl/mgl32"

	"github.com/nicholasblaskey/svg-rasterizer/board"
	"github.com/nicholasblaskey/svg-rasterizer/triangulate"
)

type Color struct {
	r float32
	g float32
	b float32
	a float32
}

func maxOfThree(x, y, z float32) float32 {
	return float32(math.Max(float64(x), math.Max(float64(y), float64(z))))
}

func minOfThree(x, y, z float32) float32 {
	return float32(math.Min(float64(x), math.Min(float64(y), float64(z))))
}

func crossProduct(x1, y1, x2, y2 float32) float32 {
	return x1*y2 - y1*x2
}

func parseColor(col string) Color {
	if len(col) == 0 {
		return Color{0, 0, 0, 0}
	}

	if len(col) == 6 { // Add in # if missing
		col = "#" + col
	}

	r, _ := strconv.ParseInt(col[1:3], 16, 9)
	g, _ := strconv.ParseInt(col[3:5], 16, 9)
	b, _ := strconv.ParseInt(col[5:7], 16, 9)
	a := 255

	return Color{
		float32(r) / 255.0,
		float32(g) / 255.0,
		float32(b) / 255.0,
		float32(a) / 255.0,
	}
}

type rasterizer struct {
	board               *board.Board
	svg                 *Svg
	pixels              []byte
	widthPixels         int
	heightPixels        int
	width               float32
	height              float32
	sampleRate          int
	samplePixels        int
	origWidthPixels     int
	origHeightPixels    int
	origWidth           float32
	origHeight          float32
	pointsToFill        []int
	colorOfPointsToFill []Color
}

type Svg struct {
	XMLName         xml.Name
	Width           string    `xml:"width,attr"`
	Height          string    `xml:"height,attr"`
	ViewBox         string    `xml:"viewBox,attr"`
	Rects           []Rect    `xml:"rect"`
	Lines           []Line    `xml:"line"`
	Polygons        []Polygon `xml:"polygon"`
	Groups          []Svg     `xml:"g"`
	Images          []Image   `xml:"image"`
	Transform       string    `xml:"transform,attr"`
	transformMatrix mgl.Mat3
}

type Rect struct {
	X      float32 `xml:"x,attr"`
	Y      float32 `xml:"y,attr"`
	Fill   string  `xml:"fill,attr"`
	Width  float32 `xml:"width,attr"`
	Height float32 `xml:"height,attr"`
}

func (s *Rect) rasterize(r *rasterizer) {
	col := parseColor(s.Fill)
	r.drawPixel(s.X, s.Y, col)
}

// This draws a point which will then be anti aliased.
func (r *rasterizer) drawPoint(x, y float32, col Color) {
	// TODO is this width and height divide right?
	xCoord := int(x * float32(r.widthPixels) / r.width)
	yCoord := r.heightPixels - int(y*float32(r.heightPixels)/r.height)

	if xCoord < 0 || xCoord >= r.widthPixels ||
		yCoord < 0 || yCoord >= r.heightPixels {
		return
	}

	r.pixels[(xCoord+yCoord*r.widthPixels)*4] = byte(col.r * 255.0)
	r.pixels[(xCoord+yCoord*r.widthPixels)*4+1] = byte(col.g * 255.0)
	r.pixels[(xCoord+yCoord*r.widthPixels)*4+2] = byte(col.b * 255.0)
	r.pixels[(xCoord+yCoord*r.widthPixels)*4+3] = byte(col.a * 255.0)
}

// This draws a pixel which will be drawn into the final buffer after everything else
// has been resolved.
func (r *rasterizer) drawPixel(x, y float32, col Color) {
	xCoord := int(x * float32(r.origWidthPixels) / r.origWidth)
	yCoord := r.origHeightPixels - int(y*float32(r.origHeightPixels)/r.origHeight)

	if xCoord < 0 || xCoord >= r.origWidthPixels ||
		yCoord < 0 || yCoord >= r.origHeightPixels {
		return
	}

	r.pointsToFill = append(r.pointsToFill, (xCoord+yCoord*r.origWidthPixels)*4)
	r.colorOfPointsToFill = append(r.colorOfPointsToFill, col)
}

type Line struct {
	X1   float32 `xml:"x1,attr"`
	Y1   float32 `xml:"y1,attr"`
	X2   float32 `xml:"x2,attr"`
	Y2   float32 `xml:"y2,attr"`
	Fill string  `xml:"stroke,attr"`
}

func round(x float32) float32 {
	return float32(int(x + 0.5))
}

func fpart(x float32) float32 {
	return x - float32(int(x))
}

func rfpart(x float32) float32 {
	return 1.0 - fpart(x)
}

// Uses a single strain of Xiaolin since it seems to give the best results.
// The two strains makes the colors look odd however revisit this after antialiasing.
// Not sure if the resolution is just too low.
func (r *rasterizer) drawLine(x0, y0, x1, y1 float32, col Color) {
	steep := math.Abs(float64(y1-y0)) > math.Abs(float64(x1-x0))
	if steep {
		x0, y0 = y0, x0
		x1, y1 = y1, x1
	}

	if x0 > x1 {
		x0, x1 = x1, x0
		y0, y1 = y1, y0
	}

	dx := x1 - x0
	dy := y1 - y0
	gradient := dy / dx
	if dx == 0.0 {
		gradient = 1.0
	}

	// Handle first endpoint
	xend := round(x0)
	yend := y0 + gradient*(xend-x0)
	xpxl1 := xend // This will be used in the main loop
	ypxl1 := float32(int(yend))
	if steep {
		r.drawPixel(ypxl1, xpxl1, col)
	} else {
		r.drawPixel(xpxl1, ypxl1, col)
	}
	intery := yend + gradient // first y-intersection for the main loop

	// Handle second endpoint
	xend = round(x1)
	yend = y1 + gradient*(xend-x1)
	xpxl2 := xend // This will be used in the main loop
	ypxl2 := float32(int(yend))
	if steep {
		r.drawPixel(ypxl2, xpxl2, col)
	} else {
		r.drawPixel(xpxl2, ypxl2, col)
	}

	// Main loop
	if steep {
		for x := xpxl1 + 1; x <= xpxl2-1; x++ {
			r.drawPixel(intery, x, col)
			intery += gradient
		}
	} else {
		for x := xpxl1 + 1; x <= xpxl2-1; x++ {
			r.drawPixel(x, intery, col)
			intery += gradient
		}
	}
}

func (s *Line) rasterize(r *rasterizer) {
	col := parseColor(s.Fill)
	r.drawLine(s.X1, s.Y1, s.X2, s.Y2, col)
}

type Polygon struct {
	Fill            string `xml:"fill,attr"`
	Stroke          string `xml:"stroke,attr"`
	Points          string `xml:"points,attr"`
	Transform       string `xml:"transform,attr"`
	transformMatrix mgl.Mat3
}

func parseTransform(trans string) mgl.Mat3 {
	if strings.Contains(trans, "matrix") { // Matrix transformation case
		trans = strings.TrimPrefix(trans, "matrix(")
		trans = strings.Trim(trans, " )\n\t\r")

		points := []float32{}
		for _, s := range strings.Split(trans, ",") {
			x, err := strconv.ParseFloat(s, 32)
			if err != nil {
				panic(err)
			}
			points = append(points, float32(x))
		}

		mat := mgl.Ident3()
		mat[0], mat[1] = points[0], points[1]
		mat[3], mat[4] = points[2], points[3]
		mat[6], mat[7] = points[4], points[5]

		return mat
	}
	return mgl.Ident3()
}

func (r *rasterizer) transform(points []float32, trans mgl.Mat3) []float32 {
	for i := 0; i < len(points); i += 2 {
		xyz := mgl.Vec3{points[i], points[i+1], 1.0}
		transformed := trans.Mul3x1(xyz)
		points[i] = transformed[0] * float32(r.sampleRate)
		points[i+1] = transformed[1] * float32(r.sampleRate)
	}

	return points
}

func (r *rasterizer) pointsToTriangles(in string,
	transformation mgl.Mat3) ([]*triangulate.Triangle, []float32) {

	points := strings.Split(strings.Trim(in, " "), " ")

	pointsFloat := []float32{}
	for _, p := range points {
		xy := strings.Split(strings.Trim(p, "\n\r\t "), ",")
		x, err1 := strconv.ParseFloat(xy[0], 32)
		y, err2 := strconv.ParseFloat(xy[1], 32)
		if err1 != nil || err2 != nil { // TODO figure out error handling
			if err1 != nil {
				panic(err1)
			}
			panic(err2)
		}
		pointsFloat = append(pointsFloat, float32(x), float32(y))
	}

	pointsFloat = r.transform(pointsFloat, transformation)

	triangles := triangulate.Triangulate(pointsFloat)
	for _, t := range triangles {
		// Sort triangle such that y1 < y2 < y3
		if t.Y1 > t.Y3 {
			t.X1, t.Y1, t.X3, t.Y3 = t.X3, t.Y3, t.X1, t.Y1
		}
		if t.Y1 > t.Y2 {
			t.X1, t.Y1, t.X2, t.Y2 = t.X2, t.Y2, t.X1, t.Y1
		}
		if t.Y2 > t.Y3 {
			t.X2, t.Y2, t.X3, t.Y3 = t.X3, t.Y3, t.X2, t.Y2
		}

	}
	return triangles, pointsFloat
}

func (s *Polygon) rasterize(r *rasterizer) {
	s.boundingBoxApproach(r)
}

func (s *Polygon) boundingBoxApproach(r *rasterizer) {
	triangles, points := r.pointsToTriangles(s.Points, s.transformMatrix)

	// Draw each triangle
	col := parseColor(s.Fill)
	for _, t := range triangles {
		minX := minOfThree(t.X1, t.X2, t.X3)
		maxX := maxOfThree(t.X1, t.X2, t.X3)
		minY := minOfThree(t.Y1, t.Y2, t.Y3)
		maxY := maxOfThree(t.Y1, t.Y2, t.Y3)

		vsX1, vsY1 := t.X2-t.X1, t.Y2-t.Y1
		vsX2, vsY2 := t.X3-t.X1, t.Y3-t.Y1

		for x := float32(int(minX)); x <= maxX; x++ {
			for y := float32(int(minY)); y <= maxY; y++ {
				//for x := float32(minX); x <= maxX; x++ {
				//	for y := float32(minY); y <= maxY; y++ {

				qx, qy := x-t.X1, y-t.Y1

				s := crossProduct(qx, qy, vsX2, vsY2) / crossProduct(vsX1, vsY1, vsX2, vsY2)
				t := crossProduct(vsX1, vsY1, qx, qy) / crossProduct(vsX1, vsY1, vsX2, vsY2)

				if s >= 0 && t >= 0 && s+t <= 1 {
					r.drawPoint(x, y, col)
					//r.drawPoint(0, 0, red, g, b, a)
				}
			}
		}

		/*
			r.drawLine(t.X1, t.Y1, t.X2, t.Y2, 0, 0, 0, 255)
			r.drawLine(t.X2, t.Y2, t.X3, t.Y3, 0, 0, 0, 255)
			r.drawLine(t.X3, t.Y3, t.X1, t.Y1, 0, 0, 0, 255)
		*/
	}

	// Draw the outline if it exists.
	if s.Stroke == "" {
		return
	}
	outlineCol := parseColor(s.Stroke)
	for i := 0; i < len(points); i += 2 {
		p1X, p1Y := points[i], points[i+1]
		p2X, p2Y := points[(i+2)%len(points)], points[(i+3)%len(points)]
		r.drawLine(p1X/float32(r.sampleRate), p1Y/float32(r.sampleRate),
			p2X/float32(r.sampleRate), p2Y/float32(r.sampleRate), outlineCol)
	}

}

type Image struct {
	X          int    `xml:"x,attr"`
	Y          int    `xml:"y,attr"`
	Width      int    `xml:"width,attr"`
	Height     int    `xml:"height,attr"`
	Href       string `xml:"href,attr"` // Assume all images of base64 png encoded
	imageSizeX int    // Width of image loaded
	imageSizeY int    // Height of image laoded
}

type mip struct {
	w    int
	h    int
	data []byte
}

func (m *mip) At(x, y int) (byte, byte, byte, byte) {
	i := x + y*m.w
	return m.data[i], m.data[i+1], m.data[i+2], m.data[i+3]
}

// Must be a power of two image
func generateMipMaps(img image.Image) []mip {
	bounds := img.Bounds()
	w := bounds.Max.X - bounds.Min.X
	h := bounds.Max.Y - bounds.Min.Y

	// Get original mip.
	mips := []mip{mip{w, h, make([]byte, w*h*4)}}
	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			r, g, b, a := img.At(x, y).RGBA()

			i := (x + y*w) * 4
			mips[0].data[i] = byte(float32(r) / 0xFFFF * 0xFF)
			mips[0].data[i+1] = byte(float32(g) / 0xFFFF * 0xFF)
			mips[0].data[i+2] = byte(float32(b) / 0xFFFF * 0xFF)
			mips[0].data[i+3] = byte(float32(a) / 0xFFFF * 0xFF)
		}
	}
	w /= 2
	h /= 2

	for w > 1 && h > 1 {
		buff := downSampleBuffer(mips[len(mips)-1].data, 2, w, h)

		w /= 2
		h /= 2

		mips = append(mips, mip{w, h, buff})
	}

	return mips
}

func (s *Image) rasterize(r *rasterizer) {
	// TODO Remove this
	//s.Width = 128
	//s.Height = s.Width

	// Load the image.
	baseImage := strings.Split(s.Href, ",")[1] // Only works for data:image/png;base64,...
	decoded, err := base64.StdEncoding.DecodeString(baseImage)
	if err != nil { // Remove this.
		panic(err)
	}
	reader := bytes.NewReader(decoded)

	img, err := png.Decode(reader)
	if err != nil {
		panic(err)
	}
	bounds := img.Bounds()
	s.imageSizeX = bounds.Max.X - bounds.Min.X
	s.imageSizeY = bounds.Max.Y - bounds.Min.Y

	mipMaps := generateMipMaps(img) // TODO make this at the start once

	// Loop through all the pixels
	// Then get the coordinate from texture space from screenspace?
	// then implmeenet sampleNearest and sampleBiliniear
	for x := s.X; x < s.X+s.Width; x++ {
		for y := s.Y; y < s.Y+s.Height; y++ {
			col := s.sampleNearest(mipMaps[0], float32(x), float32(y))
			//red, g, b, a := s.sampleBilinear(img, float32(x), float32(y))

			r.drawPixel(float32(x), float32(y), col)

		}
	}
}

func (s *Image) sampleNearest(img mip, x, y float32) Color {
	x -= float32(s.X) + 0.5
	y -= float32(s.Y) + 0.5

	x = x / float32(s.Width) * float32(s.imageSizeX)
	y = y / float32(s.Height) * float32(s.imageSizeY)

	red, g, b, a := img.At(int(x), int(y))

	return Color{float32(red) / 0xFF, float32(g) / 0xFF,
		float32(b) / 0xFF, float32(a) / 0xFF}
}

func blendColor(c0, c1 color.Color, amount float32) color.Color {
	r0, g0, b0, a0 := c0.RGBA()
	r1, g1, b1, a1 := c1.RGBA()

	return color.NRGBA64{blend(r0, r1, amount), blend(g0, g1, amount),
		blend(b0, b1, amount), blend(a0, a1, amount)}
}

func blend(x0, x1 uint32, amount float32) uint16 {
	return uint16((float32(x0)*amount + float32(x1)*(1-amount)))
}

func (s *Image) sampleBilinear(img image.Image, x, y float32) (float32, float32, float32, float32) {
	x = x - float32(s.X) + 0.5
	y = y - float32(s.Y) + 0.5
	x = x / float32(s.Width) * float32(s.imageSizeX)
	y = y / float32(s.Height) * float32(s.imageSizeY)

	tt := x - float32(int(x+0.5)) + 0.5
	st := y - float32(int(y+0.5)) + 0.5

	//fmt.Printf("(%f, %f), (%f, %f)\n", x, y, tt, st)
	//	fmt.Printf("(%f, %f) (%f, %f) (%d, %d)\n", x, y, y+1/2.0, y-1/2.0, int(y+1/2.0), int(y-1/2.0))
	//fmt.Printf("above (%d, %d) (%d, %d)\n", int(x-0.5), int(y-0.5), int(x+0.5), int(y+0.5))

	f00 := img.At(int(x-0.5), int(y+0.5))
	f01 := img.At(int(x-0.5), int(y-0.5))
	f10 := img.At(int(x+0.5), int(y+0.5))
	f11 := img.At(int(x+0.5), int(y-0.5))

	//fmt.Println(tt, st, f00, f01, f10, f11)

	c0 := blendColor(f00, f10, tt)
	c1 := blendColor(f01, f11, tt)
	c := blendColor(c0, c1, st)

	//return tt, st, 1.0, 1.0

	//fmt.Println(c, f00, f10, tt)

	/*
		w, h := float32(s.Width), float32(s.Height)

			u, v := x/w, y/h
			i, j := u+1/w, v+1/h
			st := u - (i + 1/w)
			tt := v - (j + 1/h)


			if st < 0 {
				st = 0
			}
			if tt < 0 {
				tt = 0
			}
	*/

	// TODO make sure coordinate is right and just draw the svg correctly.
	// Also make sure we do the actual blending now.
	//	fmt.Printf("(w, h) = (%f, %f) (x, y) = (%f, %f) (u, v) = (%f, %f) (i, j) = (%f, %f) (s, t) = (%f, %f)\n",
	//	w, h, x, y, u, v, i, j, st, tt)
	//c := img.At(int(x), int(y))

	red, g, b, a := c.RGBA()

	return float32(red) / 0xFFFF, float32(g) / 0xFFFF,
		float32(b) / 0xFFFF, float32(a) / 0xFFFF
}

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
	width, _ := strconv.ParseFloat(strings.Split(svg.Width, "px")[0], 64)
	height, _ := strconv.ParseFloat(strings.Split(svg.Height, "px")[0], 64)

	if svg.ViewBox != "" { // Does not have a viewbox
		viewBox := strings.Split(svg.ViewBox, " ")
		widthPixels, _ := strconv.ParseFloat(viewBox[2], 64)
		heightPixels, _ := strconv.ParseFloat(viewBox[3], 64)
		r.widthPixels = int(widthPixels)
		r.heightPixels = int(heightPixels)
	} else {
		r.widthPixels, r.heightPixels = int(width), int(height)
	}

	r.width = float32(width)
	r.height = float32(height)

	// Create board.
	canvas.Set("width", r.widthPixels)
	canvas.Set("height", r.heightPixels)
	b, err := board.New(canvas)
	if err != nil {
		panic(err)
	}

	pixelInspectorOn := false
	js.Global().Call("addEventListener", "keydown", js.FuncOf(
		func(this js.Value, args []js.Value) interface{} {
			if args[0].Get("keyCode").Int() == 90 { // z
				pixelInspectorOn = !pixelInspectorOn
				b.EnablePixelInspector(pixelInspectorOn)
			}
			return nil
		}))
	r.board = b

	r.sampleRate = 2

	return r, nil
}

func downSampleBuffer(from []byte, sampleRate int, w, h int) []byte {
	targetW := w / sampleRate
	targetH := h / sampleRate
	target := make([]byte, targetW*targetH*4)

	scaleFactor := byte(sampleRate * sampleRate)
	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			i := (x/sampleRate + y/sampleRate*targetW) * 4
			j := (x + y*w) * 4

			if w == 1 {
				fmt.Println(i, j, w, h, sampleRate, targetW, targetH, "IJ")
			}
			target[i] += from[j] / scaleFactor
			target[i+1] += from[j+1] / scaleFactor
			target[i+2] += from[j+2] / scaleFactor
			target[i+3] += from[j+3] / scaleFactor
		}
	}
	return target
}

func (r *rasterizer) Draw() {
	r.origWidthPixels, r.origHeightPixels = r.widthPixels, r.heightPixels
	r.origWidth, r.origHeight = r.width, r.height

	r.pointsToFill = []int{}

	r.widthPixels *= r.sampleRate
	r.heightPixels *= r.sampleRate
	r.width *= float32(r.sampleRate)
	r.height *= float32(r.sampleRate)
	r.pixels = make([]byte, 4*r.widthPixels*r.heightPixels)

	for i := 0; i < len(r.pixels); i++ {
		r.pixels[i] = 255
	}

	r.svg.transformMatrix = parseTransform(r.svg.Transform) // Can an SVG element have a transform??

	r.svg.rasterize(r)

	if r.sampleRate > 1 { // Anti aliasing
		r.pixels = downSampleBuffer(r.pixels, r.sampleRate, r.widthPixels, r.heightPixels)
	}

	// Fill points/lines that we aren't antialiaisng on.
	for i, point := range r.pointsToFill {
		r.pixels[point] = byte(r.colorOfPointsToFill[i].r * 255.0)
		r.pixels[point+1] = byte(r.colorOfPointsToFill[i].g * 255.0)
		r.pixels[point+2] = byte(r.colorOfPointsToFill[i].b * 255.0)
		r.pixels[point+3] = byte(r.colorOfPointsToFill[i].a * 255.0)
	}
	//fmt.Println(r.pixels)
	//fmt.Println(len(r.pixels))

	r.board.SetPixels(r.pixels)

	r.widthPixels, r.heightPixels = r.origWidthPixels, r.origHeightPixels
	r.width, r.height = r.origWidth, r.origHeight
}

func (s *Svg) rasterize(r *rasterizer) {
	//fmt.Println("svg transform", r.svg.transformMatrix)

	for _, rect := range s.Rects {
		rect.rasterize(r)
	}
	for _, line := range s.Lines {
		line.rasterize(r)
	}
	for _, polygon := range s.Polygons {
		polygon.transformMatrix = parseTransform(polygon.Transform)
		polygon.transformMatrix = s.transformMatrix.Mul3(polygon.transformMatrix)

		polygon.rasterize(r)
	}
	for _, group := range s.Groups {
		// Set transformation matrix for the group.
		group.transformMatrix = parseTransform(group.Transform)
		group.transformMatrix = s.transformMatrix.Mul3(group.transformMatrix)

		group.rasterize(r)
	}
	for _, image := range s.Images {
		image.rasterize(r)
	}
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
	//r, err := New(canvas, "/svg/test2.svg")
	//r, err := New(canvas, "/svg/test3.svg")
	//r, err := New(canvas, "/svg/test4.svg")
	//r, err := New(canvas, "/svg/test5.svg")
	//r, err := New(canvas, "/svg/test6.svg")
	r, err := New(canvas, "/svg/test7.svg")

	if err != nil {
		panic(err)
	}

	/*
		canvas.Set("height", 900)
		canvas.Set("width", 900)
	*/

	r.Draw()

	fmt.Println("starting", rand.Int31n(256))

	<-make(chan bool) // Prevent program from exiting
}
