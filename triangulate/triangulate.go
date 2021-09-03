package triangulate

type Triangle struct {
	X1 float32
	Y1 float32
	X2 float32
	Y2 float32
	X3 float32
	Y3 float32
}

type vec2 struct {
	x float32
	y float32
}

// Code translated from
// https://github.com/CMU-Graphics/DrawSVG/blob/master/src/triangulation.cpp
// TODO find and implement an algorithm on your own.
func Triangulate(points []float32) []*Triangle {
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
