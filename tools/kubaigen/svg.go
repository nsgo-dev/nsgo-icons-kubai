package main

import (
	"encoding/xml"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// element is one element of the markup the upstream renderer writes.
type element struct {
	tag      string
	attrs    map[string]string
	children []*element
	text     string
}

func (e *element) attr(name string) string { return e.attrs[name] }

// parseMarkup reads an icon's inner markup: its defs and layers.
func parseMarkup(markup string) (*element, error) {
	dec := xml.NewDecoder(strings.NewReader("<svg>" + markup + "</svg>"))
	root := &element{tag: "#root", attrs: map[string]string{}}
	stack := []*element{root}
	for {
		tok, err := dec.Token()
		if err != nil {
			if len(stack) == 1 {
				return root, nil
			}
			return nil, err
		}
		top := stack[len(stack)-1]
		switch t := tok.(type) {
		case xml.StartElement:
			el := &element{tag: t.Name.Local, attrs: map[string]string{}}
			for _, a := range t.Attr {
				el.attrs[a.Name.Local] = a.Value
			}
			top.children = append(top.children, el)
			stack = append(stack, el)
		case xml.EndElement:
			stack = stack[:len(stack)-1]
		case xml.CharData:
			top.text += strings.TrimSpace(string(t))
		}
	}
}

// walk visits every element below e in document order.
func (e *element) walk(fn func(*element)) {
	for _, c := range e.children {
		fn(c)
		c.walk(fn)
	}
}

// matrix is an affine transform a, b, c, d, e, f, as SVG's matrix().
type matrix [6]float64

var identity = matrix{1, 0, 0, 1, 0, 0}

func (m matrix) mul(n matrix) matrix {
	return matrix{
		m[0]*n[0] + m[2]*n[1], m[1]*n[0] + m[3]*n[1],
		m[0]*n[2] + m[2]*n[3], m[1]*n[2] + m[3]*n[3],
		m[0]*n[4] + m[2]*n[5] + m[4], m[1]*n[4] + m[3]*n[5] + m[5],
	}
}

func (m matrix) apply(x, y float64) (float64, float64) {
	return m[0]*x + m[2]*y + m[4], m[1]*x + m[3]*y + m[5]
}

// parseTransform reads a transform attribute.
func parseTransform(s string) (matrix, error) {
	m := identity
	s = strings.TrimSpace(s)
	for s != "" {
		open := strings.IndexByte(s, '(')
		end := strings.IndexByte(s, ')')
		if open < 0 || end < open {
			return m, fmt.Errorf("transform %q", s)
		}
		name := strings.TrimSpace(strings.Trim(s[:open], " ,"))
		n, err := numbers(s[open+1 : end])
		if err != nil {
			return m, err
		}
		arg := func(i int, def float64) float64 {
			if i < len(n) {
				return n[i]
			}
			return def
		}
		switch name {
		case "translate":
			m = m.mul(matrix{1, 0, 0, 1, arg(0, 0), arg(1, 0)})
		case "scale":
			sx := arg(0, 1)
			m = m.mul(matrix{sx, 0, 0, arg(1, sx), 0, 0})
		case "rotate":
			r := arg(0, 0) * math.Pi / 180
			c, sn := math.Cos(r), math.Sin(r)
			x, y := arg(1, 0), arg(2, 0)
			m = m.mul(matrix{c, sn, -sn, c, x - c*x + sn*y, y - sn*x - c*y})
		case "matrix":
			if len(n) != 6 {
				return m, fmt.Errorf("matrix %q", s)
			}
			m = m.mul(matrix{n[0], n[1], n[2], n[3], n[4], n[5]})
		case "skewX":
			m = m.mul(matrix{1, 0, math.Tan(arg(0, 0) * math.Pi / 180), 1, 0, 0})
		case "skewY":
			m = m.mul(matrix{1, math.Tan(arg(0, 0) * math.Pi / 180), 0, 1, 0, 0})
		default:
			return m, fmt.Errorf("transform %q", name)
		}
		s = strings.TrimSpace(strings.TrimLeft(s[end+1:], " ,"))
	}
	return m, nil
}

// numbers reads a list of numbers separated by spaces or commas, as SVG
// writes them, a sign or a second point also starting a number.
func numbers(s string) ([]float64, error) {
	var out []float64
	i := 0
	for i < len(s) {
		c := s[i]
		if c == ' ' || c == ',' || c == '\n' || c == '\t' || c == '\r' {
			i++
			continue
		}
		j := i
		if s[j] == '-' || s[j] == '+' {
			j++
		}
		dot := false
		for j < len(s) && (s[j] >= '0' && s[j] <= '9' || s[j] == '.' && !dot) {
			if s[j] == '.' {
				dot = true
			}
			j++
		}
		if j < len(s) && (s[j] == 'e' || s[j] == 'E') {
			j++
			if j < len(s) && (s[j] == '-' || s[j] == '+') {
				j++
			}
			for j < len(s) && s[j] >= '0' && s[j] <= '9' {
				j++
			}
		}
		if j == i {
			return nil, fmt.Errorf("number at %q", s[i:])
		}
		v, err := strconv.ParseFloat(s[i:j], 64)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
		i = j
	}
	return out, nil
}

// seg is a segment of a normalized path: M, L, C or Z with absolute
// points. Quadratic curves stay Q; arcs become four cubic curves.
type seg struct {
	cmd byte
	pts []float64
}

// parsePath normalizes SVG path data: absolute points, H and V as L,
// smooth curves written out and every arc as four cubic curves, so a path
// keeps its shape of commands however its numbers move.
func parsePath(d string) ([]seg, error) {
	var out []seg
	var x, y, sx, sy, cx, cy float64
	var prev byte
	i := 0
	for i < len(d) {
		c := d[i]
		if c == ' ' || c == ',' || c == '\n' || c == '\t' || c == '\r' {
			i++
			continue
		}
		if !strings.ContainsRune("MmLlHhVvCcSsQqTtAaZz", rune(c)) {
			return nil, fmt.Errorf("path command %q", c)
		}
		j := i + 1
		for j < len(d) && !strings.ContainsRune("MmLlHhVvCcSsQqTtAaZz", rune(d[j])) {
			j++
		}
		var n []float64
		var err error
		if c == 'A' || c == 'a' {
			n, err = arcNumbers(d[i+1 : j])
		} else {
			n, err = numbers(d[i+1 : j])
		}
		if err != nil {
			return nil, err
		}
		i = j
		rel := c >= 'a'
		up := c &^ 0x20
		if up == 'Z' {
			out = append(out, seg{cmd: 'Z'})
			x, y = sx, sy
			prev = 'Z'
			continue
		}
		width := map[byte]int{'M': 2, 'L': 2, 'H': 1, 'V': 1, 'C': 6, 'S': 4, 'Q': 4, 'T': 2, 'A': 7}[up]
		if len(n) == 0 || len(n)%width != 0 {
			return nil, fmt.Errorf("path %c with %d numbers", c, len(n))
		}
		for k := 0; k < len(n); k += width {
			a := n[k : k+width]
			ox, oy := 0.0, 0.0
			if rel {
				ox, oy = x, y
			}
			switch up {
			case 'M':
				if k > 0 {
					x, y = a[0]+ox, a[1]+oy
					out = append(out, seg{'L', []float64{x, y}})
					prev = 'L'
					continue
				}
				x, y = a[0]+ox, a[1]+oy
				sx, sy = x, y
				out = append(out, seg{'M', []float64{x, y}})
			case 'L':
				x, y = a[0]+ox, a[1]+oy
				out = append(out, seg{'L', []float64{x, y}})
			case 'H':
				x = a[0] + ox
				out = append(out, seg{'L', []float64{x, y}})
			case 'V':
				y = a[0] + oy
				out = append(out, seg{'L', []float64{x, y}})
			case 'C':
				cx, cy = a[2]+ox, a[3]+oy
				out = append(out, seg{'C', []float64{a[0] + ox, a[1] + oy, cx, cy, a[4] + ox, a[5] + oy}})
				x, y = a[4]+ox, a[5]+oy
			case 'S':
				x1, y1 := x, y
				if prev == 'C' || prev == 'S' {
					x1, y1 = 2*x-cx, 2*y-cy
				}
				cx, cy = a[0]+ox, a[1]+oy
				out = append(out, seg{'C', []float64{x1, y1, cx, cy, a[2] + ox, a[3] + oy}})
				x, y = a[2]+ox, a[3]+oy
			case 'Q':
				cx, cy = a[0]+ox, a[1]+oy
				out = append(out, seg{'Q', []float64{cx, cy, a[2] + ox, a[3] + oy}})
				x, y = a[2]+ox, a[3]+oy
			case 'T':
				x1, y1 := x, y
				if prev == 'Q' || prev == 'T' {
					x1, y1 = 2*x-cx, 2*y-cy
				}
				cx, cy = x1, y1
				out = append(out, seg{'Q', []float64{x1, y1, a[0] + ox, a[1] + oy}})
				x, y = a[0]+ox, a[1]+oy
			case 'A':
				ex, ey := a[5]+ox, a[6]+oy
				out = append(out, arcCubics(x, y, a[0], a[1], a[2], a[3] != 0, a[4] != 0, ex, ey)...)
				x, y = ex, ey
			}
			prev = up
		}
	}
	return out, nil
}

// arcNumbers reads an arc's numbers, whose flags may run into the numbers
// after them ("a1 1 0 011 1").
func arcNumbers(s string) ([]float64, error) {
	var out []float64
	rest := s
	for strings.TrimSpace(strings.Trim(rest, " ,")) != "" {
		for k := 0; k < 7; k++ {
			rest = strings.TrimLeft(rest, " ,\n\t\r")
			if k == 3 || k == 4 {
				if rest == "" || rest[0] != '0' && rest[0] != '1' {
					return nil, fmt.Errorf("arc flag in %q", s)
				}
				out = append(out, float64(rest[0]-'0'))
				rest = rest[1:]
				continue
			}
			end := 1
			for end < len(rest) && !strings.ContainsRune(" ,\n\t\r", rune(rest[end])) &&
				!((rest[end] == '-' || rest[end] == '+') && rest[end-1] != 'e' && rest[end-1] != 'E') &&
				!(rest[end] == '.' && strings.Contains(rest[:end], ".")) {
				end++
			}
			n, err := numbers(rest[:end])
			if err != nil || len(n) != 1 {
				return nil, fmt.Errorf("arc number in %q", s)
			}
			out = append(out, n[0])
			rest = rest[end:]
		}
	}
	return out, nil
}

// arcCubics turns an SVG arc into four cubic curves, each a quarter of its
// sweep, so the count never depends on the arc's numbers.
func arcCubics(x1, y1, rx, ry, rot float64, large, sweep bool, x2, y2 float64) []seg {
	out := make([]seg, 4)
	rx, ry = math.Abs(rx), math.Abs(ry)
	if rx < 1e-9 || ry < 1e-9 || x1 == x2 && y1 == y2 {
		// Nothing to sweep: four straight pieces to the end.
		for k := range out {
			t := float64(k+1) / 4
			px, py := x1+(x2-x1)*t, y1+(y2-y1)*t
			qx, qy := x1+(x2-x1)*(float64(k)/4), y1+(y2-y1)*(float64(k)/4)
			out[k] = seg{'C', []float64{qx, qy, px, py, px, py}}
		}
		return out
	}
	phi := rot * math.Pi / 180
	cs, sn := math.Cos(phi), math.Sin(phi)
	dx, dy := (x1-x2)/2, (y1-y2)/2
	x1p, y1p := cs*dx+sn*dy, -sn*dx+cs*dy
	lam := x1p*x1p/(rx*rx) + y1p*y1p/(ry*ry)
	if lam > 1 {
		s := math.Sqrt(lam)
		rx, ry = rx*s, ry*s
	}
	num := rx*rx*ry*ry - rx*rx*y1p*y1p - ry*ry*x1p*x1p
	den := rx*rx*y1p*y1p + ry*ry*x1p*x1p
	co := 0.0
	if den > 0 && num > 0 {
		co = math.Sqrt(num / den)
	}
	if large == sweep {
		co = -co
	}
	cxp, cyp := co*rx*y1p/ry, -co*ry*x1p/rx
	cx, cy := cs*cxp-sn*cyp+(x1+x2)/2, sn*cxp+cs*cyp+(y1+y2)/2
	ang := func(ux, uy, vx, vy float64) float64 {
		return math.Atan2(ux*vy-uy*vx, ux*vx+uy*vy)
	}
	t1 := ang(1, 0, (x1p-cxp)/rx, (y1p-cyp)/ry)
	dt := ang((x1p-cxp)/rx, (y1p-cyp)/ry, (-x1p-cxp)/rx, (-y1p-cyp)/ry)
	if !sweep && dt > 0 {
		dt -= 2 * math.Pi
	} else if sweep && dt < 0 {
		dt += 2 * math.Pi
	}
	step := dt / 4
	k := 4.0 / 3 * math.Tan(step/4)
	pt := func(t float64) (float64, float64) {
		ex, ey := rx*math.Cos(t), ry*math.Sin(t)
		return cs*ex - sn*ey + cx, sn*ex + cs*ey + cy
	}
	der := func(t float64) (float64, float64) {
		ex, ey := -rx*math.Sin(t), ry*math.Cos(t)
		return cs*ex - sn*ey, sn*ex + cs*ey
	}
	for i := range out {
		a, b := t1+step*float64(i), t1+step*float64(i+1)
		ax, ay := pt(a)
		bx, by := pt(b)
		if i == 3 {
			bx, by = x2, y2
		}
		dax, day := der(a)
		dbx, dby := der(b)
		out[i] = seg{'C', []float64{ax + k*dax, ay + k*day, bx - k*dbx, by - k*dby, bx, by}}
	}
	return out
}

// circlePath is a circle as four cubic curves, counter-clockwise as the
// upstream kit's circles are.
func ellipsePath(cx, cy, rx, ry float64) []seg {
	const k = 0.5522847498307936
	return []seg{
		{'M', []float64{cx + rx, cy}},
		{'C', []float64{cx + rx, cy - k*ry, cx + k*rx, cy - ry, cx, cy - ry}},
		{'C', []float64{cx - k*rx, cy - ry, cx - rx, cy - k*ry, cx - rx, cy}},
		{'C', []float64{cx - rx, cy + k*ry, cx - k*rx, cy + ry, cx, cy + ry}},
		{'C', []float64{cx + k*rx, cy + ry, cx + rx, cy + k*ry, cx + rx, cy}},
		{'Z', nil},
	}
}

// rectPath is a rectangle with corners rounded by rx, ry, clockwise; its
// corners are always curves, so a radius that animates to 0 keeps them.
func rectPath(x, y, w, h, rx, ry float64) []seg {
	rx, ry = min(rx, w/2), min(ry, h/2)
	const k = 0.5522847498307936
	return []seg{
		{'M', []float64{x + rx, y}},
		{'L', []float64{x + w - rx, y}},
		{'C', []float64{x + w - rx + k*rx, y, x + w, y + ry - k*ry, x + w, y + ry}},
		{'L', []float64{x + w, y + h - ry}},
		{'C', []float64{x + w, y + h - ry + k*ry, x + w - rx + k*rx, y + h, x + w - rx, y + h}},
		{'L', []float64{x + rx, y + h}},
		{'C', []float64{x + rx - k*rx, y + h, x, y + h - ry + k*ry, x, y + h - ry}},
		{'L', []float64{x, y + ry}},
		{'C', []float64{x, y + ry - k*ry, x + rx - k*rx, y, x + rx, y}},
		{'Z', nil},
	}
}

// pathLength is the length of a path, its curves flattened.
func pathLength(segs []seg) float64 {
	var total, x, y, sx, sy float64
	for _, s := range segs {
		switch s.cmd {
		case 'M':
			x, y = s.pts[0], s.pts[1]
			sx, sy = x, y
		case 'L':
			total += math.Hypot(s.pts[0]-x, s.pts[1]-y)
			x, y = s.pts[0], s.pts[1]
		case 'Q', 'C':
			px, py := x, y
			for i := 1; i <= 32; i++ {
				t := float64(i) / 32
				var qx, qy float64
				if s.cmd == 'Q' {
					u := 1 - t
					qx = u*u*x + 2*u*t*s.pts[0] + t*t*s.pts[2]
					qy = u*u*y + 2*u*t*s.pts[1] + t*t*s.pts[3]
				} else {
					u := 1 - t
					qx = u*u*u*x + 3*u*u*t*s.pts[0] + 3*u*t*t*s.pts[2] + t*t*t*s.pts[4]
					qy = u*u*u*y + 3*u*u*t*s.pts[1] + 3*u*t*t*s.pts[3] + t*t*t*s.pts[5]
				}
				total += math.Hypot(qx-px, qy-py)
				px, py = qx, qy
			}
			x, y = s.pts[len(s.pts)-2], s.pts[len(s.pts)-1]
		case 'Z':
			total += math.Hypot(sx-x, sy-y)
			x, y = sx, sy
		}
	}
	return total
}
