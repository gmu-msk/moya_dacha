package main

// Плоды, ягоды и цветы — детали, из которых собираются сцены.

import "math"

var (
	leafG  = []RGB{hx("#3f7a2a"), hx("#4f8f33"), hx("#2f6324"), hx("#5d9a3a")}
	leafY  = []RGB{hx("#6b9a35"), hx("#8aaa3d"), hx("#57832c")}
	leafDk = []RGB{hx("#24481b"), hx("#2f5a22"), hx("#1c3a16")}
)

func rot2(x, y, a float64) (float64, float64) {
	cs, sn := math.Cos(a), math.Sin(a)
	return x*cs - y*sn, x*sn + y*cs
}

func berryPoly(x, y, w, h, rot float64) *Poly {
	const n = 44
	pts := make([][2]float64, n)
	for i := range pts {
		a := 2 * math.Pi * float64(i) / n
		ly := -math.Cos(a)
		lx := math.Sin(a) * (1 - 0.55*smooth((ly+0.3)/1.3))
		if ly < 0 {
			ly *= 0.75
		}
		px, py := rot2(lx*w, ly*h, rot)
		pts[i] = [2]float64{x + px, y + py}
	}
	return NewPoly(pts)
}

// strawberry — ягода с семечками и чашелистиками; s — высота ягоды.
func (z *Z) strawberry(x, y, s, rot, ripe float64) {
	w, h := s*0.42, s*0.5
	body := berryPoly(x, y, w, h, rot)
	red := hx("#c8141f").Mix(hx("#e8e0a0"), 1-ripe)
	z.fill(body, Shade(x-w*0.1, y-h*0.15, w*1.3, h*1.25, rot, red, 0.45))
	for k := 0; k < 70; k++ {
		px, py := x+z.rf(-w, w), y+z.rf(-h, h)
		if body.Dist(px, py) > -s*0.03 {
			continue
		}
		z.fillA(Ellipse{px, py, s * 0.018, s * 0.026, rot}, Solid(red.Mul(0.55)), 0.7, 1.5)
		z.solid(Ellipse{px, py - s*0.006, s * 0.009, s * 0.014, rot}, hx("#f0d060").Mix(red, 0.2))
	}
	tx, ty := rot2(0, -h*0.72, rot)
	tx, ty = x+tx, y+ty
	for k := 0; k < 8; k++ {
		a := rot - math.Pi/2 + (float64(k)/7-0.5)*math.Pi*1.5 + z.rf(-0.15, 0.15)
		z.fill(Leaf(tx, ty, s*z.rf(0.28, 0.38), s*0.12, a+math.Pi), Solid(z.jit(hx("#3d7a26"), 0.15)))
	}
	sx, sy := rot2(0, -h*0.72-s*0.35, rot)
	z.fill(Capsule{tx, ty, x + sx, y + sy, s * 0.025, s * 0.018}, Solid(hx("#4a7a2c")))
}

// tomato — помидор с чашечкой; ribs — ребристый, как «Бычье сердце».
func (z *Z) tomato(x, y, r float64, col RGB, ribs bool) {
	z.sphere(x, y, r*1.08, r*0.92, 0, col, 0.55)
	if ribs {
		for k := 0; k < 6; k++ {
			a := float64(k) / 6 * 2 * math.Pi
			pts := [][2]float64{}
			for t := 0.15; t <= 1.0; t += 0.1 {
				rr := r * t
				pts = append(pts, [2]float64{x + math.Cos(a+t*0.3)*rr*1.05, y - r*0.55 + math.Sin(a+t*0.3)*rr*0.6 + t*t*r*0.35})
			}
			z.fillA(NewLine(r*0.05, pts...), Solid(col.Mul(0.6)), 0.35, r*0.06)
		}
	}
	for k := 0; k < 5; k++ {
		a := float64(k)/5*2*math.Pi + z.rf(-0.2, 0.2)
		z.fill(Leaf(x, y-r*0.8, r*z.rf(0.35, 0.5), r*0.13, a), Solid(z.jit(hx("#3b6e24"), 0.12)))
	}
	z.fill(Capsule{x, y - r*0.8, x + r*0.08, y - r*1.05, r * 0.06, r * 0.05}, Solid(hx("#4d7a2c")))
}

// cucumber — огурец с пупырышками.
func (z *Z) cucumber(x1, y1, x2, y2, r float64, col RGB) {
	z.fill(Capsule{x1, y1, x2, y2, r, r * 0.85}, func(x, y float64) RGB {
		vx, vy := x2-x1, y2-y1
		l := math.Hypot(vx, vy)
		d := ((x-x1)*(-vy) + (y-y1)*vx) / l / r // −1..1 поперёк
		stripe := 0.5 + 0.5*math.Sin(d*9+fbm(x/30, y/30, 2, 3)*2)
		c := col.Mix(col.Mul(1.45), 0.25*stripe*smooth(1-math.Abs(d)))
		return c.Mul(0.55 + 0.6*smooth(0.9-d*0.7))
	})
	l := math.Hypot(x2-x1, y2-y1)
	for k := 0; k < int(l/r*6); k++ {
		t := z.rf(0.05, 0.95)
		px, py := x1+(x2-x1)*t, y1+(y2-y1)*t
		o := z.rf(-0.8, 0.8) * r
		nx, ny := -(y2-y1)/l, (x2-x1)/l
		z.fillA(Circle{px + nx*o, py + ny*o, r * 0.07}, Solid(col.Mul(0.5)), 0.6, 1.5)
		z.fillA(Circle{px + nx*o - r*0.02, py + ny*o - r*0.02, r * 0.035}, Solid(col.Mul(1.6)), 0.5, 1)
	}
}

// apple — яблоко: основной цвет, румянец и хвостик.
func (z *Z) apple(x, y, r float64, base, blush RGB, k float64) {
	sd := z.seed + int(x*7+y)
	sh := Shade(x, y, r*1.05, r*0.95, 0, RGB{1, 1, 1}, 0)
	h := norm3(light[0], light[1], light[2]+1)
	_ = h
	z.fill(Ellipse{x, y, r * 1.05, r * 0.95, 0}, func(px, py float64) RGB {
		n := fbm(px/(r*0.35), py/(r*0.9), 3, sd)
		c := base.Mix(blush, clamp(k*(n*1.6-0.3)+k*0.3*(1-(py-y+r)/(2*r)), 0, 1))
		l := sh(px, py)
		return RGB{c.R * l.R, c.G * l.G, c.B * l.B}
	})
	z.fillA(Ellipse{x - r*0.35, y - r*0.4, r * 0.22, r * 0.14, -0.6}, Solid(RGB{1, 1, 1}), 0.35, r*0.15)
	z.fillA(Ellipse{x, y - r*0.8, r * 0.22, r * 0.1, 0}, Solid(base.Mul(0.45)), 0.6, r*0.1)
	z.fill(Capsule{x, y - r*0.8, x + r*0.12, y - r*1.15, r * 0.05, r * 0.035}, Solid(hx("#5a3a1e")))
}

// flower — цветок из n лепестков вокруг серединки.
func (z *Z) flower(x, y, r float64, n int, petal, center RGB, pw float64) {
	a0 := z.rf(0, math.Pi)
	for k := 0; k < n; k++ {
		a := a0 + float64(k)/float64(n)*2*math.Pi
		px, py := x+math.Cos(a)*r*0.55, y+math.Sin(a)*r*0.55
		c := z.jit(petal, 0.06)
		z.fill(Ellipse{px, py, r * 0.55, r * pw, a}, Lin(x, y, x+math.Cos(a)*r, y+math.Sin(a)*r, c.Mul(0.82), c))
	}
	z.sphere(x, y, r*0.28, r*0.28, 0, center, 0.2)
}

// bloom — пышный цветок кольцами лепестков: пион, роза, георгин.
func (z *Z) bloom(x, y, r float64, col RGB, rings int, pointy bool) {
	for ring := 0; ring < rings; ring++ {
		t := float64(ring) / float64(rings)
		rr := r * (1 - t*0.8)
		n := 9 + (rings-ring)*2
		a0 := z.rf(0, 2*math.Pi)
		c := col.Mul(0.72 + 0.35*t)
		for k := 0; k < n; k++ {
			a := a0 + float64(k)/float64(n)*2*math.Pi
			if pointy {
				z.fill(Leaf(x, y, rr, rr*0.32, a), Lin(x, y, x+math.Cos(a)*rr, y+math.Sin(a)*rr, c.Mul(0.75), z.jit(c, 0.05).Mul(1.1)))
			} else {
				px, py := x+math.Cos(a)*rr*0.5, y+math.Sin(a)*rr*0.5
				z.fill(Ellipse{px, py, rr * 0.52, rr * 0.4, a}, Lin(x, y, px+math.Cos(a)*rr*0.5, py+math.Sin(a)*rr*0.5, c.Mul(0.7), z.jit(c, 0.05).Mul(1.08)))
			}
		}
	}
	z.fillA(Circle{x, y, r * 0.12}, Solid(col.Mul(0.55)), 0.8, r*0.08)
}

// berries — гроздь круглых ягод: смородина, вишня, малина по отдельности.
func (z *Z) berries(x, y, spread, r float64, n int, col RGB, stems bool) {
	for k := 0; k < n; k++ {
		bx, by := x+z.rf(-1, 1)*spread, y+z.rf(0, 1)*spread*1.6
		if stems {
			z.fill(NewLine(r*0.12, [2]float64{x, y - spread*0.3}, [2]float64{(x + bx) / 2, by - r*1.5}, [2]float64{bx, by - r}), Solid(hx("#5b6a2a")))
		}
		z.sphere(bx, by, r, r, 0, z.jit(col, 0.1), 0.9)
	}
}

// raspberry — малина из костяночек.
func (z *Z) raspberry(x, y, s, rot float64, col RGB) {
	z.fill(berryPoly(x, y, s*0.4, s*0.45, rot), Solid(col.Mul(0.45)))
	for k := 0; k < 38; k++ {
		ly := z.rf(-0.8, 0.9)
		w := (1 - 0.45*smooth((ly+0.3)/1.3)) * math.Sqrt(math.Max(0, 1-ly*ly*0.6))
		lx := z.rf(-1, 1) * w * 0.85
		px, py := rot2(lx*s*0.38, ly*s*0.42, rot)
		rr := s * 0.075
		z.sphere(x+px, y+py, rr, rr, 0, z.jit(col, 0.08), 0.8)
	}
}

func (z *Z) potato(x, y, r float64, col RGB) {
	b := Blob(z.r, x, y, r*z.rf(1.1, 1.4), r*z.rf(0.8, 1), 0.25)
	sd := z.seed + int(x)
	sh := Shade(x, y, r*1.3, r, 0, RGB{1, 1, 1}, 0.05)
	z.fill(b, func(px, py float64) RGB {
		c := col.Mix(col.Mul(0.7), fbm(px/(r*0.3), py/(r*0.3), 3, sd))
		l := sh(px, py)
		return RGB{c.R * l.R, c.G * l.G, c.B * l.B}
	})
	for k := 0; k < 4; k++ {
		z.fillA(Circle{x + z.rf(-r, r), y + z.rf(-r*0.6, r*0.6), r * 0.05}, Solid(col.Mul(0.4)), 0.7, 2)
	}
	for k := 0; k < 3; k++ {
		z.fillA(Blob(z.r, x+z.rf(-r, r)*0.8, y+z.rf(-r, r)*0.6, r*0.3, r*0.2, 0.5), Solid(hx("#4a3825")), 0.35, r*0.1)
	}
}

// jar — стеклянная банка с содержимым и крышкой.
func (z *Z) jar(x, y, w, h float64, content, lid RGB, inside func(x0, y0, x1, y1 float64)) {
	z.shadow(x+w*0.1, y+h*0.5, w*0.6, h*0.06, 0.4)
	z.fill(Box{x, y, w, h, 0, w * 0.12}, Lin(x-w/2, 0, x+w/2, 0, content.Mul(0.75), content.Mul(0.9)))
	if inside != nil {
		inside(x-w/2, y-h/2+h*0.08, x+w/2, y+h/2)
	}
	z.fillA(Box{x - w*0.3, y, w * 0.08, h * 0.8, 0, w * 0.04}, Solid(RGB{1, 1, 1}), 0.35, w*0.05)
	z.fillA(Box{x + w*0.35, y, w * 0.04, h * 0.7, 0, w * 0.02}, Solid(RGB{1, 1, 1}), 0.2, w*0.03)
	z.fill(Box{x, y - h/2 - h*0.02, w * 0.86, h * 0.1, 0, w * 0.03}, Lin(x-w/2, 0, x+w/2, 0, lid.Mul(1.2), lid.Mul(0.7)))
	z.fillA(Box{x, y - h/2 - h*0.05, w * 0.8, h * 0.02, 0, 0}, Solid(RGB{1, 1, 1}), 0.4, 2)
}
