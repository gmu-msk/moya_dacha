package main

// Общие планы: участок, улица СНТ, теплица, баня, ульи, зима.

import "math"

// persp — простая перспектива: точка на земле в X метрах вбок и D метрах
// вперёд; D = 1 приходится на нижний край кадра.
type persp struct{ vpx, hor, f, ch float64 }

func (z *Z) persp(hor, ch float64) persp { return persp{z.W / 2, hor, (z.H - hor) / ch, ch} }
func (p persp) pt(X, D, Y float64) (float64, float64) {
	return p.vpx + X*p.f/D, p.hor + (p.ch-Y)*p.f/D
}
func (p persp) scale(D float64) float64 { return p.f / D }

func (z *Z) sky(hor float64, top, bot RGB, clouds int, cloud RGB) {
	z.c.Each(func(x, y float64, _ RGB) RGB { return top.Mix(bot, clamp(y/hor, 0, 1)) })
	for i := 0; i < clouds; i++ {
		z.cloud(z.rf(0, 1)*z.W, z.rf(0.05, 0.7)*hor, z.S*z.rf(0.25, 0.5), cloud, 0.8)
	}
}

// crown — крона дерева: несколько объёмных пятен и листва поверх.
func (z *Z) crown(x, y, r float64, pal []RGB, detail bool) {
	for k := 0; k < 7; k++ {
		a := z.rf(0, 2*math.Pi)
		px, py := x+math.Cos(a)*r*0.45, y+math.Sin(a)*r*0.35
		rr := r * z.rf(0.45, 0.65)
		z.fill(Blob(z.r, px, py, rr, rr*0.9, 0.25), Shade(x, y, r*1.2, r*1.1, 0, z.jit(pal[k%len(pal)], 0.08), 0.05))
	}
	if detail {
		z.leaves(x, y, r*0.9, r*0.8, int(r/3), r*0.18, pal...)
	}
}

func greenhouse(z *Z, v int) {
	top, bot, gr := hx("#6aa0d8"), hx("#d8e8f0"), leafG
	plants := leafG
	if v == 0 {
		top, bot, gr = hx("#8a96a4"), hx("#d8d8d0"), []RGB{hx("#8a8a40"), hx("#a09048"), hx("#6a7a3a")}
		plants = []RGB{hx("#7a6a38"), hx("#5a6a30"), hx("#8a7a40")}
	}
	hor := z.H * 0.45
	z.sky(hor, top, bot, 4, RGB{1, 1, 1})
	z.treeline(hor, z.S*0.12, gr[0].Mul(0.55), z.seed)
	z.ground(func(float64) float64 { return hor }, gr[0], gr[2], z.S*0.05)
	cx, base := z.W*0.42, z.H*0.85
	w, h := z.S*0.8, z.S*0.5
	arch := func(ox, oy, sc float64) [][2]float64 {
		pts := [][2]float64{}
		for i := 0; i <= 24; i++ {
			a := math.Pi * float64(i) / 24
			pts = append(pts, [2]float64{ox - math.Cos(a)*w/2*sc, oy - h*0.3*sc - math.Sin(a)*h*0.7*sc})
		}
		return pts
	}
	withBase := func(pts [][2]float64, ox, oy, sc float64) [][2]float64 {
		out := [][2]float64{{ox - w/2*sc, oy}}
		out = append(out, pts...)
		return append(out, [2]float64{ox + w/2*sc, oy})
	}
	bx, by, bs := cx+w*0.42, base-h*0.14, 0.82
	fa, ba := arch(cx, base, 1), arch(bx, by, bs)
	z.leaves(cx+w*0.2, base-h*0.3, w*0.5, h*0.3, 160, z.S*0.06, plants...)
	glass := hx("#eef4f6")
	for i := 1; i < len(fa); i++ {
		if v == 0 && i > 14 && i < 20 {
			continue // плёнку уже начали снимать
		}
		q := NewPoly([][2]float64{fa[i-1], fa[i], ba[i], ba[i-1]})
		z.fillA(q, Solid(glass.Mul(0.9-0.2*float64(i)/24)), 0.5, 1.5)
	}
	z.fillA(NewPoly([][2]float64{{cx + w/2, base}, fa[24], ba[24], {bx + w/2*bs, by}}), Solid(glass.Mul(0.8)), 0.5, 1.5)
	z.fillA(NewPoly(withBase(fa, cx, base, 1)), Solid(glass), 0.4, 1.5)
	rib := hx("#f8f8f8")
	z.fill(NewLine(z.S*0.01, withBase(fa, cx, base, 1)...), Solid(rib))
	z.fill(NewLine(z.S*0.008, withBase(ba, bx, by, bs)...), Solid(rib))
	for i := 0; i <= 24; i += 4 {
		z.fill(NewLine(z.S*0.006, fa[i], ba[i]), Solid(rib))
	}
	z.fill(Ring{Box{cx, base - h*0.33, w * 0.28, h * 0.66, 0, 0}, z.S * 0.01}, Solid(rib))
	if v == 0 {
		z.fillA(Blob(z.r, (fa[17][0]+ba[17][0])/2, (fa[17][1]+ba[17][1])/2+h*0.3, w*0.12, h*0.35, 0.4), Solid(glass), 0.55, 4)
	}
	z.grass(base-z.S*0.05, z.H*1.03, 1800, z.S*0.03, z.S*0.08, gr...)
	z.finish(0.15, 0.3)
}

func compost(z *Z, v int) {
	hor := z.H * 0.4
	z.sky(hor, hx("#7aa8d8"), hx("#e0e8ec"), 3, RGB{1, 1, 1})
	z.treeline(hor, z.S*0.1, hx("#3a5a2e"), z.seed)
	z.vplanks(0, z.W, z.H*0.28, z.H*0.62, hx("#8a8a84"), z.S*0.09)
	z.ground(func(float64) float64 { return z.H * 0.6 }, hx("#5a8a30"), hx("#3a6a22"), z.S*0.05)
	x0, x1, base, hh := z.W*0.1, z.W*0.9, z.H*0.9, z.S*0.45
	pts := [][2]float64{}
	for i := 0; i <= 60; i++ {
		t := float64(i) / 60
		y := base - hh*math.Pow(math.Sin(math.Pi*t), 0.7)*(0.9+0.25*fbm(t*6, 0, 3, z.seed))
		pts = append(pts, [2]float64{x0 + (x1-x0)*t, y})
	}
	mound := NewPoly(pts)
	sh := Shade(z.W/2, base-hh*0.3, (x1-x0)*0.55, hh*1.1, 0, RGB{1, 1, 1}, 0)
	sd := z.seed
	z.fill(mound, func(x, y float64) RGB {
		c := hx("#3a2818").Mix(hx("#1e140c"), fbm(x/14, y/14, 4, sd))
		l := sh(x, y)
		return RGB{c.R * l.R, c.G * l.G, c.B * l.B}
	})
	for k := 0; k < 500; k++ {
		x, y := z.rf(x0, x1), z.rf(base-hh, base)
		if mound.Dist(x, y) > -3 {
			continue
		}
		a := z.rf(0, math.Pi)
		z.fill(Capsule{x, y, x + math.Cos(a)*z.S*0.02, y + math.Sin(a)*z.S*0.02, 1.2, 1}, Solid(z.jit(hx("#b09a60"), 0.2)))
	}
	z.fill(Capsule{z.W * 0.7, z.H * 0.62, z.W * 0.82, z.H * 0.2, z.S * 0.012, z.S * 0.012}, Solid(hx("#b08a58")))
	z.grass(z.H*0.85, z.H*1.03, 1200, z.S*0.03, z.S*0.08, leafG...)
	z.finish(0.15, 0.3)
}

func street(z *Z, v int) {
	hor := z.H * 0.45
	top, bot := hx("#5a90d0"), hx("#d8e8f0")
	crowns := leafG
	road := hx("#9a8a70")
	switch v {
	case 0:
		top, bot = hx("#8aa0b8"), hx("#e0e0d8")
		crowns = []RGB{hx("#d8a030"), hx("#c07028"), hx("#a8a040"), hx("#6a8a3a")}
	case 2:
		top, bot = hx("#98a0a8"), hx("#d8dcdc")
	case 3:
		road = hx("#b8b4ac")
	}
	z.sky(hor, top, bot, 5, RGB{1, 1, 1})
	p := z.persp(hor, 1.6)
	p.vpx = z.W * 0.52
	z.treeline(hor, z.S*0.05, crowns[0].Mul(0.5), z.seed)
	z.ground(func(float64) float64 { return hor }, hx("#6a9a3a"), hx("#4a7a2a"), z.S*0.04)
	lx0, ly0 := p.pt(-2.5, 1, 0)
	rx0, ry0 := p.pt(2.5, 1, 0)
	_, _ = ly0, ry0
	z.fill(NewPoly([][2]float64{{p.vpx - 1, hor}, {p.vpx + 1, hor}, {rx0, z.H}, {lx0, z.H}}), func(x, y float64) RGB {
		s := (y - hor) / (z.H - hor)
		return road.Mix(road.Mul(0.7), fbm(x/(3+s*20), y/(2+s*15), 3, 7))
	})
	if v == 1 {
		dx0, _ := p.pt(-3.8, 1, 0)
		z.fill(NewPoly([][2]float64{{p.vpx - 2, hor}, {p.vpx - 1, hor}, {lx0 - z.S*0.05, z.H}, {dx0, z.H}}), Lin(0, hor, 0, z.H, hx("#5a6a60"), hx("#2a3a30")))
	}
	for D := 60.0; D >= 1.2; D *= 0.93 {
		for _, side := range []float64{-1, 1} {
			tx, ty := p.pt(side*z.rf(6, 9), D+z.rf(0, 2), 0)
			s := p.scale(D)
			if int(D*10)%3 == 0 {
				z.fill(Capsule{tx, ty, tx, ty - s*3, s * 0.12, s * 0.08}, Solid(hx("#4a3a2a")))
				z.crown(tx, ty-s*4, s*2.2, crowns, false)
			}
			fx, fy := p.pt(side*4.2, D, 0)
			fh := s * 1.5
			fw := s * 0.1
			z.fill(Box{fx, fy - fh/2, fw, fh, 0, 0}, Solid(z.jit(hx("#a89878"), 0.1).Mul(0.8+0.2*side)))
			if side > 0 && int(D*10)%11 == 0 {
				z.fill(Capsule{fx + s*0.8, fy, fx + s*0.8, fy - s*8, s * 0.1, s * 0.08}, Solid(hx("#5a4a3a")))
			}
		}
	}
	for _, side := range []float64{-1, 1} {
		a0x, a0y := p.pt(side*4.2, 60, 1.1)
		a1x, a1y := p.pt(side*4.2, 1.2, 1.1)
		z.fill(NewLine(3, [2]float64{a0x, a0y}, [2]float64{a1x, a1y}), Solid(hx("#8a7a5a")))
	}
	if v == 2 {
		bx, by := z.W*0.18, z.H*0.72
		for _, dx := range []float64{-0.1, 0.1} {
			z.solid(Box{bx + dx*z.S, by, z.S * 0.02, z.S * 0.5, 0, 0}, hx("#5a4a38"))
		}
		z.solid(Box{bx, by - z.S*0.1, z.S * 0.3, z.S * 0.2, 0, 0}, hx("#6a8a9a"))
		z.solid(Box{bx, by - z.S*0.22, z.S * 0.34, z.S * 0.03, 0, 0}, hx("#4a5a60"))
		for k := 0; k < 4; k++ {
			z.solid(Box{bx + z.rf(-0.1, 0.1)*z.S, by - z.S*0.1 + z.rf(-0.05, 0.05)*z.S, z.S * 0.06, z.S * 0.08, z.rf(-0.1, 0.1), 0}, hx("#f4f2ec"))
		}
	}
	z.grass(z.H*0.85, z.H*1.03, 600, z.S*0.03, z.S*0.09, leafG...)
	z.finish(0.2, 0.3)
}

func (z *Z) hive(x, base, w float64, col, roof RGB) {
	h := w * 0.95
	z.shadow(x+w*0.1, base, w*0.7, w*0.08, 0.5)
	for _, dx := range []float64{-0.35, 0.35} {
		z.solid(Box{x + dx*w, base - w*0.1, w * 0.08, w * 0.2, 0, 0}, hx("#3a2e24"))
	}
	body := func(y, hh float64, c RGB) {
		z.fill(Box{x, y, w, hh, 0, 0}, func(px, py float64) RGB {
			k := math.Mod((py-y+hh/2)/(hh/4), 1)
			cc := c.Mul(0.9 + 0.12*smooth(k*4))
			return cc.Mul(0.85 + 0.2*clamp((x+w/2-px)/w, 0, 1))
		})
	}
	body(base-w*0.2-h*0.35, h*0.7, col)
	body(base-w*0.2-h*0.85, h*0.3, col.Mix(RGB{1, 1, 1}, 0.15))
	z.solid(Box{x, base - w*0.2 - h - w*0.03, w * 1.15, w * 0.1, 0, 2}, roof)
	z.solid(Box{x, base - w*0.28, w * 0.35, w * 0.04, 0, 0}, hx("#1a1410"))
	z.solid(Box{x, base - w*0.23, w * 0.45, w * 0.03, 0, 0}, col.Mul(0.7))
}

func hives(z *Z, v int) {
	switch v {
	case 0, 1:
		hor := z.H * 0.45
		top, bot, gr := hx("#5a98d8"), hx("#d8e8f0"), leafG
		cr := leafG
		if v == 1 {
			top, bot, gr = hx("#90989c"), hx("#d0d0c8"), []RGB{hx("#7a7a3a"), hx("#8a7a40"), hx("#5a6a30")}
			cr = []RGB{hx("#d8a030"), hx("#c08030"), hx("#8a8a3a")}
		}
		z.sky(hor, top, bot, 4, RGB{1, 1, 1})
		z.treeline(hor, z.S*0.06, gr[0].Mul(0.5), z.seed)
		for k := 0; k < 5; k++ {
			x := z.W * (0.1 + 0.2*float64(k))
			z.crown(x, hor-z.S*0.1, z.S*z.rf(0.12, 0.18), cr, false)
		}
		z.ground(func(float64) float64 { return hor }, gr[0], gr[2], z.S*0.05)
		cols := []RGB{hx("#3a7ac0"), hx("#e8c030"), hx("#d85a30"), hx("#5aa050"), hx("#f0f0e8")}
		for k := 0; k < 5; k++ {
			x := z.W * (0.12 + 0.19*float64(k))
			base := z.H * (0.72 + 0.03*float64(k%2))
			col := cols[k]
			if v == 1 {
				col = z.jit(hx("#2a2a2c"), 0.1)
			}
			z.hive(x, base, z.S*0.22, col, hx("#9aa0a4"))
			if v == 1 {
				z.fill(Box{x, base - z.S*0.28, z.S * 0.24, z.S * 0.06, 0, 3}, Tex(hx("#d8b860"), hx("#a88a40"), 6, 6, 2, k))
			}
		}
		if v == 1 {
			for k := 0; k < 150; k++ {
				z.fill(Leaf(z.rf(0, z.W), z.rf(z.H*0.75, z.H), z.S*0.03, z.S*0.015, z.rf(0, 6.3)), Solid(z.jit(hx("#d8a020"), 0.2)))
			}
		} else {
			for k := 0; k < 80; k++ {
				z.solid(Circle{z.rf(0, z.W), z.rf(z.H*0.78, z.H), z.S * 0.006}, z.pick(hx("#ffffff"), hx("#f4e040"), hx("#c890e0")))
			}
		}
		z.grass(z.H*0.8, z.H*1.03, 900, z.S*0.03, z.S*0.08, gr...)
	case 2:
		z.bokeh(leafG, hx("#fffbe0"), 35, 0.04)
		z.grass(z.H*0.6, z.H*1.03, 1500, z.S*0.04, z.S*0.1, leafG...)
		z.hive(z.W*0.5, z.H*0.88, z.S*0.6, hx("#f0c420"), hx("#a8acb0"))
		for k := 0; k < 25; k++ {
			x, y := z.rf(0.2, 0.8)*z.W, z.rf(0.3, 0.9)*z.H
			z.fillA(Ellipse{x, y - z.S*0.008, z.S * 0.008, z.S * 0.005, -0.4}, Solid(RGB{1, 1, 1}), 0.6, 1)
			z.solid(Ellipse{x, y, z.S * 0.009, z.S * 0.005, z.rf(-0.5, 0.5)}, hx("#3a2a10"))
		}
	}
	z.finish(0.2, 0.3)
}

func banya(z *Z, v int) {
	logC := hx("#c89a58")
	logTex := func(y0 float64, sd int) Paint {
		return func(x, y float64) RGB {
			return logC.Mix(logC.Mul(0.72), fbm(x/(z.S*0.15), y/(z.S*0.008), 3, sd))
		}
	}
	end := func(x, y, r float64) {
		z.fill(Circle{x, y, r}, Solid(hx("#d8b070")))
		for k := 1; k < 5; k++ {
			z.fillA(Ring{Circle{x + r*0.05, y, r * float64(k) / 5}, 1.5}, Solid(hx("#9a7040")), 0.7, 1)
		}
	}
	switch v {
	case 0:
		hor := z.H * 0.5
		z.sky(hor, hx("#5a98d8"), hx("#d8e8f0"), 4, RGB{1, 1, 1})
		z.treeline(hor, z.S*0.12, hx("#3a5a2a"), z.seed)
		z.ground(func(float64) float64 { return hor }, hx("#6a9a3a"), hx("#4a7a2a"), z.S*0.05)
		x0, x1, base := z.W*0.2, z.W*0.68, z.H*0.85
		lh := z.S * 0.055
		for i := 0; i < 11; i++ {
			y := base - float64(i)*lh*0.9
			// Боковая стена уходит вглубь.
			z.fill(NewPoly([][2]float64{{x1, y - lh/2}, {x1 + z.S*0.25, y - lh/2 - z.S*0.08}, {x1 + z.S*0.25, y + lh*0.35 - z.S*0.08}, {x1, y + lh/2}}), Solid(logC.Mul(0.6)))
			z.fill(Capsule{x0, y, x1, y, lh / 2, lh / 2}, logTex(y, i))
			z.fill(Capsule{x0, y - lh*0.45, x1, y - lh*0.45, 1.5, 1.5}, Solid(hx("#6a7a3a")))
			end(x0-lh*0.3, y, lh*0.55)
			end(x1+lh*0.3, y, lh*0.55)
		}
		z.fill(Box{(x0 + x1) / 2, base - lh*3.5, z.S * 0.14, lh * 6, 0, 0}, Solid(hx("#2a2018")))
		z.fill(Box{x0 + (x1-x0)*0.8, base - lh*6, z.S * 0.1, lh * 2.5, 0, 0}, Solid(hx("#2a2018")))
		for r := 0; r < 3; r++ {
			for k := 0; k < 4-r; k++ {
				end(z.W*0.82+float64(k)*z.S*0.07+float64(r)*z.S*0.035, z.H*0.93-float64(r)*z.S*0.06, z.S*0.035)
			}
		}
		z.grass(z.H*0.85, z.H*1.03, 900, z.S*0.03, z.S*0.08, leafG...)
	case 1:
		z.bokeh([]RGB{hx("#7aa0c8"), hx("#a8c0d8"), hx("#5a7a4a")}, hx("#ffffff"), 20, 0.05)
		lh := z.S * 0.16
		for i := 0; i < 8; i++ {
			y := z.H - float64(i)*lh*0.95
			x1 := z.W * 0.55
			if i%2 == 1 {
				x1 = z.W * 0.62
			}
			z.fill(Capsule{-50, y, x1, y, lh / 2, lh / 2}, logTex(y, i))
			z.fill(Capsule{-50, y - lh*0.47, x1, y - lh*0.47, 3, 3}, Solid(hx("#5a7a30")))
			end(x1+lh*0.1, y, lh*0.55)
		}
	}
	z.finish(0.3, 0.3)
}

func shed(z *Z) {
	hor := z.H * 0.5
	z.sky(hor, hx("#5a98d8"), hx("#e0ecf0"), 4, RGB{1, 1, 1})
	z.treeline(hor, z.S*0.1, hx("#3a5a2a"), z.seed)
	z.ground(func(float64) float64 { return hor }, hx("#7a9a3a"), hx("#5a7a2a"), z.S*0.05)
	sx, base, w, h := z.W*0.55, z.H*0.72, z.S*0.55, z.S*0.4
	z.vplanks(sx-w/2, sx+w/2, base-h, base, hx("#8a8a84"), z.S*0.05)
	z.fill(NewPoly([][2]float64{{sx - w*0.6, base - h + z.S*0.02}, {sx + w*0.6, base - h - z.S*0.1}, {sx + w*0.6, base - h - z.S*0.06}, {sx - w*0.6, base - h + z.S*0.06}}), Tex(hx("#9a5a30"), hx("#5a3a24"), 20, 20, 3, 3))
	z.solid(Box{sx + w*0.15, base - h*0.4, w * 0.25, h * 0.75, 0.03, 0}, hx("#5a5a54"))
	z.fill(Capsule{z.W * 0.15, z.H, z.W * 0.16, z.H * 0.1, z.S * 0.02, z.S * 0.012}, Solid(hx("#ecebe4")))
	z.leaves(z.W*0.15, z.H*0.2, z.S*0.2, z.S*0.2, 140, z.S*0.05, leafY...)
	z.grass(z.H*0.55, z.H*1.03, 3500, z.S*0.08, z.S*0.25, append(leafY, hx("#a09a50"))...)
	for k := 0; k < 8; k++ {
		x, y := z.rf(0, z.W), z.rf(0.7, 1)*z.H
		z.fill(Leaf(x, y, z.S*0.2, z.S*0.16, -math.Pi/2+z.rf(-1, 1)), Solid(z.jit(hx("#3a6a2a"), 0.12)))
	}
	for k := 0; k < 12; k++ {
		x, y := z.rf(0, z.W), z.rf(0.55, 0.75)*z.H
		z.fill(Capsule{x, y, x, z.H, 2, 2}, Solid(hx("#6a7a3a")))
		for f := 0; f < 8; f++ {
			z.solid(Circle{x + z.rf(-0.02, 0.02)*z.S, y + z.rf(-0.02, 0.01)*z.S, z.S * 0.007}, hx("#e8c020"))
		}
	}
	z.finish(0.2, 0.3)
}

func beds(z *Z, v int) {
	switch v {
	case 0: // первая грядка
		z.ground(func(float64) float64 { return 0 }, hx("#5a8a30"), hx("#3a6a22"), z.S*0.05)
		z.grass(0, z.H*1.03, 3000, z.S*0.02, z.S*0.05, leafG...)
		p := z.persp(-z.H*0.3, 1.6)
		c := [4][2]float64{}
		c[0][0], c[0][1] = p.pt(-0.6, 1.3, 0)
		c[1][0], c[1][1] = p.pt(0.6, 1.3, 0)
		c[2][0], c[2][1] = p.pt(0.6, 4, 0)
		c[3][0], c[3][1] = p.pt(-0.6, 4, 0)
		z.fill(NewPoly([][2]float64{{c[0][0], c[0][1]}, {c[1][0], c[1][1]}, {c[2][0], c[2][1]}, {c[3][0], c[3][1]}}), Tex(hx("#4a3424"), hx("#2e2016"), z.S*0.04, z.S*0.03, 4, 3))
		for i := 0; i < 4; i++ {
			a, b := c[i], c[(i+1)%4]
			z.fill(NewLine(z.S*0.025, a, b), Solid(hx("#a88458")))
		}
		for _, X := range []float64{-0.3, 0, 0.3} {
			for D := 1.4; D < 3.9; D += 0.15 {
				x, y := p.pt(X, D, 0)
				s := p.scale(D) * 0.05
				z.fill(Leaf(x, y, s, s*0.5, -0.5), Solid(hx("#7ab840")))
				z.fill(Leaf(x, y, s, s*0.5, math.Pi+0.5), Solid(hx("#6aa838")))
			}
		}
		cx, cy := z.W*0.85, z.H*0.8
		z.fill(Ellipse{cx, cy, z.S * 0.1, z.S * 0.08, 0}, Shade(cx, cy, z.S*0.1, z.S*0.08, 0, hx("#3a8a5a"), 0.4))
		z.fill(Capsule{cx - z.S*0.08, cy, cx - z.S*0.2, cy - z.S*0.08, z.S * 0.012, z.S * 0.008}, Solid(hx("#2e7a4a")))
	case 1, 2:
		hor := z.H * 0.3
		z.sky(hor, hx("#6aa0d8"), hx("#e0ecf0"), 3, RGB{1, 1, 1})
		z.treeline(hor, z.S*0.08, hx("#3a5a2a"), z.seed)
		z.ground(func(float64) float64 { return hor }, hx("#5a4030"), hx("#3a2a1c"), z.S*0.04)
		p := z.persp(hor, 1.6)
		rows := []float64{-2.4, -1.2, 0, 1.2, 2.4}
		if v == 2 {
			rows = []float64{-1.5, 0, 1.5}
			for _, X := range rows {
				q := [][2]float64{}
				for _, pt := range [][2]float64{{X - 0.5, 1.2}, {X + 0.5, 1.2}, {X + 0.5, 30}, {X - 0.5, 30}} {
					x, y := p.pt(pt[0], pt[1], 0)
					q = append(q, [2]float64{x, y})
				}
				z.fill(NewPoly(q), Tex(hx("#3a281a"), hx("#281a10"), 10, 10, 3, 5))
				for _, e := range []float64{-0.5, 0.5} {
					a0, b0 := p.pt(X+e, 1.2, 0)
					a1, b1 := p.pt(X+e, 30, 0)
					z.fill(NewLine(z.S*0.02, [2]float64{a0, b0}, [2]float64{a1, b1}), Solid(hx("#b08a5a")))
				}
			}
		}
		for i, X := range rows {
			for D := 30.0; D > 1.2; D *= 0.9 {
				x, y := p.pt(X, D, 0)
				s := p.scale(D)
				switch {
				case v == 2 && i == 1:
					for k := 0; k < 6; k++ {
						z.fill(Capsule{x + float64(k-3)*s*0.05, y, x + float64(k-3)*s*0.07, y - s*0.35, s * 0.012, s * 0.006}, Solid(hx("#5a9a4a")))
					}
				case v == 2:
					for l := 0; l < 7; l++ {
						a := float64(l) / 7 * 2 * math.Pi
						z.fill(Ellipse{x + math.Cos(a)*s*0.1, y - s*0.05 + math.Sin(a)*s*0.05, s * 0.12, s * 0.07, a}, Solid(z.jit(hx("#8ac850"), 0.1)))
					}
				default:
					z.leaves(x, y-s*0.12, s*0.25, s*0.15, 10, s*0.14, leafG...)
				}
			}
		}
	}
	z.finish(0.2, 0.3)
}

func (z *Z) fir(x, base, h float64, col RGB, snow bool) {
	z.solid(Box{x, base - h*0.05, h * 0.05, h * 0.1, 0, 0}, hx("#3a2a20"))
	for k := 0; k < 5; k++ {
		t := float64(k) / 5
		y := base - h*0.1 - t*h*0.8
		w := h * 0.45 * (1 - t*0.8)
		tri := NewPoly([][2]float64{{x - w, y}, {x, y - h*0.32}, {x + w, y}})
		z.fill(tri, Solid(col.Mul(0.8+0.3*t)))
		if snow {
			z.fillA(NewPoly([][2]float64{{x - w*0.9, y - h*0.01}, {x, y - h*0.3}, {x + w*0.9, y - h*0.01}, {x, y - h*0.12}}), Solid(hx("#f4f8fc")), 0.85, 1.5)
		}
	}
}

func winter(z *Z, v int) {
	hor := z.H * 0.5
	z.sky(hor, hx("#8ab0d8"), hx("#e8eef4"), 2, hx("#f4f6f8"))
	z.treeline(hor, z.S*0.12, hx("#5a6a68"), z.seed)
	sd := z.seed
	z.c.Below(func(x float64) float64 { return hor + z.S*0.02*math.Sin(x/(z.S*0.3)) }, func(x, y float64) RGB {
		return hx("#f4f8fc").Mix(hx("#b8c8dc"), 0.5*fbm(x/(z.S*0.2), y/(z.S*0.05), 4, sd)+0.2*(1-(y-hor)/(z.H-hor)))
	}, 1)
	hx0, base, w, h := z.W*0.45, z.H*0.68, z.S*0.5, z.S*0.28
	z.vplanks(hx0-w/2, hx0+w/2, base-h, base, hx("#7a5a3a"), z.S*0.04)
	z.fill(NewPoly([][2]float64{{hx0 - w*0.62, base - h}, {hx0, base - h - z.S*0.22}, {hx0 + w*0.62, base - h}}), Solid(hx("#5a3a2a")))
	z.fill(NewPoly([][2]float64{{hx0 - w*0.64, base - h + 4}, {hx0, base - h - z.S*0.235}, {hx0 + w*0.64, base - h + 4}, {hx0 + w*0.55, base - h - z.S*0.02}, {hx0, base - h - z.S*0.18}, {hx0 - w*0.55, base - h - z.S*0.02}}), Solid(hx("#f8fafc")))
	z.solid(Box{hx0, base - h*0.5, z.S * 0.1, z.S * 0.1, 0, 0}, hx("#f0ece0"))
	z.solid(Box{hx0, base - h*0.5, z.S * 0.08, z.S * 0.08, 0, 0}, hx("#5a6a78"))
	z.fill(Ellipse{hx0, base + 4, w * 0.6, z.S * 0.04, 0}, Solid(hx("#f4f8fc")))
	for _, f := range [][2]float64{{0.12, 0.45}, {0.85, 0.5}, {0.95, 0.35}, {0.75, 0.25}} {
		z.fir(z.W*f[0], z.H*(0.6+f[1]*0.3), z.S*f[1]*1.2, hx("#1e3a28"), true)
	}
	for k := 0; k < 14; k++ {
		x := z.W*0.35 + float64(k)*z.S*0.04
		y := z.H*0.95 - float64(k)*z.S*0.018
		z.fillA(Ellipse{x + float64(k%2)*z.S*0.02, y, z.S * 0.012, z.S * 0.006, 0}, Solid(hx("#a8b8cc")), 0.8, 2)
	}
	for k := 0; k < 400; k++ {
		z.fillA(Circle{z.rf(0, z.W), z.rf(0, z.H), z.S * z.rf(0.002, 0.005)}, Solid(RGB{1, 1, 1}), 0.8, 1.5)
	}
	z.finish(-0.3, 0.25)
}

func field(z *Z, v int) {
	hor := z.H * 0.62
	z.c.Each(func(x, y float64, _ RGB) RGB {
		t := clamp(y/hor, 0, 1)
		return hx("#3a4a6a").Mix(hx("#f0a050"), smooth(t*1.3-0.2))
	})
	for k := 0; k < 8; k++ {
		z.cloud(z.rf(0, 1)*z.W, z.rf(0.05, 0.35)*z.H, z.S*z.rf(0.4, 0.8), hx("#3a3e52"), 0.85)
	}
	sx, sy := z.W*0.7, hor-z.S*0.08
	z.glow(sx, sy, z.S*0.35, hx("#ffc060"), 0.8)
	for k := 0; k < 7; k++ {
		a := math.Pi/2 + z.rf(-0.8, 0.8)
		z.fillA(NewPoly([][2]float64{{sx, sy}, {sx + math.Cos(a-0.03)*z.W, sy + math.Sin(a-0.03)*z.W}, {sx + math.Cos(a+0.03)*z.W, sy + math.Sin(a+0.03)*z.W}}), Solid(hx("#ffe0a0")), 0.08, 20)
	}
	z.solid(Circle{sx, sy, z.S * 0.03}, hx("#fff4d0"))
	z.treeline(hor, z.S*0.1, hx("#2a2418"), z.seed)
	z.ground(func(float64) float64 { return hor }, hx("#c89a40"), hx("#8a6a28"), z.S*0.05)
	z.fillA(Ellipse{z.W * 0.35, z.H * 0.85, z.S * 0.3, z.S * 0.04, 0}, Solid(hx("#f8c078")), 0.7, 6)
	z.grass(hor, z.H*1.03, 3500, z.S*0.02, z.S*0.15, hx("#d8a840"), hx("#b88a30"), hx("#e8c060"))
	z.finish(0.4, 0.4)
}

func bonfire(z *Z, v int) {
	hor := z.H * 0.55
	z.c.Each(func(x, y float64, _ RGB) RGB {
		t := clamp(y/hor, 0, 1)
		return hx("#141a38").Mix(hx("#e87a38"), smooth(t*1.2-0.2))
	})
	z.treeline(hor, z.S*0.15, hx("#0e0c10"), z.seed)
	for _, x := range []float64{0.08, 0.9} {
		z.fir(z.W*x, hor+z.S*0.05, z.S*0.7, hx("#0a0a0c"), false)
	}
	z.ground(func(float64) float64 { return hor }, hx("#1a1612"), hx("#0e0c0a"), z.S*0.05)
	mx, my, mw := z.W*0.5, z.H*0.72, z.S*0.7
	z.glow(mx, my-z.S*0.05, z.S*0.4, hx("#ff7a20"), 0.7)
	for _, dx := range []float64{-0.45, 0.45} {
		z.solid(Box{mx + dx*mw, my + z.S*0.12, z.S * 0.02, z.S * 0.25, 0, 0}, hx("#1a1a1c"))
	}
	z.fill(NewPoly([][2]float64{{mx - mw/2, my - z.S*0.04}, {mx + mw/2, my - z.S*0.04}, {mx + mw*0.48, my + z.S*0.06}, {mx - mw*0.48, my + z.S*0.06}}), Solid(hx("#2a2a2e")))
	for k := 0; k < 30; k++ {
		x := mx + z.rf(-0.45, 0.45)*mw
		z.fillA(Circle{x, my - z.S*0.04, z.S * z.rf(0.015, 0.03)}, Solid(z.pick(hx("#ff6a10"), hx("#ffb030"), hx("#c02a08"))), 0.9, 3)
	}
	for k := 0; k < 5; k++ {
		x := mx - mw*0.4 + float64(k)*mw*0.2
		z.fill(Capsule{x - z.S*0.03, my - z.S*0.06, x + z.S*0.03, my - z.S*0.06, 2, 2}, Solid(hx("#b8b8c0")))
		z.fill(Capsule{x, my + z.S*0.02, x, my - z.S*0.3, 2, 2}, Solid(hx("#b8b8c0")))
		for m := 0; m < 4; m++ {
			y := my - z.S*0.08 - float64(m)*z.S*0.05
			z.fill(Box{x, y, z.S * 0.05, z.S * 0.04, z.rf(-0.2, 0.2), z.S * 0.012}, Shade(x, y, z.S*0.04, z.S*0.03, 0, hx("#8a3a1a"), 0.3))
		}
	}
	for k := 0; k < 6; k++ {
		z.fillA(Circle{mx + z.rf(-0.2, 0.2)*z.S, my - z.S*(0.35+0.1*float64(k)), z.S * (0.08 + 0.03*float64(k))}, Solid(hx("#c8c0c8")), 0.1, z.S*0.05)
	}
	for k := 0; k < 40; k++ {
		z.solid(Circle{mx + z.rf(-0.3, 0.3)*z.S, my - z.rf(0.1, 0.6)*z.S, z.S * 0.003}, hx("#ffb040"))
	}
	z.finish(0.3, 0.45)
}
