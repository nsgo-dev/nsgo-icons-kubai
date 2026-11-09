package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// slotKind says what a number in a compiled frame may do across frames.
type slotKind uint8

const (
	// slotSym is not a number: a letter of the program.
	slotSym slotKind = iota
	// slotPoint is a coordinate or a length the platform animates.
	slotPoint
	// slotLinear is a transform's linear part, written more precisely.
	slotLinear
	// slotAlpha is a paint's opacity; a hidden variant sets it to 0.
	slotAlpha
	// slotRest keeps its value from the rest pose: a gradient's ends and a
	// stroke's dashes, which the platform does not animate.
	slotRest
)

type tok struct {
	kind slotKind
	s    string
	v    float64
}

// style is what an element inherits.
type style struct {
	fill, stroke     string
	fillRule         string
	width            float64
	cap, join        string
	dash             []float64
	dashOffset       float64
	pathLen          float64
	opacity          float64
	fillOp, strokeOp float64
	shadow, light    bool
}

// compiler turns one frame's markup into a program of letters and
// numbers. Frames of one motion compile to the same letters, so their
// numbers line up.
type compiler struct {
	toks  []tok
	ids   map[string]*element
	grads map[string]int
	names []string
	paths int
	seen  map[string]string
	// union holds the attributes an element has in some frame of the
	// motion, by its place in the tree, so each frame writes them all.
	union map[string]map[string]bool
	warn  map[string]bool
}

func (c *compiler) sym(s string)          { c.toks = append(c.toks, tok{kind: slotSym, s: s}) }
func (c *compiler) num(k slotKind, v float64) { c.toks = append(c.toks, tok{kind: k, v: v}) }

var optional = []string{"transform", "opacity", "stroke-dashoffset", "fill-opacity", "stroke-opacity", "stroke-width"}

// collectUnion records the optional attributes each element has.
func collectUnion(root *element, union map[string]map[string]bool) {
	var visit func(e *element, key string)
	visit = func(e *element, key string) {
		for i, ch := range e.children {
			k := key + "." + strconv.Itoa(i)
			for _, a := range optional {
				if _, ok := ch.attrs[a]; ok {
					if union[k] == nil {
						union[k] = map[string]bool{}
					}
					union[k][a] = true
				}
			}
			visit(ch, k)
		}
	}
	visit(root, "")
}

func compileFrame(root *element, union map[string]map[string]bool, warn map[string]bool) ([]tok, error) {
	c := &compiler{ids: map[string]*element{}, grads: map[string]int{}, union: union, warn: warn, seen: map[string]string{}}
	root.walk(func(e *element) {
		if id := e.attr("id"); id != "" {
			c.ids[id] = e
		}
	})
	st := style{fill: "none", stroke: "none", fillRule: "n", width: 1, cap: "b", join: "m", opacity: 1, fillOp: 1, strokeOp: 1}
	if err := c.children(root, "", st); err != nil {
		return nil, err
	}
	// The gradients the program paints with come first.
	var head []tok
	for _, id := range c.names {
		g := c.ids[id]
		var stops []*element
		for _, s := range g.children {
			if s.tag == "stop" {
				stops = append(stops, s)
			}
		}
		head = append(head, tok{s: "G"}, tok{s: strconv.Itoa(len(stops))})
		for _, s := range stops {
			off, err := strconv.ParseFloat(strings.TrimSuffix(s.attr("offset"), "%"), 64)
			if err != nil {
				return nil, err
			}
			if strings.HasSuffix(s.attr("offset"), "%") {
				off /= 100
			}
			col, ok := colorToken(s.attr("stop-color"))
			if !ok {
				return nil, fmt.Errorf("stop color %q", s.attr("stop-color"))
			}
			head = append(head, tok{kind: slotRest, v: off}, tok{s: col})
		}
	}
	return append(head, c.toks...), nil
}

func (c *compiler) children(e *element, key string, st style) error {
	for i, ch := range e.children {
		if err := c.element(ch, key+"."+strconv.Itoa(i), st); err != nil {
			return err
		}
	}
	return nil
}

func parseNum(s string, def float64) float64 {
	if s == "" {
		return def
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return def
	}
	return v
}

func (c *compiler) has(e *element, key, attr string) bool {
	if _, ok := e.attrs[attr]; ok {
		return true
	}
	return c.union[key][attr]
}

func (c *compiler) element(e *element, key string, st style) error {
	switch e.tag {
	case "defs", "clipPath", "mask", "linearGradient", "radialGradient", "filter", "title", "desc":
		return nil
	}
	if e.attr("display") == "none" {
		return nil
	}
	// Inherited paint properties.
	if v, ok := e.attrs["fill"]; ok {
		st.fill = v
	}
	if v, ok := e.attrs["stroke"]; ok {
		st.stroke = v
	}
	if v, ok := e.attrs["fill-rule"]; ok {
		st.fillRule = map[bool]string{true: "e", false: "n"}[v == "evenodd"]
	}
	if c.has(e, key, "stroke-width") {
		st.width = parseNum(e.attr("stroke-width"), st.width)
	}
	if v, ok := e.attrs["stroke-linecap"]; ok {
		st.cap = map[string]string{"round": "r", "square": "s"}[v]
		if st.cap == "" {
			st.cap = "b"
		}
	}
	if v, ok := e.attrs["stroke-linejoin"]; ok {
		st.join = map[string]string{"round": "r", "bevel": "v"}[v]
		if st.join == "" {
			st.join = "m"
		}
	}
	if v, ok := e.attrs["stroke-dasharray"]; ok {
		if v == "none" {
			st.dash = nil
		} else {
			n, err := numbers(v)
			if err != nil {
				return err
			}
			if len(n)%2 == 1 {
				n = append(n, n...)
			}
			st.dash = n
		}
	}
	if c.has(e, key, "stroke-dashoffset") {
		st.dashOffset = parseNum(e.attr("stroke-dashoffset"), 0)
	}
	if v, ok := e.attrs["pathLength"]; ok {
		st.pathLen = parseNum(v, 0)
	}
	if c.has(e, key, "fill-opacity") {
		st.fillOp = parseNum(e.attr("fill-opacity"), 1)
	}
	if c.has(e, key, "stroke-opacity") {
		st.strokeOp = parseNum(e.attr("stroke-opacity"), 1)
	}
	if c.has(e, key, "opacity") {
		st.opacity *= parseNum(e.attr("opacity"), 1)
	}
	if f := e.attr("filter"); f != "" {
		switch {
		case strings.HasPrefix(f, "url(#sh-"):
			st.shadow = true
		case strings.HasPrefix(f, "url(#hi-"):
			st.light = true
		default:
			c.warn["filter "+f] = true
		}
	}
	// A transform, a clip and a mask open a state of their own.
	framed := c.has(e, key, "transform") || e.attr("clip-path") != "" || e.attr("mask") != ""
	if framed {
		c.sym("S")
	}
	if c.has(e, key, "transform") {
		m, err := parseTransform(e.attr("transform"))
		if err != nil {
			return err
		}
		c.sym("X")
		for i, v := range m {
			k := slotPoint
			if i < 4 {
				k = slotLinear
			}
			c.num(k, v)
		}
	}
	if cp := e.attr("clip-path"); cp != "" {
		if err := c.clip(cp); err != nil {
			return err
		}
	}
	if mk := e.attr("mask"); mk != "" {
		if err := c.mask(mk, e); err != nil {
			return err
		}
	}
	var err error
	switch e.tag {
	case "g", "a", "svg":
		err = c.children(e, key, st)
	case "text":
		err = c.text(e, st)
	default:
		var segs []seg
		segs, err = shapeSegs(e)
		if err == nil && segs != nil {
			err = c.shape(segs, st)
		}
	}
	if err != nil {
		return err
	}
	if framed {
		c.sym("R")
	}
	return nil
}

// shapeSegs is a shape element's path; nil for elements that draw nothing.
func shapeSegs(e *element) ([]seg, error) {
	n := func(a string) float64 { return parseNum(e.attr(a), 0) }
	switch e.tag {
	case "path":
		return parsePath(e.attr("d"))
	case "circle":
		return ellipsePath(n("cx"), n("cy"), n("r"), n("r")), nil
	case "ellipse":
		return ellipsePath(n("cx"), n("cy"), n("rx"), n("ry")), nil
	case "rect":
		rx, ry := e.attr("rx"), e.attr("ry")
		if rx == "" {
			rx = ry
		}
		if ry == "" {
			ry = rx
		}
		return rectPath(n("x"), n("y"), n("width"), n("height"), parseNum(rx, 0), parseNum(ry, 0)), nil
	case "line":
		return []seg{{'M', []float64{n("x1"), n("y1")}}, {'L', []float64{n("x2"), n("y2")}}}, nil
	case "polyline", "polygon":
		pts, err := numbers(e.attr("points"))
		if err != nil || len(pts) < 4 {
			return nil, err
		}
		segs := []seg{{'M', pts[:2]}}
		for i := 2; i+1 < len(pts); i += 2 {
			segs = append(segs, seg{'L', pts[i : i+2]})
		}
		if e.tag == "polygon" {
			segs = append(segs, seg{'Z', nil})
		}
		return segs, nil
	}
	return nil, fmt.Errorf("element <%s>", e.tag)
}

// path writes a path definition and returns its number.
// A shape, its outline and its outline's mask share one path.
func (c *compiler) path(segs []seg) string {
	var key strings.Builder
	for _, s := range segs {
		key.WriteByte(s.cmd)
		for _, v := range s.pts {
			key.WriteString(strconv.FormatFloat(v, 'g', -1, 64))
			key.WriteByte(' ')
		}
	}
	if id, ok := c.seen[key.String()]; ok {
		return id
	}
	c.sym("P")
	for _, s := range segs {
		c.sym(string(s.cmd))
		for _, v := range s.pts {
			c.num(slotPoint, v)
		}
	}
	c.sym(";")
	id := strconv.Itoa(c.paths)
	c.paths++
	c.seen[key.String()] = id
	return id
}

func bounds(segs []seg) (x0, y0, x1, y1 float64) {
	x0, y0, x1, y1 = math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, s := range segs {
		for i := 0; i+1 < len(s.pts); i += 2 {
			x0, x1 = min(x0, s.pts[i]), max(x1, s.pts[i])
			y0, y1 = min(y0, s.pts[i+1]), max(y1, s.pts[i+1])
		}
	}
	if x0 > x1 {
		return 0, 0, 0, 0
	}
	return
}

// paint writes a paint: a color, or a gradient across the shape's bounds.
func (c *compiler) paint(p string, segs []seg) error {
	if strings.HasPrefix(p, "url(#") {
		id := strings.TrimSuffix(strings.TrimPrefix(p, "url(#"), ")")
		g := c.ids[id]
		if g == nil || g.tag != "linearGradient" {
			return fmt.Errorf("paint %q", p)
		}
		i, ok := c.grads[id]
		if !ok {
			i = len(c.names)
			c.grads[id] = i
			c.names = append(c.names, id)
		}
		c.sym("g" + strconv.Itoa(i))
		bx0, by0, bx1, by1 := bounds(segs)
		w, h := bx1-bx0, by1-by0
		gx := func(a string, def float64) float64 { return parseNum(g.attr(a), def) }
		if g.attr("gradientUnits") == "userSpaceOnUse" {
			c.num(slotRest, gx("x1", 0))
			c.num(slotRest, gx("y1", 0))
			c.num(slotRest, gx("x2", 1))
			c.num(slotRest, gx("y2", 0))
			return nil
		}
		c.num(slotRest, bx0+gx("x1", 0)*w)
		c.num(slotRest, by0+gx("y1", 0)*h)
		c.num(slotRest, bx0+gx("x2", 1)*w)
		c.num(slotRest, by0+gx("y2", 0)*h)
		return nil
	}
	col, ok := colorToken(p)
	if !ok {
		return fmt.Errorf("paint %q", p)
	}
	c.sym(col)
	return nil
}

// colorToken writes a color of the template paint: a hex color, a tint of
// a reference color ({t#93BBFFc}), the white of marks ({w}) or the top of a
// solid's gradient ({p…}).
func colorToken(s string) (string, bool) {
	s = strings.TrimSpace(s)
	switch {
	case s == "{w}":
		return "w", true
	case strings.HasPrefix(s, "{p") && strings.HasSuffix(s, "}"):
		inner, ok := colorToken(s[2 : len(s)-1])
		return "p" + inner, ok
	case strings.HasPrefix(s, "{t#") && strings.HasSuffix(s, "}"):
		body := s[3 : len(s)-1]
		if len(body) < 7 {
			return "", false
		}
		src, rest := strings.ToUpper(body[:6]), body[6:]
		switch {
		case rest == "c" || rest == "e":
			return "t" + src + rest, true
		case strings.HasPrefix(rest, "b#") && len(rest) == 8:
			return "t" + src + "b" + strings.ToUpper(rest[2:]), true
		}
		return "", false
	case s == "white":
		return "#FFFFFF", true
	case s == "black":
		return "#000000", true
	case strings.HasPrefix(s, "#") && len(s) == 4:
		return "#" + strings.ToUpper(string([]byte{s[1], s[1], s[2], s[2], s[3], s[3]})), true
	case strings.HasPrefix(s, "#") && len(s) == 7:
		return "#" + strings.ToUpper(s[1:]), true
	}
	return "", false
}

func (c *compiler) alpha(v float64) {
	c.sym("A")
	c.num(slotAlpha, v)
}

func (c *compiler) shape(segs []seg, st style) error {
	if len(segs) == 0 {
		return nil
	}
	fill := st.fill != "none" && st.fill != "transparent"
	stroke := st.stroke != "none" && st.stroke != "transparent"
	if !fill && !stroke {
		return nil
	}
	id := c.path(segs)
	if fill {
		if st.shadow {
			c.sym("W")
		}
		c.alpha(st.opacity * st.fillOp)
		c.sym("F")
		c.sym(id)
		if err := c.paint(st.fill, segs); err != nil {
			return err
		}
		c.sym(st.fillRule)
		if st.shadow {
			c.sym("w")
		}
		if st.light {
			c.sym("H")
			c.sym(id)
			c.sym(st.fillRule)
		}
	}
	if stroke {
		if st.shadow {
			c.sym("W")
		}
		c.alpha(st.opacity * st.strokeOp)
		c.sym("K")
		c.sym(id)
		if err := c.paint(st.stroke, segs); err != nil {
			return err
		}
		c.num(slotPoint, st.width)
		c.sym(st.cap + st.join)
		scale := 1.0
		if st.pathLen > 0 && len(st.dash) > 0 {
			scale = pathLength(segs) / st.pathLen
		}
		c.sym(strconv.Itoa(len(st.dash)))
		if len(st.dash) > 0 {
			for _, v := range st.dash {
				c.num(slotRest, v*scale)
			}
			c.num(slotPoint, st.dashOffset*scale)
		}
		if st.shadow {
			c.sym("w")
		}
	}
	return nil
}

// geometry is the path of shapes inside a clip path or a mask, their
// transforms applied.
func (c *compiler) geometry(e *element, m matrix, out *[]seg, keep func(*element) bool) error {
	for _, ch := range e.children {
		cm := m
		if t := ch.attr("transform"); t != "" {
			tm, err := parseTransform(t)
			if err != nil {
				return err
			}
			cm = m.mul(tm)
		}
		if ch.tag == "g" {
			if err := c.geometry(ch, cm, out, keep); err != nil {
				return err
			}
			continue
		}
		if !keep(ch) {
			continue
		}
		segs, err := shapeSegs(ch)
		if err != nil {
			return err
		}
		for _, s := range segs {
			pts := make([]float64, len(s.pts))
			for i := 0; i+1 < len(s.pts); i += 2 {
				pts[i], pts[i+1] = cm.apply(s.pts[i], s.pts[i+1])
			}
			*out = append(*out, seg{s.cmd, pts})
		}
	}
	return nil
}

func (c *compiler) ref(u string) *element {
	if !strings.HasPrefix(u, "url(#") {
		return nil
	}
	return c.ids[strings.TrimSuffix(strings.TrimPrefix(u, "url(#"), ")")]
}

func (c *compiler) clip(u string) error {
	cp := c.ref(u)
	if cp == nil {
		return fmt.Errorf("clip %q", u)
	}
	var segs []seg
	rule := "n"
	if err := c.geometry(cp, identity, &segs, func(e *element) bool {
		if e.attr("clip-rule") == "evenodd" {
			rule = "e"
		}
		return true
	}); err != nil {
		return err
	}
	id := c.path(segs)
	c.sym("c")
	c.sym(id)
	c.sym(rule)
	return nil
}

// mask writes the masks the renderer uses as a clip outside a shape: a
// white rectangle with black shapes cut out of it, as the outside outline
// and a badge's gap are. Other masks are left out.
func (c *compiler) mask(u string, e *element) error {
	mk := c.ref(u)
	if mk == nil {
		return fmt.Errorf("mask %q", u)
	}
	cut := true
	for i, ch := range mk.children {
		col, _ := colorToken(ch.attr("fill"))
		if i == 0 && ch.tag == "rect" && col == "#FFFFFF" {
			continue
		}
		if col != "#000000" {
			cut = false
		}
	}
	if !cut || len(mk.children) < 2 {
		c.warn["mask "+mk.attr("id")] = true
		return nil
	}
	var segs []seg
	rule := "n"
	if err := c.geometry(mk, identity, &segs, func(ch *element) bool {
		col, _ := colorToken(ch.attr("fill"))
		if col == "#000000" && ch.attr("fill-rule") == "evenodd" {
			rule = "e"
		}
		return col == "#000000"
	}); err != nil {
		return err
	}
	id := c.path(segs)
	c.sym("o")
	c.sym(id)
	c.sym(rule)
	return nil
}

func (c *compiler) text(e *element, st style) error {
	if st.fill == "none" {
		return nil
	}
	c.alpha(st.opacity * st.fillOp)
	c.sym("T")
	c.num(slotPoint, parseNum(e.attr("x"), 0))
	c.num(slotPoint, parseNum(e.attr("y"), 0))
	c.sym(strconv.FormatFloat(parseNum(e.attr("font-size"), 16), 'f', -1, 64))
	c.sym(strconv.Itoa(int(parseNum(e.attr("font-weight"), 400))))
	anchor := map[string]string{"middle": "m", "end": "e"}[e.attr("text-anchor")]
	if anchor == "" {
		anchor = "s"
	}
	c.sym(anchor)
	if err := c.paint(st.fill, nil); err != nil {
		return err
	}
	c.sym(strconv.Quote(e.text))
	return nil
}
