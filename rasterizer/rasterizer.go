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

type vec2 struct {
	x float32
	y float32
}

// Code translated from
// https://github.com/CMU-Graphics/DrawSVG/blob/master/src/triangulation.cpp
// TODO find and implement an algorithm on your own.
func triangulate(points []float32) []*Triangle {
	contour := []vec2{}
	for i := 0; i < len(points); i += 2 {
		contour = append(contour, vec2{points[i], points[i+1]})
	}

	// Initialize list of vertices in the polygon
	n := len(contour)
	if n < 3 {
		return nil
	}

	// We want a counter-clockwise polygon in V
	V := make([]int, n)
	if 0.0 < area(contour) {
		for v := 0; v < n; v++ {
			V[v] = v
		}
	} else {
		for v := 0; v < n; v++ {
			V[v] = (n - 1) - v
		}
	}

	nv := n

	// Remove nv-2 Vertices, each time creating a triangle
	triangles := []*Triangle{}
	count := 2 * nv // Error detection
	for m, v := 0, nv-1; nv > 2; {
		// If we loop it is likely a non-simple polygon
		if 0 >= count {
			return triangles // Error, probably a bad polygon!
		}
		count -= 1

		// Three consecutive vertices in current polygon, <u,v,w>
		u := v
		if nv <= u { // prev
			u = 0
		}
		v = u + 1
		if nv <= v { // new v
			v = 0
		}
		w := v + 1
		if nv <= w { // net
			w = 0
		}

		//fmt.Println("u,v,w", u, v, w)

		if snip(contour, u, v, w, nv, V) {
			var a, b, c, s, t int
			a, b, c = V[u], V[v], V[w]

			/*
				fmt.Printf("nv %d, (%f, %f, %f, %f, %f, %f)\n", nv,
					contour[a].x, contour[a].y,
					contour[b].x, contour[b].y,
					contour[c].x, contour[c].y)
			*/

			triangles = append(triangles, &Triangle{
				contour[a].x, contour[a].y,
				contour[b].x, contour[b].y,
				contour[c].x, contour[c].y,
			})

			m += 1

			// Remove v from remaining polygon
			s, t = v, v+1
			for t < nv {
				//fmt.Println("s, t", s, t)

				V[s] = V[t]
				s += 1
				t += 1
			}
			nv -= 1

			count = 2 * nv // reset error detection counter
		}
		//fmt.Println("nv", nv)
	}
	return triangles
}

func area(contour []vec2) float32 {
	n := len(contour)
	a := float32(0.0)

	p, q := n-1, 0
	for q < n {
		a += contour[p].x*contour[q].y - contour[q].x*contour[p].y

		p = q
		q += 1
	}

	return a * 0.5
}

func snip(contour []vec2, u, v, w, n int, V []int) bool {
	const EPSILON = 0.0000000001

	Ax := contour[V[u]].x
	Ay := contour[V[u]].y

	Bx := contour[V[v]].x
	By := contour[V[v]].y

	Cx := contour[V[w]].x
	Cy := contour[V[w]].y

	if EPSILON > (((Bx - Ax) * (Cy - Ay)) - ((By - Ay) * (Cx - Ax))) {
		return false
	}

	for p := 0; p < n; p++ {
		if p == u || p == v || p == w {
			continue
		}
		Px := contour[V[p]].x
		Py := contour[V[p]].y
		if inside(Ax, Ay, Bx, By, Cx, Cy, Px, Py) {
			return false
		}
	}

	return true
}

func inside(Ax, Ay, Bx, By, Cx, Cy, Px, Py float32) bool {
	ax, ay := Cx-Bx, Cy-By
	bx, by := Ax-Cx, Ay-Cy
	cx, cy := Bx-Ax, By-Ay

	apx, apy := Px-Ax, Py-Ay
	bpx, bpy := Px-Bx, Py-By
	cpx, cpy := Px-Cx, Py-Cy

	aCROSSbp := ax*bpy - ay*bpx
	cCROSSap := cx*apy - cy*apx
	bCROSScp := bx*cpy - by*cpx

	return aCROSSbp >= 0.0 && bCROSScp >= 0.0 && cCROSSap >= 0.0
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

func parseColor(col string) (byte, byte, byte, byte) {
	if len(col) == 6 { // Add in # if missing
		col = "#" + col
	}

	r, _ := strconv.ParseInt(col[1:3], 16, 9)
	g, _ := strconv.ParseInt(col[3:5], 16, 9)
	b, _ := strconv.ParseInt(col[5:7], 16, 9)
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
	XMLName  xml.Name
	Width    string    `xml:"width,attr"`
	Height   string    `xml:"height,attr"`
	ViewBox  string    `xml:"viewBox,attr"`
	Rects    []Rect    `xml:"rect"`
	Lines    []Line    `xml:"line"`
	Polygons []Polygon `xml:"polygon"`
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
	/*
		x, y = x*float32(r.widthPixels)/r.width, y*float32(r.heightPixels)/r.height
		x = float32(math.Round(float64(x)))
		y = float32(math.Round(float64(y)))
		xCoord, yCoord := int(x), r.heightPixels-int(y)
	*/

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
	r.drawLine(s.X1, s.Y1, s.X2, s.Y2, red, g, b, a)
}

func (r *rasterizer) drawLine(x1, y1, x2, y2 float32, red, g, b, a byte) {
	slope := (y2 - y1) / (x2 - x1)

	// Slope greater than one case
	if math.Abs(float64(slope)) > 1.0 {
		if y1 < y2 {
			r.bresenham(y1, x1, y2, 1.0/slope, red, g, b, a, true)
		} else { // Flip and y1 and y2
			r.bresenham(y2, x2, y1, 1.0/slope, red, g, b, a, true)
		}
		return
	}

	// Slope less than one case
	if x1 < x2 {
		r.bresenham(x1, y1, x2, slope, red, g, b, a, false)
	} else { // Flip and x1 and x2
		r.bresenham(x2, y2, x1, slope, red, g, b, a, false)
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

type Polygon struct {
	Fill   string `xml:"fill,attr"`
	Points string `xml:"points,attr"`
}

type Triangle struct {
	X1 float32
	Y1 float32
	X2 float32
	Y2 float32
	X3 float32
	Y3 float32
}

func pointsToTriangles(in string) []*Triangle {
	// TODO implmenet an algorithm to convert polygons into triangles
	// https://stackoverflow.com/questions/7316000/convert-polygon-to-triangles
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

	triangles := triangulate(pointsFloat)
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
	return triangles //[]Triangle{t}
}

func (s *Polygon) rasterize(r *rasterizer) {
	//s.flatTriangleApproach(r)
	s.boundingBoxApproach(r)

	// TODO figure out how to draw outline
	// Think we need to unsort and rely on the algorithm to tell us the outline?
	/*
	  c = polygon.style.strokeColor;
	  if( c.a != 0 ) {
	    int nPoints = polygon.points.size();
	    for( int i = 0; i < nPoints; i++ ) {
	      Vector2D p0 = transform(polygon.points[(i+0) % nPoints]);
	      Vector2D p1 = transform(polygon.points[(i+1) % nPoints]);
	      rasterize_line( p0.x, p0.y, p1.x, p1.y, c );
	    }
	  }
	*/
}

func (s *Polygon) boundingBoxApproach(r *rasterizer) {
	triangles := pointsToTriangles(s.Points)
	fmt.Println("Len of triangles", len(triangles), r.widthPixels, r.heightPixels)
	//triangles = triangles[:5]

	// TODO for loop these triangles when we start doing filling and polygons
	//t := triangles[0]
	for _, t := range triangles {

		fmt.Println(t)

		red, g, b, a := parseColor(s.Fill)

		/*
			r.drawLine(t.X1, t.Y1, t.X2, t.Y2, red, g, b, a)
			r.drawLine(t.X2, t.Y2, t.X3, t.Y3, red, g, b, a)
			r.drawLine(t.X1, t.Y1, t.X3, t.Y3, red, g, b, a)
		*/

		minX := minOfThree(t.X1, t.X2, t.X3)
		maxX := maxOfThree(t.X1, t.X2, t.X3)
		minY := minOfThree(t.Y1, t.Y2, t.Y3)
		maxY := maxOfThree(t.Y1, t.Y2, t.Y3)

		vsX1, vsY1 := t.X2-t.X1, t.Y2-t.Y1
		vsX2, vsY2 := t.X3-t.X1, t.Y3-t.Y1
		for x := float32(int(minX)); x <= maxX; x++ {
			for y := float32(int(minY)); y <= maxY; y++ {
				qx, qy := x-t.X1, y-t.Y1

				s := crossProduct(qx, qy, vsX2, vsY2) / crossProduct(vsX1, vsY1, vsX2, vsY2)
				t := crossProduct(vsX1, vsY1, qx, qy) / crossProduct(vsX1, vsY1, vsX2, vsY2)

				if s >= 0 && t >= 0 && s+t <= 1 {
					r.drawPoint(x, y, red, g, b, a)
				}
			}
		}
	}
}

func (s *Polygon) flatTriangleApproach(r *rasterizer) {
	triangles := pointsToTriangles(s.Points)

	// TODO for loop these triangles when we start doing filling and polygons
	for _, t := range triangles {
		/*
			red, g, b, a := parseColor(s.Fill)
			r.drawLine(t.X1, t.Y1, t.X2, t.Y2, red, g, b, a)
			r.drawLine(t.X2, t.Y2, t.X3, t.Y3, red, g, b, a)
			r.drawLine(t.X1, t.Y1, t.X3, t.Y3, red, g, b, a)
		*/

		red, g, b, a := parseColor(s.Fill)

		// http://www.sunshine2k.de/coding/java/TriangleRasterization/TriangleRasterization.html
		if t.Y2 == t.Y3 { // Only flat bottom triangle
			r.drawFlatBottomTriangle(t.X1, t.Y1, t.X2, t.Y2, t.X3, t.Y3, red, g, b, a)
			return
		} else if t.Y1 == t.Y2 { // Only flat top triangle
			r.drawFlatTopTriangle(t.X1, t.Y1, t.X2, t.Y2, t.X3, t.Y3, red, g, b, a)
			return
		}

		// Split triangle into a topflat and bottom flat triangle
		x4 := t.X1 + (t.Y2-t.Y1)/(t.Y3-t.Y1)*(t.X3-t.X1)
		y4 := t.Y2

		r.drawFlatBottomTriangle(t.X1, t.Y1, t.X2, t.Y2, x4, y4, red, g, b, a)
		r.drawFlatTopTriangle(t.X2, t.Y2, x4, y4, t.X3, t.Y3, red, g, b, a)

	}
}

func (r *rasterizer) drawFlatBottomTriangle(x1, y1, x2, y2, x3, y3 float32, red, g, b, a byte) {
	invSlope1 := (x2 - x1) / (y2 - y1)
	invSlope2 := (x3 - x1) / (y3 - y1)
	curX1, curX2 := x1, x1
	for scanLineY := y1; scanLineY <= y2; scanLineY++ {
		r.drawLine(curX1, scanLineY, curX2, scanLineY, red, g, b, a)

		curX1 += invSlope1
		curX2 += invSlope2
	}
}

func (r *rasterizer) drawFlatTopTriangle(x1, y1, x2, y2, x3, y3 float32, red, g, b, a byte) {
	invSlope1 := (x3 - x1) / (y3 - y1)
	invSlope2 := (x3 - x2) / (y3 - y2)
	curX1, curX2 := x3, x3

	for scanLineY := y3; scanLineY > y1; scanLineY-- {
		r.drawLine(curX1, scanLineY, curX2, scanLineY, red, g, b, a)

		curX1 -= invSlope1
		curX2 -= invSlope2
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

	//fmt.Println("FMT", widthPixels, heightPixels, width, height)

	// Create board.
	canvas.Set("height", r.widthPixels)
	canvas.Set("width", r.heightPixels)
	b, err := board.New(canvas)
	if err != nil {
		panic(err)
	}

	pixelInspectorOn := false
	js.Global().Call("addEventListener", "keydown", js.FuncOf(
		func(this js.Value, args []js.Value) interface{} {
			if args[0].Get("keyCode").Int() == 90 { // z
				b.EnablePixelInspector(pixelInspectorOn)
				pixelInspectorOn = !pixelInspectorOn
			}
			return nil
		}))

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
	for _, polygon := range r.svg.Polygons {
		polygon.rasterize(r)
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
	//r, err := New(canvas, "/svg/test2.svg")
	//r, err := New(canvas, "/svg/test3.svg")
	//r, err := New(canvas, "/svg/test4.svg")
	r, err := New(canvas, "/svg/test5.svg")
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
