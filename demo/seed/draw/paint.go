package main

// Маленький растеризатор: фигуры задаются расстоянием до края, поэтому
// сглаживание и размытые края получаются одной формулой, без зависимостей.

import (
	"fmt"
	"math"
	"math/rand"
)

type RGB struct{ R, G, B float64 }

func hx(s string) RGB {
	var r, g, b int
	fmt.Sscanf(s, "#%02x%02x%02x", &r, &g, &b)
	return RGB{float64(r) / 255, float64(g) / 255, float64(b) / 255}
}

func (a RGB) Mix(b RGB, t float64) RGB {
	return RGB{a.R + (b.R-a.R)*t, a.G + (b.G-a.G)*t, a.B + (b.B-a.B)*t}
}
func (a RGB) Mul(k float64) RGB { return RGB{a.R * k, a.G * k, a.B * k} }
func (a RGB) Add(b RGB) RGB     { return RGB{a.R + b.R, a.G + b.G, a.B + b.B} }

// Jit — тот же цвет, чуть светлее или темнее и чуть сдвинутый по оттенку:
// сто одинаковых листьев выглядят как заглушка, сто разных — как куст.
func Jit(c RGB, r *rand.Rand, k float64) RGB {
	b := 1 + (r.Float64()*2-1)*k
	return RGB{
		c.R * b * (1 + (r.Float64()*2-1)*k*0.4),
		c.G * b * (1 + (r.Float64()*2-1)*k*0.4),
		c.B * b * (1 + (r.Float64()*2-1)*k*0.4),
	}
}

func clamp(v, a, b float64) float64 { return math.Max(a, math.Min(b, v)) }
func clampi(v, a, b int) int {
	if v < a {
		return a
	}
	if v > b {
		return b
	}
	return v
}
func smooth(t float64) float64 { t = clamp(t, 0, 1); return t * t * (3 - 2*t) }

type Canvas struct {
	W, H int
	P    []RGB
}

func NewCanvas(w, h int) *Canvas { return &Canvas{W: w, H: h, P: make([]RGB, w*h)} }

type Shape interface {
	Bounds() (x0, y0, x1, y1 float64)
	Dist(x, y float64) float64 // меньше нуля — внутри
}

type Paint func(x, y float64) RGB

func Solid(c RGB) Paint { return func(float64, float64) RGB { return c } }

// Fill закрашивает фигуру; feather — ширина края в пикселях (1 — чёткий).
func (c *Canvas) Fill(s Shape, p Paint, alpha, feather float64) {
	if feather < 1 {
		feather = 1
	}
	x0, y0, x1, y1 := s.Bounds()
	pad := feather + 1
	ix0 := clampi(int(math.Floor(x0-pad)), 0, c.W)
	iy0 := clampi(int(math.Floor(y0-pad)), 0, c.H)
	ix1 := clampi(int(math.Ceil(x1+pad)), 0, c.W)
	iy1 := clampi(int(math.Ceil(y1+pad)), 0, c.H)
	for y := iy0; y < iy1; y++ {
		py := float64(y) + .5
		row := y * c.W
		for x := ix0; x < ix1; x++ {
			px := float64(x) + .5
			d := s.Dist(px, py)
			cov := 0.5 - d/feather
			if cov <= 0 {
				continue
			}
			if cov > 1 {
				cov = 1
			}
			q := &c.P[row+x]
			*q = q.Mix(p(px, py), cov*alpha)
		}
	}
}

func (c *Canvas) Each(f func(x, y float64, p RGB) RGB) {
	for y := 0; y < c.H; y++ {
		for x := 0; x < c.W; x++ {
			i := y*c.W + x
			c.P[i] = f(float64(x)+.5, float64(y)+.5, c.P[i])
		}
	}
}

// Below закрашивает всё ниже кривой top(x): земля, холмы, вода, лес.
func (c *Canvas) Below(top func(x float64) float64, p Paint, alpha float64) {
	for x := 0; x < c.W; x++ {
		px := float64(x) + .5
		t := top(px)
		for y := clampi(int(math.Floor(t-1)), 0, c.H); y < c.H; y++ {
			py := float64(y) + .5
			cov := clamp(py-t+0.5, 0, 1)
			if cov <= 0 {
				continue
			}
			i := y*c.W + x
			c.P[i] = c.P[i].Mix(p(px, py), cov*alpha)
		}
	}
}

// Blur — трижды ящичное размытие, почти гауссово: фон «не в фокусе».
func (c *Canvas) Blur(r int) {
	if r < 1 {
		return
	}
	tmp := make([]RGB, len(c.P))
	for pass := 0; pass < 3; pass++ {
		boxBlur(c.P, tmp, c.W, c.H, r, 1, c.W)
		boxBlur(tmp, c.P, c.H, c.W, r, c.W, 1)
	}
}

// boxBlur размывает n строк длины w; step — шаг внутри строки, stride — между строками.
func boxBlur(src, dst []RGB, w, n, r, step, stride int) {
	inv := 1 / float64(2*r+1)
	for j := 0; j < n; j++ {
		base := j * stride
		at := func(i int) RGB { return src[base+clampi(i, 0, w-1)*step] }
		var acc RGB
		for k := -r; k <= r; k++ {
			acc = acc.Add(at(k))
		}
		for i := 0; i < w; i++ {
			dst[base+i*step] = acc.Mul(inv)
			a, b := at(i+r+1), at(i-r)
			acc = RGB{acc.R + a.R - b.R, acc.G + a.G - b.G, acc.B + a.B - b.B}
		}
	}
}

// ---- фигуры ----

type Circle struct{ X, Y, R float64 }

func (s Circle) Bounds() (float64, float64, float64, float64) {
	return s.X - s.R, s.Y - s.R, s.X + s.R, s.Y + s.R
}
func (s Circle) Dist(x, y float64) float64 { return math.Hypot(x-s.X, y-s.Y) - s.R }

type Ellipse struct{ X, Y, RX, RY, Rot float64 }

func (s Ellipse) Bounds() (float64, float64, float64, float64) {
	m := math.Max(s.RX, s.RY)
	return s.X - m, s.Y - m, s.X + m, s.Y + m
}
func (s Ellipse) Dist(x, y float64) float64 {
	dx, dy := x-s.X, y-s.Y
	cs, sn := math.Cos(s.Rot), math.Sin(s.Rot)
	lx, ly := dx*cs+dy*sn, -dx*sn+dy*cs
	k := math.Hypot(lx/s.RX, ly/s.RY)
	if k < 1e-9 {
		return -math.Min(s.RX, s.RY)
	}
	g := math.Hypot(lx/(s.RX*s.RX*k), ly/(s.RY*s.RY*k))
	return (k - 1) / g
}

// Capsule — отрезок с толщиной, которая может меняться от R1 к R2:
// стебли, ветки, морковь, кабачки.
type Capsule struct{ X1, Y1, X2, Y2, R1, R2 float64 }

func (s Capsule) Bounds() (float64, float64, float64, float64) {
	m := math.Max(s.R1, s.R2)
	return math.Min(s.X1, s.X2) - m, math.Min(s.Y1, s.Y2) - m, math.Max(s.X1, s.X2) + m, math.Max(s.Y1, s.Y2) + m
}
func (s Capsule) Dist(x, y float64) float64 {
	vx, vy := s.X2-s.X1, s.Y2-s.Y1
	l2 := vx*vx + vy*vy
	t := 0.0
	if l2 > 0 {
		t = clamp(((x-s.X1)*vx+(y-s.Y1)*vy)/l2, 0, 1)
	}
	return math.Hypot(x-(s.X1+vx*t), y-(s.Y1+vy*t)) - (s.R1 + (s.R2-s.R1)*t)
}

// Box — прямоугольник по центру, с поворотом и скруглением углов.
type Box struct{ X, Y, W, H, Rot, Rad float64 }

func (s Box) Bounds() (float64, float64, float64, float64) {
	m := math.Hypot(s.W, s.H) / 2
	return s.X - m, s.Y - m, s.X + m, s.Y + m
}
func (s Box) Dist(x, y float64) float64 {
	dx, dy := x-s.X, y-s.Y
	cs, sn := math.Cos(s.Rot), math.Sin(s.Rot)
	lx, ly := math.Abs(dx*cs+dy*sn), math.Abs(-dx*sn+dy*cs)
	qx, qy := lx-s.W/2+s.Rad, ly-s.H/2+s.Rad
	out := math.Hypot(math.Max(qx, 0), math.Max(qy, 0))
	in := math.Min(math.Max(qx, qy), 0)
	return out + in - s.Rad
}

type Poly struct {
	P              [][2]float64
	x0, y0, x1, y1 float64
}

func NewPoly(pts [][2]float64) *Poly {
	p := &Poly{P: pts, x0: math.Inf(1), y0: math.Inf(1), x1: math.Inf(-1), y1: math.Inf(-1)}
	for _, q := range pts {
		p.x0, p.y0 = math.Min(p.x0, q[0]), math.Min(p.y0, q[1])
		p.x1, p.y1 = math.Max(p.x1, q[0]), math.Max(p.y1, q[1])
	}
	return p
}
func (p *Poly) Bounds() (float64, float64, float64, float64) { return p.x0, p.y0, p.x1, p.y1 }
func (p *Poly) Dist(x, y float64) float64 {
	if x < p.x0-3 || x > p.x1+3 || y < p.y0-3 || y > p.y1+3 {
		return 4
	}
	d := math.Inf(1)
	in := false
	n := len(p.P)
	for i, j := 0, n-1; i < n; j, i = i, i+1 {
		a, b := p.P[j], p.P[i]
		d = math.Min(d, segDist(x, y, a, b))
		if (b[1] > y) != (a[1] > y) && x < (a[0]-b[0])*(y-b[1])/(a[1]-b[1])+b[0] {
			in = !in
		}
	}
	if in {
		return -d
	}
	return d
}

func segDist(x, y float64, a, b [2]float64) float64 {
	vx, vy := b[0]-a[0], b[1]-a[1]
	l2 := vx*vx + vy*vy
	t := 0.0
	if l2 > 0 {
		t = clamp(((x-a[0])*vx+(y-a[1])*vy)/l2, 0, 1)
	}
	return math.Hypot(x-(a[0]+vx*t), y-(a[1]+vy*t))
}

// Line — ломаная толщины W: проволока, рёбра теплицы, улыбка.
type Line struct {
	P [][2]float64
	W float64
	b [4]float64
}

func NewLine(w float64, pts ...[2]float64) *Line {
	l := &Line{P: pts, W: w}
	pp := NewPoly(pts)
	l.b = [4]float64{pp.x0 - w, pp.y0 - w, pp.x1 + w, pp.y1 + w}
	return l
}
func (l *Line) Bounds() (float64, float64, float64, float64) { return l.b[0], l.b[1], l.b[2], l.b[3] }
func (l *Line) Dist(x, y float64) float64 {
	d := math.Inf(1)
	for i := 1; i < len(l.P); i++ {
		d = math.Min(d, segDist(x, y, l.P[i-1], l.P[i]))
	}
	return d - l.W/2
}

// Ring — контур любой фигуры толщины w: оправа очков, обод бочки.
type Ring struct {
	S Shape
	W float64
}

func (r Ring) Bounds() (float64, float64, float64, float64) {
	x0, y0, x1, y1 := r.S.Bounds()
	return x0 - r.W, y0 - r.W, x1 + r.W, y1 + r.W
}
func (r Ring) Dist(x, y float64) float64 { return math.Abs(r.S.Dist(x, y)) - r.W/2 }

// Cut — фигура A без фигуры B.
type Cut struct{ A, B Shape }

func (c Cut) Bounds() (float64, float64, float64, float64) { return c.A.Bounds() }
func (c Cut) Dist(x, y float64) float64                    { return math.Max(c.A.Dist(x, y), -c.B.Dist(x, y)) }

// And — пересечение: то, что внутри обеих фигур.
type And struct{ A, B Shape }

func (c And) Bounds() (float64, float64, float64, float64) { return c.A.Bounds() }
func (c And) Dist(x, y float64) float64                    { return math.Max(c.A.Dist(x, y), c.B.Dist(x, y)) }

// Leaf — лист от черешка (x, y) в сторону ang.
func Leaf(x, y, length, width, ang float64) *Poly {
	const n = 14
	cs, sn := math.Cos(ang), math.Sin(ang)
	pts := make([][2]float64, 0, 2*n+2)
	put := func(t, h float64) {
		lx, ly := t*length, h
		pts = append(pts, [2]float64{x + lx*cs - ly*sn, y + lx*sn + ly*cs})
	}
	for i := 0; i <= n; i++ {
		t := float64(i) / n
		put(t, width/2*math.Pow(math.Sin(math.Pi*t), 0.85)*(1-0.35*t))
	}
	for i := n - 1; i > 0; i-- {
		t := float64(i) / n
		put(t, -width/2*math.Pow(math.Sin(math.Pi*t), 0.85)*(1-0.35*t))
	}
	return NewPoly(pts)
}

// Blob — неровный круг: картофелина, камень, куст, облако.
func Blob(r *rand.Rand, x, y, rx, ry, rough float64) *Poly {
	const n = 40
	ph := [3]float64{r.Float64() * 6, r.Float64() * 6, r.Float64() * 6}
	am := [3]float64{r.Float64(), r.Float64(), r.Float64()}
	pts := make([][2]float64, n)
	for i := range pts {
		a := 2 * math.Pi * float64(i) / n
		k := 1 + rough*(am[0]*math.Sin(2*a+ph[0])*0.5+am[1]*math.Sin(3*a+ph[1])*0.35+am[2]*math.Sin(5*a+ph[2])*0.2)
		pts[i] = [2]float64{x + math.Cos(a)*rx*k, y + math.Sin(a)*ry*k}
	}
	return NewPoly(pts)
}

// ---- заливки ----

func Lin(x0, y0, x1, y1 float64, a, b RGB) Paint {
	vx, vy := x1-x0, y1-y0
	l2 := vx*vx + vy*vy
	return func(x, y float64) RGB { return a.Mix(b, clamp(((x-x0)*vx+(y-y0)*vy)/l2, 0, 1)) }
}

var light = norm3(-0.45, -0.6, 0.66)

func norm3(x, y, z float64) [3]float64 {
	l := math.Sqrt(x*x + y*y + z*z)
	return [3]float64{x / l, y / l, z / l}
}

// Shade — объём шара или эллипсоида под светом сверху-слева, с бликом.
func Shade(cx, cy, rx, ry, rot float64, base RGB, spec float64) Paint {
	cs, sn := math.Cos(rot), math.Sin(rot)
	h := norm3(light[0], light[1], light[2]+1)
	return func(x, y float64) RGB {
		dx, dy := x-cx, y-cy
		lx, ly := (dx*cs+dy*sn)/rx, (-dx*sn+dy*cs)/ry
		nx, ny := lx*cs-ly*sn, lx*sn+ly*cs
		d2 := nx*nx + ny*ny
		if d2 > 1 {
			k := 1 / math.Sqrt(d2)
			nx, ny, d2 = nx*k, ny*k, 1
		}
		nz := math.Sqrt(1 - d2)
		dif := math.Max(0, nx*light[0]+ny*light[1]+nz*light[2])
		c := base.Mul(0.3 + 0.82*dif)
		s := math.Pow(math.Max(0, nx*h[0]+ny*h[1]+nz*h[2]), 36) * spec
		return c.Add(RGB{s, s, s})
	}
}

// ---- шум ----

func hash2(ix, iy, seed int) float64 {
	h := uint32(ix*374761393 + iy*668265263 + seed*1442695041)
	h = (h ^ (h >> 13)) * 1274126177
	h ^= h >> 16
	return float64(h&0xffffff) / float64(0xffffff)
}

func vnoise(x, y float64, seed int) float64 {
	ix, iy := math.Floor(x), math.Floor(y)
	fx, fy := smooth(x-ix), smooth(y-iy)
	i, j := int(ix), int(iy)
	a := hash2(i, j, seed)
	b := hash2(i+1, j, seed)
	c := hash2(i, j+1, seed)
	d := hash2(i+1, j+1, seed)
	return a + (b-a)*fx + (c-a)*fy + (a-b-c+d)*fx*fy
}

func fbm(x, y float64, oct, seed int) float64 {
	s, amp, norm := 0.0, 0.5, 0.0
	for o := 0; o < oct; o++ {
		s += vnoise(x, y, seed+o*17) * amp
		norm += amp
		amp *= 0.5
		x, y = x*2.03, y*2.03
	}
	return s / norm
}

// Tex — фактура: земля, дерево, мешковина, снег.
func Tex(a, b RGB, sx, sy float64, oct, seed int) Paint {
	return func(x, y float64) RGB { return a.Mix(b, smooth(fbm(x/sx, y/sy, oct, seed)*1.6-0.3)) }
}
