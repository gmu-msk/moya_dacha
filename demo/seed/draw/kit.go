package main

// Общие детали дачных снимков: небо, земля, трава, листва, доски, плоды.

import (
	"math"
	"math/rand"
)

type Z struct {
	c    *Canvas
	r    *rand.Rand
	W, H float64
	S    float64 // меньшая сторона: размеры в сценах считаются от неё
	seed int
}

func (z *Z) rf(a, b float64) float64  { return a + z.r.Float64()*(b-a) }
func (z *Z) pick(cs ...RGB) RGB       { return cs[z.r.Intn(len(cs))] }
func (z *Z) jit(c RGB, k float64) RGB { return Jit(c, z.r, k) }

func (z *Z) fill(s Shape, p Paint)                { z.c.Fill(s, p, 1, 1) }
func (z *Z) fillA(s Shape, p Paint, a, f float64) { z.c.Fill(s, p, a, f) }
func (z *Z) solid(s Shape, c RGB)                 { z.c.Fill(s, Solid(c), 1, 1) }

// vgrad — вертикальный переход через несколько цветов.
func (z *Z) vgrad(stops ...RGB) {
	n := float64(len(stops) - 1)
	z.c.Each(func(x, y float64, _ RGB) RGB {
		t := clamp(y/z.H, 0, 1) * n
		i := int(math.Min(t, n-1e-9))
		return stops[i].Mix(stops[i+1], t-float64(i))
	})
}

// glow — мягкое пятно света, прибавляется к картинке.
func (z *Z) glow(x, y, rad float64, col RGB, k float64) {
	z.c.Each(func(px, py float64, p RGB) RGB {
		d2 := ((px-x)*(px-x) + (py-y)*(py-y)) / (rad * rad)
		if d2 > 9 {
			return p
		}
		return p.Add(col.Mul(k * math.Exp(-d2)))
	})
}

func (z *Z) cloud(x, y, w float64, col RGB, a float64) {
	for i := 0; i < 9; i++ {
		cx := x + z.rf(-0.5, 0.5)*w
		cy := y + z.rf(-0.12, 0.08)*w
		r := z.rf(0.12, 0.26) * w
		z.fillA(Ellipse{cx, cy, r * 1.3, r * 0.75, 0}, Solid(col.Mix(col.Mul(0.86), clamp((cy-y)/w*4+0.5, 0, 1))), a, r*0.5)
	}
}

// treeline — тёмная кромка леса у горизонта.
func (z *Z) treeline(base, amp float64, col RGB, sd int) {
	z.c.Below(func(x float64) float64 {
		n := fbm(x/(z.S*0.05), 0, 4, sd)
		spikes := math.Pow(math.Abs(math.Sin(x/(z.S*0.011)+fbm(x/(z.S*0.02), 1, 2, sd+5)*6)), 4) * 0.2
		return base - amp*(n*1.2+spikes)
	}, func(x, y float64) RGB { return col.Mix(col.Mul(1.25), fbm(x/40, y/40, 2, sd)) }, 1)
}

// ground — земля ниже кривой, с фактурой.
func (z *Z) ground(top func(float64) float64, a, b RGB, scale float64) {
	sd := z.seed
	z.c.Below(top, Tex(a, b, scale, scale*0.6, 4, sd), 1)
}

// grass — отдельные травинки, от дальних к ближним.
func (z *Z) grass(y0, y1 float64, n int, h0, h1 float64, pal ...RGB) {
	for i := 0; i < n; i++ {
		t := float64(i) / float64(n)
		y := y0 + (y1-y0)*t + z.rf(-0.02, 0.02)*z.S
		x := z.rf(-0.02, 1.02) * z.W
		h := (h0 + (h1-h0)*t) * z.rf(0.6, 1.3)
		ang := -math.Pi/2 + z.rf(-0.35, 0.35)
		c := z.jit(pal[z.r.Intn(len(pal))], 0.18).Mul(0.75 + 0.35*t)
		w := h * z.rf(0.025, 0.05)
		z.fill(Capsule{x, y, x + math.Cos(ang)*h, y + math.Sin(ang)*h, w, w * 0.2}, Solid(c))
	}
}

// leaves — россыпь листьев внутри эллипса: куст, крона, ботва.
func (z *Z) leaves(cx, cy, rx, ry float64, n int, size float64, pal ...RGB) {
	for i := 0; i < n; i++ {
		a := z.rf(0, 2*math.Pi)
		d := math.Sqrt(z.r.Float64())
		x, y := cx+math.Cos(a)*rx*d, cy+math.Sin(a)*ry*d
		l := size * z.rf(0.7, 1.3)
		ang := z.rf(0, 2*math.Pi)
		base := pal[z.r.Intn(len(pal))]
		// Нижние и дальние листья в тени.
		shade := 0.72 + 0.4*clamp(0.5-(y-cy)/(2*ry), 0, 1)
		c := z.jit(base, 0.15).Mul(shade)
		lf := Leaf(x, y, l, l*z.rf(0.4, 0.6), ang)
		z.fill(lf, Lin(x, y, x+math.Cos(ang)*l, y+math.Sin(ang)*l, c.Mul(0.8), c.Mul(1.12)))
		z.fillA(Capsule{x, y, x + math.Cos(ang)*l*0.85, y + math.Sin(ang)*l*0.85, l * 0.012, l * 0.004}, Solid(c.Mul(1.3)), 0.6, 1)
	}
}

// bokeh — фон не в фокусе: пятна зелени и бликов, потом сильное размытие.
func (z *Z) bokeh(pal []RGB, lights RGB, n int, blur float64) {
	z.vgrad(pal[0].Mul(1.1), pal[0].Mul(0.7))
	for i := 0; i < n; i++ {
		r := z.rf(0.05, 0.22) * z.S
		z.fillA(Blob(z.r, z.rf(0, 1)*z.W, z.rf(0, 1)*z.H, r, r*z.rf(0.6, 1.2), 0.4), Solid(z.jit(pal[z.r.Intn(len(pal))], 0.2)), z.rf(0.5, 1), 1)
	}
	z.c.Blur(int(blur * z.S))
	for i := 0; i < n/3; i++ {
		r := z.rf(0.015, 0.045) * z.S
		z.fillA(Circle{z.rf(0, 1) * z.W, z.rf(0, 0.7) * z.H, r}, Solid(lights), z.rf(0.08, 0.3), r*0.35)
	}
}

// planks — доски стола или крыльца.
func (z *Z) planks(y0 float64, col RGB, n int) {
	h := (z.H - y0) / float64(n)
	for i := 0; i < n; i++ {
		top := y0 + float64(i)*h
		c := z.jit(col, 0.12).Mul(0.85 + 0.25*float64(i)/float64(n))
		sd := z.seed + i*31
		z.fill(Box{z.W / 2, top + h/2, z.W * 1.1, h - 2, 0, 0}, func(x, y float64) RGB {
			g := fbm(x/(z.S*0.25), y/(z.S*0.006), 3, sd)
			return c.Mix(c.Mul(0.72), smooth(g*1.8-0.5))
		})
		z.fillA(Box{z.W / 2, top + 1, z.W * 1.1, 3, 0, 0}, Solid(col.Mul(0.35)), 0.8, 2)
	}
}

// vplanks — вертикальные доски: забор, стена сарая.
func (z *Z) vplanks(x0, x1, y0, y1 float64, col RGB, w float64) {
	for x := x0; x < x1; x += w {
		c := z.jit(col, 0.1)
		sd := z.seed + int(x)
		z.fill(Box{x + w/2, (y0 + y1) / 2, w - 2, y1 - y0, 0, 0}, func(px, py float64) RGB {
			g := fbm(px/(w*0.2), py/(z.S*0.2), 3, sd)
			return c.Mix(c.Mul(0.75), smooth(g*1.8-0.5))
		})
	}
}

// fence — штакетник на линии y высотой h.
func (z *Z) fence(y, h float64, col RGB, step float64) {
	z.solid(Box{z.W / 2, y - h*0.72, z.W * 1.2, h * 0.07, 0, 0}, col.Mul(0.7))
	z.solid(Box{z.W / 2, y - h*0.25, z.W * 1.2, h * 0.07, 0, 0}, col.Mul(0.7))
	for x := z.rf(0, step); x < z.W+step; x += step {
		w := step * 0.62
		c := z.jit(col, 0.08)
		z.fill(NewPoly([][2]float64{{x - w/2, y}, {x - w/2, y - h}, {x, y - h - w*0.5}, {x + w/2, y - h}, {x + w/2, y}}),
			Lin(x-w/2, 0, x+w/2, 0, c, c.Mul(0.8)))
	}
}

func (z *Z) sphere(x, y, rx, ry, rot float64, col RGB, spec float64) {
	z.fill(Ellipse{x, y, rx, ry, rot}, Shade(x, y, rx, ry, rot, col, spec))
}

// shadow — мягкая тень под предметом.
func (z *Z) shadow(x, y, rx, ry, a float64) {
	z.fillA(Ellipse{x, y, rx, ry, 0}, Solid(RGB{0.05, 0.04, 0.03}), a, ry*0.8)
}

// finish — то, что делает картинку похожей на снимок: тон и виньетка.
// Зерна нет намеренно: шум JPEG не сжимает, а вес репозитория дороже.
func (z *Z) finish(warm, vig float64) {
	cx, cy := z.W/2, z.H/2
	rr := math.Hypot(cx, cy)
	z.c.Each(func(x, y float64, p RGB) RGB {
		d := math.Hypot(x-cx, y-cy) / rr
		k := 1 - vig*smooth((d-0.45)/0.6)
		p = RGB{p.R * (1 + warm*0.06), p.G * (1 + warm*0.015), p.B * (1 - warm*0.06)}.Mul(k)
		// Лёгкая S-кривая контраста.
		p = RGB{scurve(p.R), scurve(p.G), scurve(p.B)}
		return p
	})
}

func scurve(v float64) float64 {
	v = clamp(v, 0, 1)
	return v + 0.18*(v-0.5)*(1-math.Abs(2*v-1))
}
