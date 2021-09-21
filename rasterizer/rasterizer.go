package main

import (
	"bytes"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"image"
	"image/png"
	"io/ioutil"
	"math"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"syscall/js"

	mgl "github.com/go-gl/mathgl/mgl32"
	"github.com/nicholasblaskey/dat-gui-go-wasm/datGUI"

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
		return Color{0, 0, 0, 1.0}
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
	canvas              js.Value
}

type Svg struct {
	XMLName         xml.Name
	Width           string     `xml:"width,attr"`
	Height          string     `xml:"height,attr"`
	ViewBox         string     `xml:"viewBox,attr"`
	Rects           []Rect     `xml:"rect"`
	Lines           []Line     `xml:"line"`
	Polylines       []Polyline `xml:"polyline"`
	Polygons        []Polygon  `xml:"polygon"`
	Circles         []Circle   `xml:"circle"`
	Groups          []*Svg     `xml:"g"`
	Images          []*Image   `xml:"image"`
	Transform       string     `xml:"transform,attr"`
	transformMatrix mgl.Mat3
}

type Rect struct {
	X               float32 `xml:"x,attr"`
	Y               float32 `xml:"y,attr"`
	Fill            string  `xml:"fill,attr"`
	Stroke          string  `xml:"stroke,attr"`
	Width           float32 `xml:"width,attr"`
	Height          float32 `xml:"height,attr"`
	StrokeOpacity   float32 `xml:"stroke-opacityt,attr"`
	FillOpacity     float32 `xml:"fill-opacity,attr"`
	Transform       string  `xml:"transform,attr"`
	transformMatrix mgl.Mat3
}

func (s *Rect) rasterize(r *rasterizer) {
	col := parseColor(s.Fill)
	col.a = s.FillOpacity
	if col.a == 0.0 {
		col.a = 1.0
	}

	// If either width or height is 0 assume we have a single point.
	if s.Width == 0.0 || s.Height == 0.0 {
		r.drawPixel(s.X, s.Y, col)
		return
	}

	// Otherwise have a full on rectangle.
	transformed := r.transform([]float32{s.X, s.Y}, s.transformMatrix)
	x, y := transformed[0], transformed[1] //, transformed[2], transformed[3]
	w := s.Width * float32(r.sampleRate)
	h := s.Height * float32(r.sampleRate)

	// Draw inside of rectangle.
	for x0 := x; x0 < x+w; x0++ {
		for y0 := y; y0 < y+h; y0++ {
			r.drawPoint(x0, y0, col)
		}
	}

	// Draw rectangle border.
	outlineCol := parseColor(s.Stroke)
	col.a = s.StrokeOpacity
	if col.a == 0.0 {
		col.a = 1.0
	}
	for i := float32(0); i < w; i++ {
		r.drawPixel(x+i, y+1.0, outlineCol)
		r.drawPixel(x+i, y+float32(s.Height)-1, outlineCol)
	}
	for i := float32(0); i < h; i++ {
		r.drawPixel(x+0, y+i, outlineCol)
		r.drawPixel(x+float32(s.Width)-1, y+i, outlineCol)
	}
}

func blendColors(col Color, red, g, b, a byte) (byte, byte, byte, byte) {
	aPrimeA := float32(a) / 0xFF
	aPrimeR := float32(red) / 0xFF * aPrimeA
	aPrimeG := float32(g) / 0xFF * aPrimeA
	aPrimeB := float32(b) / 0xFF * aPrimeA

	bPrimeR := col.r * col.a
	bPrimeG := col.g * col.a
	bPrimeB := col.b * col.a
	bPrimeA := col.a

	cPrimeR := bPrimeR + (1-bPrimeA)*aPrimeR
	cPrimeG := bPrimeG + (1-bPrimeA)*aPrimeG
	cPrimeB := bPrimeB + (1-bPrimeA)*aPrimeB
	cPrimeA := bPrimeA + (1-bPrimeA)*aPrimeA

	return byte(cPrimeR * 0xFF), byte(cPrimeG * 0xFF),
		byte(cPrimeB * 0xFF), byte(cPrimeA * 0xFF)
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

	red := r.pixels[(xCoord+yCoord*r.widthPixels)*4]
	g := r.pixels[(xCoord+yCoord*r.widthPixels)*4+1]
	b := r.pixels[(xCoord+yCoord*r.widthPixels)*4+2]
	a := r.pixels[(xCoord+yCoord*r.widthPixels)*4+3]

	red, g, b, a = blendColors(col, red, g, b, a)

	r.pixels[(xCoord+yCoord*r.widthPixels)*4] = red
	r.pixels[(xCoord+yCoord*r.widthPixels)*4+1] = g
	r.pixels[(xCoord+yCoord*r.widthPixels)*4+2] = b
	r.pixels[(xCoord+yCoord*r.widthPixels)*4+3] = a
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

type Polyline struct {
	Stroke          string `xml:"stroke,attr"`
	Points          string `xml:"points,attr"`
	Transform       string `xml:"transform,attr"`
	transformMatrix mgl.Mat3
}

func (s *Polyline) rasterize(r *rasterizer) {
	col := parseColor(s.Stroke)

	pointsFloat := []float32{}
	points := strings.Split(strings.Trim(s.Points, " \n\r\t"), " ")
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

	pointsFloat = r.transform(pointsFloat, s.transformMatrix)

	for i := 0; i < len(pointsFloat)/2-1; i++ {
		r.drawLine(pointsFloat[i*2]/float32(r.sampleRate),
			pointsFloat[i*2+1]/float32(r.sampleRate),
			pointsFloat[(i+1)*2]/float32(r.sampleRate),
			pointsFloat[(i+1)*2+1]/float32(r.sampleRate), col)
	}
}

type Circle struct {
	Cx   float32 `xml:"cx,attr"`
	Cy   float32 `xml:"cy,attr"`
	R    float32 `xml:"r,attr"`
	Fill string  `xml:'fill,attr"`
}

func (s *Circle) rasterize(r *rasterizer) {
	cx := s.Cx * float32(r.sampleRate)
	cy := s.Cy * float32(r.sampleRate)
	radius := s.R * float32(r.sampleRate)

	col := parseColor(s.Fill)
	minX, maxX := cx-radius, cx+radius
	minY, maxY := cy-radius, cy+radius

	for x := float32(int(minX)); x <= maxX; x++ {
		for y := float32(int(minY)); y <= maxY; y++ {
			dx, dy := cx-x, cy-y
			if float32(math.Sqrt(float64(dx*dx+dy*dy))) <= radius {
				r.drawPoint(x, y, col)
			}
		}
	}
}

type Polygon struct {
	Fill            string `xml:"fill,attr"`
	Stroke          string `xml:"stroke,attr"`
	Points          string `xml:"points,attr"`
	Transform       string `xml:"transform,attr"`
	transformMatrix mgl.Mat3
	FillOpacity     float32 `xml:"fill-opacity,attr"`
	StrokeOpacity   float32 `xml:"stroke-opacity,attr"`
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
	} else if strings.Contains(trans, "translate") {
		trans = strings.TrimPrefix(trans, "translate(")
		trans = strings.Trim(trans, " )\n\t\r")
		split := strings.Split(trans, " ")

		x, err := strconv.ParseFloat(split[0], 32)
		if err != nil {
			panic(err)
		}
		y, err := strconv.ParseFloat(split[1], 32)
		if err != nil {
			panic(err)
		}

		return mgl.Translate2D(float32(x), float32(y))
	} else if strings.Contains(trans, "scale(") {
		trans = strings.TrimPrefix(trans, "scale(")
		trans = strings.Trim(trans, " )\n\t\r")
		split := strings.Split(trans, " ")

		x, err := strconv.ParseFloat(split[0], 32)
		if err != nil {
			panic(err)
		}
		y, err := strconv.ParseFloat(split[1], 32)
		if err != nil {
			panic(err)
		}

		return mgl.Scale2D(float32(x), float32(y))
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
	col.a = s.FillOpacity
	if col.a == 0.0 { // Handle missing opacity provided.
		col.a = 1.0
	}

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
				}
			}
		}
	}

	// Draw the outline if it exists.
	if s.Stroke == "" {
		return
	}
	outlineCol := parseColor(s.Stroke)
	outlineCol.a = s.StrokeOpacity
	if outlineCol.a == 0.0 {
		outlineCol.a = 1.0
	}

	for i := 0; i < len(points); i += 2 {
		p1X, p1Y := points[i], points[i+1]
		p2X, p2Y := points[(i+2)%len(points)], points[(i+3)%len(points)]
		r.drawLine(p1X/float32(r.sampleRate), p1Y/float32(r.sampleRate),
			p2X/float32(r.sampleRate), p2Y/float32(r.sampleRate), outlineCol)
	}
}

type Image struct {
	X       int    `xml:"x,attr"`
	Y       int    `xml:"y,attr"`
	Width   int    `xml:"width,attr"`
	Height  int    `xml:"height,attr"`
	Href    string `xml:"href,attr"` // Assume all images of base64 png encoded
	mipMaps []mip
	//imageSizeX int    // Width of image loaded
	//imageSizeY int    // Height of image laoded
}

type mip struct {
	w    int
	h    int
	data []byte
}

func (m *mip) At(x, y int) Color {
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	if x >= m.w {
		x = m.w - 1
	}
	if y >= m.h {
		y = m.h - 1
	}

	i := (x + y*m.w) * 4

	return Color{float32(m.data[i]) / 0xFF,
		float32(m.data[i+1]) / 0xFF,
		float32(m.data[i+2]) / 0xFF,
		float32(m.data[i+3]) / 0xFF}
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

	for w > 1 && h > 1 {
		buff := downSampleBuffer(mips[len(mips)-1].data, 2, w, h)

		w /= 2
		h /= 2

		mips = append(mips, mip{w, h, buff})
	}

	return mips
}

func (s *Image) rasterize(r *rasterizer) {
	// TODO Remove this, if using sampleBilinear on a lower mip this width needs
	// to also be adjusted.
	//s.Width = 64
	//s.Height = s.Width

	// Loop through all the pixels
	// Then get the coordinate from texture space from screenspace?
	// then implmeenet sampleNearest and sampleBiliniear
	for x := s.X; x < s.X+s.Width; x++ {
		for y := s.Y; y < s.Y+s.Height; y++ {
			col := s.sampleNearest(s.mipMaps[0], float32(x), float32(y))
			//col := s.sampleBilinear(s.mipMaps[0], float32(x), float32(y))

			r.drawPixel(float32(x), float32(y), col)
		}
	}
}

func (s *Image) sampleNearest(img mip, x, y float32) Color {
	x -= float32(s.X) + 0.5
	y -= float32(s.Y) + 0.5

	x = x / float32(s.Width) * float32(img.w)
	y = y / float32(s.Height) * float32(img.h)

	return img.At(int(x), int(y))
}

func blendColor(c0, c1 Color, amount float32) Color {
	return Color{blend(c0.r, c1.r, amount), blend(c0.g, c1.g, amount),
		blend(c0.b, c1.b, amount), blend(c0.a, c1.a, amount)}
}

func blend(x0, x1, amount float32) float32 {
	return x0*amount + x1*(1-amount)
}

func (s *Image) sampleBilinear(img mip, x, y float32) Color {
	x = x - float32(s.X) + 0.5
	y = y - float32(s.Y) + 0.5
	x = x / float32(s.Width) * float32(img.w)
	y = y / float32(s.Height) * float32(img.h)

	tt := x - float32(int(x+0.5)) + 0.5
	st := y - float32(int(y+0.5)) + 0.5

	f00 := img.At(int(x-0.5), int(y+0.5))
	f01 := img.At(int(x-0.5), int(y-0.5))
	f10 := img.At(int(x+0.5), int(y+0.5))
	f11 := img.At(int(x+0.5), int(y-0.5))

	c0 := blendColor(f00, f10, tt)
	c1 := blendColor(f01, f11, tt)

	c := blendColor(c0, c1, st)

	return c
}

func New(canvas js.Value, filePath string) (*rasterizer, error) {
	r := &rasterizer{}
	r.canvas = canvas

	b, err := board.New(r.canvas)
	if err != nil {
		panic(err)
	}
	r.board = b

	r.SetSvg(filePath)

	pixelInspectorOn := false
	js.Global().Call("addEventListener", "keydown", js.FuncOf(
		func(this js.Value, args []js.Value) interface{} {
			if args[0].Get("key").String() == "z" {
				pixelInspectorOn = !pixelInspectorOn
				b.EnablePixelInspector(pixelInspectorOn)
			}

			return nil
		}))

	return r, nil
}

func (r *rasterizer) SetSvg(filePath string) error {
	// Get xml file and parse it.
	fileString := getFile(filePath)
	buf := bytes.NewBuffer([]byte(fileString))
	dec := xml.NewDecoder(buf)
	var svg Svg
	if err := dec.Decode(&svg); err != nil {
		return err
	}
	r.svg = &svg

	// Calculate drawing info.
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

	// Update board.
	r.board.SetWidthHeight(r.widthPixels, r.heightPixels)
	r.canvas.Set("width", r.widthPixels)
	r.canvas.Set("height", r.heightPixels)

	// Calculate mip maps for all images.
	loadImagesAndCreateMipMaps(r.svg)

	r.sampleRate = 2

	r.Draw()

	return nil
}

func loadImagesAndCreateMipMaps(curSvg *Svg) {
	for _, imgSvg := range curSvg.Images {
		// Load the image.
		baseImage := strings.Split(imgSvg.Href, ",")[1] // Only works for data:image/png;base64,...
		decoded, err := base64.StdEncoding.DecodeString(baseImage)
		if err != nil { // Remove this.
			panic(err)
		}
		reader := bytes.NewReader(decoded)

		img, err := png.Decode(reader)
		if err != nil {
			panic(err)
		}

		imgSvg.mipMaps = generateMipMaps(img)
	}

	for _, g := range curSvg.Groups {
		loadImagesAndCreateMipMaps(g)
	}

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
	r.colorOfPointsToFill = []Color{}

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
		red := r.pixels[point]
		g := r.pixels[point+1]
		b := r.pixels[point+2]
		a := r.pixels[point+3]

		red, g, b, a = blendColors(r.colorOfPointsToFill[i], red, g, b, a)

		r.pixels[point] = red
		r.pixels[point+1] = g
		r.pixels[point+2] = b
		r.pixels[point+3] = a
	}
	//fmt.Println(r.pixels)
	//fmt.Println(len(r.pixels))

	r.board.SetPixels(r.pixels)

	r.widthPixels, r.heightPixels = r.origWidthPixels, r.origHeightPixels
	r.width, r.height = r.origWidth, r.origHeight
}

func (s *Svg) rasterize(r *rasterizer) {
	//fmt.Println("svg transform", r.svg.transformMatrix)

	fmt.Println(r.width, r.height, r.heightPixels, r.widthPixels)

	for _, rect := range s.Rects {
		rect.transformMatrix = parseTransform(rect.Transform)
		rect.transformMatrix = s.transformMatrix.Mul3(rect.transformMatrix)

		rect.rasterize(r)
	}

	for _, polyline := range s.Polylines {

		polyline.transformMatrix = parseTransform(polyline.Transform)
		polyline.transformMatrix = s.transformMatrix.Mul3(polyline.transformMatrix)

		polyline.rasterize(r)
	}

	for _, line := range s.Lines {
		line.rasterize(r)
	}

	for _, circle := range s.Circles {
		circle.rasterize(r)
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

func getUrl(filePath string) string {
	loc := js.Global().Get("location")
	url := loc.Get("protocol").String() + "//" +
		loc.Get("hostname").String() + ":" +
		loc.Get("port").String()

	return url + filePath
}

func getFile(url string) string {
	resp, err := http.Get(url)
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

type testType struct {
	X   int
	Y   bool
	Z   float32
	W   string
	Fun func()
}

func addSvgToGUI(gui *datGUI.GUI, path string, r *rasterizer) {
	obj := testType{Fun: func() {
		go r.SetSvg(path)
	}}

	split := strings.Split(path, "/")
	name := strings.TrimSuffix(split[len(split)-1], ".svg")
	funController := gui.Add(&obj, "Fun").Name(name)

	svgIcon := js.Global().Get("document").Call("createElement", "img")
	svgIcon.Set("background-color", "white")
	svgIcon.Get("style").Set("background-color", "white")
	svgIcon.Get("style").Set("float", "right")

	height := 75
	svgIcon.Set("src", path)
	svgIcon.Set("height", height)
	funController.JSController.Get("__li").Get("style").Set("height", height)
	funController.JSController.Get("domElement").Get("parentElement").Call("appendChild", svgIcon)
}

func createGui(r *rasterizer) {
	// New GUI
	gui := datGUI.New()

	style := js.Global().Get("document").Call("createElement", "style")
	style.Set("innerHTML", `
    ul.closed > :not(li.title) {
		display: none;
    }`)
	js.Global().Get("document").Get("head").Call("appendChild", style)

	folderNames := []string{"basic", "illustration"}
	svgFiles := [][]string{
		[]string{"test1", "test2", "test3", "test4", "test5", "test6", "test7"},
		[]string{"01_sketchpad", "02_hexes", "03_circle", "04_sun", "05_lion",
			"06_sphere", "07_lines", "08_monkeytree", "09_kochcurve",
		},
	}

	for i, folder := range folderNames {
		folderGUI := gui.AddFolder(folder)
		//folderGUI.Open()
		for _, svgFile := range svgFiles[i] {
			addSvgToGUI(folderGUI, getUrl("/svg/"+folder+"/"+svgFile+".svg"), r)
		}
	}

	//js.Global().Get("document").Get("body").Call("appendChild", svgIcon)

	/* https://github.com/PavelDoGreat/WebGL-Fluid-Simulation/blob/master/script.js
	   let github = gui.add({ fun : () => {
	       window.open('https://github.com/PavelDoGreat/WebGL-Fluid-Simulation');
	       ga('send', 'event', 'link button', 'github');
	   } }, 'fun').name('Github');
	   github.__li.className = 'cr function bigFont';
	   github.__li.style.borderLeft = '3px solid #8C8C8C';
	   let githubIcon = document.createElement('span');
	   github.domElement.parentElement.appendChild(githubIcon);
	   githubIcon.className = 'icon github';
	*/

}

func main() {
	document := js.Global().Get("document")
	canvas := document.Call("getElementById", "webgl")
	canvas.Get("style").Set("border-style", "solid")

	//r, err := New(canvas, "/svg/basic/test1.svg")
	//r, err := New(canvas, "/svg/basic/test2.svg")
	//r, err := New(canvas, "/svg/basic/test3.svg")
	//r, err := New(canvas, "/svg/basic/test4.svg")
	//r, err := New(canvas, "/svg/basic/test5.svg")
	//r, err := New(canvas, "/svg/basic/test6.svg")
	//r, err := New(canvas, "/svg/basic/test7.svg")

	//r, err := New(canvas, "/svg/alpha/01_prism.svg")
	//r, err := New(canvas, "/svg/alpha/02_cube.svg")
	//r, err := New(canvas, "/svg/alpha/03_buckyball.svg")
	//r, err := New(canvas, "/svg/alpha/04_scotty.svg")
	//r, err := New(canvas, "/svg/alpha/05_sphere.svg")

	r, err := New(canvas, getUrl("/svg/illustration/01_sketchpad.svg"))
	//r, err := New(canvas, "/svg/illustration/02_hexes.svg")
	//r, err := New(canvas, "/svg/illustration/03_circle.svg")
	//r, err := New(canvas, "/svg/illustration/04_sun.svg")
	//r, err := New(canvas, "/svg/illustration/05_lion.svg")
	//r, err := New(canvas, "/svg/illustration/06_sphere.svg")
	//r, err := New(canvas, "/svg/illustration/07_lines.svg")
	//r, err := New(canvas, "/svg/illustration/08_monkeytree.svg")
	//r, err := New(canvas, "/svg/illustration/09_kochcurve.svg")

	//r, err := New(canvas, "/svg/hardcore/01_degenerate_square1.svg")
	//r, err := New(canvas, "/svg/hardcore/02_degenerate_square2.svg")

	//r.SetSvg("/svg/illustration/01_sketchpad.svg")

	createGui(r)

	if err != nil {
		panic(err)
	}

	_ = r
	/*
		canvas.Set("height", 900)
		canvas.Set("width", 900)
	*/

	fmt.Println("starting", rand.Int31n(256))

	<-make(chan bool) // Prevent program from exiting
}
