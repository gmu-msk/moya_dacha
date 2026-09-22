package main

// Крупные планы: урожай, ягоды, цветы, заготовки, кот.

import (
	"math"
	"sort"
)

// pile — раскладывает n предметов по эллипсу и рисует от дальних к ближним.
func (z *Z) pile(cx, cy, rx, ry float64, n int, draw func(x, y float64, i int)) {
	type pt struct{ x, y float64 }
	ps := make([]pt, n)
	for i := range ps {
		a := z.rf(0, 2*math.Pi)
		d := math.Sqrt(z.r.Float64())
		ps[i] = pt{cx + math.Cos(a)*rx*d, cy + math.Sin(a)*ry*d}
	}
	sort.Slice(ps, func(i, j int) bool { return ps[i].y < ps[j].y })
	for i, p := range ps {
		draw(p.x, p.y, i)
	}
}

// basket — плетёная корзина: задняя стенка, содержимое, передняя стенка.
func (z *Z) basket(cx, cy, w, h float64, col RGB, fill func()) {
	rim := Ellipse{cx, cy - h*0.35, w / 2, h * 0.22, 0}
	weave := func(x, y float64) RGB {
		k := 0.5 + 0.5*math.Sin(x/(w*0.018))*math.Sin(y/(h*0.05))
		c := col.Mix(col.Mul(0.6), k)
		return c.Mul(0.75 + 0.35*(1-clamp((y-cy+h/2)/h, 0, 1)))
	}
	z.shadow(cx+w*0.05, cy+h*0.5, w*0.55, h*0.1, 0.5)
	z.fill(rim, Solid(col.Mul(0.35)))
	fill()
	body := NewPoly([][2]float64{{cx - w/2, cy - h*0.35}, {cx + w/2, cy - h*0.35}, {cx + w*0.4, cy + h*0.45}, {cx - w*0.4, cy + h*0.45}})
	z.fill(Cut{body, Ellipse{cx, cy - h*0.35, w / 2, h * 0.22, 0}}, weave)
	z.fill(Ellipse{cx, cy + h*0.45, w * 0.4, h * 0.08, 0}, weave)
	z.fill(And{Ring{rim, h * 0.06}, Box{cx, cy, w * 2, h * 0.7, 0, 0}}, Solid(col.Mul(0.9)))
}

func tomatoes(z *Z, v int) {
	red, black, yellow, pink := hx("#c4202a"), hx("#5a1a1e"), hx("#f0a020"), hx("#e05a60")
	switch v {
	case 0: // разные сорта на столе
		z.bokeh([]RGB{hx("#6f8a4a"), hx("#9aa86a"), hx("#c8c29a")}, hx("#fff8e0"), 30, 0.04)
		z.planks(z.H*0.35, hx("#9c7650"), 5)
		cols := []RGB{red, black, yellow, pink, hx("#d8c030"), hx("#b02030"), hx("#e87830")}
		z.pile(z.W*0.5, z.H*0.68, z.W*0.42, z.H*0.24, 24, func(x, y float64, i int) {
			r := z.S * z.rf(0.055, 0.11)
			z.shadow(x+r*0.3, y+r*0.85, r*1.1, r*0.25, 0.45)
			z.tomato(x, y, r, z.jit(cols[i%len(cols)], 0.05), i%4 == 0)
		})
		z.finish(0.35, 0.35)
	case 1: // «Чёрный принц» в эмалированной миске
		z.bokeh([]RGB{hx("#b8a888"), hx("#8a7a5a"), hx("#d8ccb0")}, hx("#fff0d0"), 30, 0.04)
		z.planks(z.H*0.5, hx("#7a5a3a"), 4)
		cx, cy := z.W*0.5, z.H*0.6
		rim := Ellipse{cx, cy, z.S * 0.4, z.S * 0.13, 0}
		z.shadow(cx, cy+z.S*0.24, z.S*0.4, z.S*0.07, 0.5)
		z.fill(rim, Solid(hx("#d8d4c8")))
		z.pile(cx, cy-z.S*0.04, z.S*0.3, z.S*0.08, 14, func(x, y float64, i int) {
			z.tomato(x, y, z.S*z.rf(0.08, 0.1), z.jit(black, 0.1), true)
		})
		front := Box{cx, cy + z.S*0.25, z.S, z.S * 0.5, 0, 0}
		z.fill(And{Ellipse{cx, cy + z.S*0.02, z.S * 0.4, z.S * 0.22, 0}, front}, Lin(0, cy, 0, cy+z.S*0.24, hx("#f4f2ec"), hx("#b4b0a4")))
		z.fill(And{Ring{rim, z.S * 0.01}, front}, Solid(hx("#8a2020")))
		z.finish(0.3, 0.4)
	case 2: // «Хурма» в корзине на траве
		z.bokeh(leafG, hx("#fffbe0"), 30, 0.04)
		z.grass(z.H*0.4, z.H*1.05, 2200, z.S*0.05, z.S*0.12, leafG...)
		z.basket(z.W*0.5, z.H*0.6, z.S*0.8, z.S*0.5, hx("#b08850"), func() {
			z.pile(z.W*0.5, z.H*0.6-z.S*0.2, z.S*0.34, z.S*0.08, 16, func(x, y float64, i int) {
				z.tomato(x, y, z.S*z.rf(0.07, 0.09), z.jit(yellow, 0.08), false)
			})
		})
		z.finish(0.4, 0.35)
	case 3: // разрезанные на семена
		z.planks(0, hx("#6a4a30"), 6)
		bx, by := z.W*0.5, z.H*0.52
		z.shadow(bx+z.S*0.03, by+z.S*0.05, z.S*0.62, z.S*0.4, 0.4)
		z.fill(Box{bx, by, z.S * 1.1, z.S * 0.72, 0.05, z.S * 0.06}, Tex(hx("#d8b98a"), hx("#c09a68"), z.S*0.3, z.S*0.02, 3, z.seed))
		for i, p := range [][2]float64{{-0.28, -0.12}, {0.02, -0.14}, {0.3, -0.08}, {-0.14, 0.14}, {0.18, 0.15}} {
			x, y, r := bx+p[0]*z.S, by+p[1]*z.S, z.S*0.12
			c := z.jit(red, 0.06)
			if i == 2 {
				c = hx("#d05060")
			}
			z.fill(Circle{x, y, r}, Solid(c.Mul(0.85)))
			z.fill(Circle{x, y, r * 0.9}, Solid(c.Mix(hx("#ff9090"), 0.25)))
			for k := 0; k < 4; k++ {
				a := float64(k)/4*2*math.Pi + 0.4
				cx, cy := x+math.Cos(a)*r*0.45, y+math.Sin(a)*r*0.45
				z.fill(Ellipse{cx, cy, r * 0.28, r * 0.2, a}, Solid(c.Mix(hx("#e8a050"), 0.3)))
				for s := 0; s < 6; s++ {
					z.solid(Ellipse{cx + z.rf(-1, 1)*r*0.15, cy + z.rf(-1, 1)*r*0.1, r * 0.035, r * 0.022, a}, hx("#f0e0a0"))
				}
			}
			z.fillA(Circle{x, y, r * 0.15}, Solid(c.Mix(hx("#ffffff"), 0.3)), 0.8, 3)
		}
		z.fill(Box{bx + z.S*0.2, by + z.S*0.36, z.S * 0.5, z.S * 0.05, -0.15, z.S * 0.01}, Lin(0, by+z.S*0.33, 0, by+z.S*0.4, hx("#e8e8ec"), hx("#8a8a90")))
		z.fill(Box{bx + z.S*0.55, by + z.S*0.3, z.S * 0.26, z.S * 0.06, -0.15, z.S * 0.02}, Solid(hx("#2a1a12")))
		z.finish(0.3, 0.3)
	case 4, 5, 6: // в теплице: красные кисти, жёлтые, джунгли
		z.bokeh([]RGB{hx("#a8c88a"), hx("#e8f0d8"), hx("#6a9a4a")}, hx("#ffffff"), 35, 0.035)
		for x := z.W * 0.15; x < z.W; x += z.W * 0.33 {
			z.fill(Capsule{x, 0, x + z.rf(-0.02, 0.02)*z.W, z.H, z.S * 0.003, z.S * 0.003}, Solid(hx("#e8e0c8")))
			z.fill(Capsule{x + z.S*0.01, z.H * 0.1, x - z.S*0.01, z.H, z.S * 0.012, z.S * 0.018}, Solid(hx("#4a7a2a")))
		}
		n := 50
		if v == 6 {
			n = 140
		}
		z.leaves(z.W*0.5, z.H*0.5, z.W*0.6, z.H*0.55, n, z.S*0.22, leafG...)
		col, r, clusters := red, z.S*0.07, 5
		switch v {
		case 5:
			col, r, clusters = hx("#f2c030"), z.S*0.09, 3
		case 6:
			col, r, clusters = hx("#8ab84a"), z.S*0.05, 3
		}
		for k := 0; k < clusters; k++ {
			x, y := z.rf(0.2, 0.8)*z.W, z.rf(0.25, 0.8)*z.H
			z.fill(NewLine(z.S*0.01, [2]float64{x - r, y - r*2}, [2]float64{x, y - r}, [2]float64{x + r, y + r}), Solid(hx("#4a7a2a")))
			z.pile(x, y, r*1.2, r*1.1, 5, func(px, py float64, i int) {
				c := col
				if v == 4 && i == 0 {
					c = hx("#e89040")
				}
				z.tomato(px, py, r*z.rf(0.8, 1), z.jit(c, 0.06), false)
			})
		}
		z.finish(0.2, 0.3)
	}
}

func cucumbers(z *Z, v int) {
	green := hx("#2f6a1e")
	switch v {
	case 0: // висят в теплице
		z.bokeh([]RGB{hx("#9fc47a"), hx("#dfe8c8"), hx("#5f8f3a")}, hx("#ffffff"), 35, 0.03)
		for _, x := range []float64{0.2, 0.5, 0.8} {
			z.fill(Capsule{z.W * x, 0, z.W*x + z.S*0.02, z.H, z.S * 0.003, z.S * 0.003}, Solid(hx("#efe8d2")))
		}
		z.leaves(z.W*0.5, z.H*0.3, z.W*0.6, z.H*0.3, 45, z.S*0.3, leafG...)
		for i, c := range [][3]float64{{0.3, 0.35, 0.34}, {0.52, 0.42, 0.4}, {0.7, 0.3, 0.3}, {0.42, 0.6, 0.3}} {
			x, y, l := z.W*c[0], z.H*c[1], z.H*c[2]
			z.fill(Capsule{x, y - z.S*0.08, x, y, z.S * 0.008, z.S * 0.008}, Solid(hx("#5a8a32")))
			z.cucumber(x, y, x+z.rf(-0.03, 0.03)*z.S, y+l, z.S*0.05, z.jit(green, 0.1))
			if i%2 == 0 {
				z.flower(x, y+l+z.S*0.05, z.S*0.05, 5, hx("#f4d020"), hx("#e0a010"), 0.4)
			}
		}
		z.leaves(z.W*0.5, z.H*0.95, z.W*0.6, z.H*0.1, 12, z.S*0.3, leafDk...)
		z.finish(0.15, 0.3)
	case 1: // на столе с укропом
		z.planks(0, hx("#8a6a48"), 6)
		for k := 0; k < 4; k++ {
			x, y := z.rf(0.1, 0.9)*z.W, z.rf(0.1, 0.9)*z.H
			for b := 0; b < 9; b++ {
				a := z.rf(-math.Pi, 0)
				l := z.S * z.rf(0.08, 0.16)
				ex, ey := x+math.Cos(a)*l, y+math.Sin(a)*l
				z.fill(Capsule{x, y, ex, ey, z.S * 0.003, z.S * 0.002}, Solid(hx("#5f8a3a")))
				for f := 0; f < 10; f++ {
					fa := a + z.rf(-0.8, 0.8)
					z.fill(Capsule{ex, ey, ex + math.Cos(fa)*l*0.3, ey + math.Sin(fa)*l*0.3, 1.2, 0.8}, Solid(z.jit(hx("#6a9a40"), 0.15)))
				}
			}
		}
		z.pile(z.W*0.5, z.H*0.5, z.W*0.36, z.H*0.3, 16, func(x, y float64, i int) {
			a := z.rf(-0.8, 0.8)
			l := z.S * z.rf(0.16, 0.22)
			z.shadow(x, y+z.S*0.04, l*0.55, z.S*0.03, 0.4)
			z.cucumber(x-math.Cos(a)*l/2, y-math.Sin(a)*l/2, x+math.Cos(a)*l/2, y+math.Sin(a)*l/2, z.S*0.035, z.jit(green, 0.12))
		})
		for k := 0; k < 5; k++ {
			x, y := z.rf(0.1, 0.9)*z.W, z.rf(0.75, 0.95)*z.H
			z.sphere(x, y, z.S*0.022, z.S*0.03, z.rf(-1, 1), hx("#f2ece0"), 0.3)
		}
		z.finish(0.3, 0.35)
	}
}

func raspberry(z *Z, v int) {
	col := hx("#c21e4a")
	switch v {
	case 0:
		z.bokeh(append(leafG, hx("#9aba60")), hx("#fffbe8"), 40, 0.035)
		z.leaves(z.W*0.5, z.H*0.45, z.W*0.6, z.H*0.5, 60, z.S*0.25, leafG...)
		for k := 0; k < 3; k++ {
			x := z.rf(0.2, 0.8) * z.W
			z.fill(NewLine(z.S*0.012, [2]float64{x, 0}, [2]float64{x + z.rf(-0.1, 0.1)*z.S, z.H * 0.5}, [2]float64{x + z.rf(-0.15, 0.15)*z.S, z.H}), Solid(hx("#7a5a3a")))
		}
		for k := 0; k < 9; k++ {
			x, y := z.rf(0.15, 0.85)*z.W, z.rf(0.3, 0.85)*z.H
			z.fill(Capsule{x, y - z.S*0.12, x, y - z.S*0.05, z.S * 0.006, z.S * 0.006}, Solid(hx("#5a7a32")))
			for s := 0; s < 5; s++ {
				a := -math.Pi/2 + (float64(s)/4-0.5)*2
				z.fill(Leaf(x, y-z.S*0.06, z.S*0.05, z.S*0.02, a), Solid(hx("#4a7a2a")))
			}
			c := col
			if k%4 == 3 {
				c = hx("#d8d890")
			}
			z.raspberry(x, y, z.S*z.rf(0.1, 0.13), z.rf(-0.2, 0.2), c)
		}
		z.finish(0.25, 0.3)
	case 1: // миска малины сверху
		z.planks(0, hx("#a07a50"), 5)
		cx, cy := z.W*0.5, z.H*0.5
		z.shadow(cx+z.S*0.03, cy+z.S*0.04, z.S*0.4, z.S*0.4, 0.5)
		z.fill(Circle{cx, cy, z.S * 0.4}, Lin(cx-z.S*0.4, cy-z.S*0.4, cx+z.S*0.4, cy+z.S*0.4, hx("#f6f4ee"), hx("#c8c4b8")))
		z.fill(Circle{cx, cy, z.S * 0.34}, Solid(hx("#6a1020")))
		z.pile(cx, cy, z.S*0.3, z.S*0.3, 70, func(x, y float64, i int) {
			z.raspberry(x, y, z.S*z.rf(0.07, 0.09), z.rf(0, 6.3), z.jit(col, 0.08))
		})
		z.fill(Leaf(cx+z.S*0.3, cy-z.S*0.35, z.S*0.2, z.S*0.1, -0.5), Solid(hx("#4a8a2a")))
		z.finish(0.2, 0.35)
	}
}

func jars(z *Z, v int) {
	switch v {
	case 0: // мёд на подоконнике
		z.bokeh([]RGB{hx("#e8d8a8"), hx("#c8b080"), hx("#f8f0d8")}, hx("#ffffff"), 30, 0.04)
		z.planks(z.H*0.62, hx("#8a6440"), 3)
		for i, x := range []float64{0.26, 0.5, 0.74} {
			w, h := z.S*0.24, z.S*0.34
			c := []RGB{hx("#b86a10"), hx("#d89a2a"), hx("#6a3a10")}[i]
			z.jar(z.W*x, z.H*0.7-h/2+z.S*0.05, w, h, c, hx("#c8a040"), nil)
		}
		z.glow(z.W*0.5, z.H*0.5, z.S*0.4, hx("#ffcc70"), 0.15)
		z.finish(0.4, 0.3)
	case 1, 2: // лечо: ряды банок и полка в кладовке
		if v == 1 {
			z.bokeh([]RGB{hx("#d8d0c0"), hx("#a8a090"), hx("#f0ece4")}, hx("#ffffff"), 20, 0.04)
			z.planks(z.H*0.7, hx("#6a4a30"), 3)
		} else {
			z.vplanks(0, z.W, 0, z.H, hx("#5a4030"), z.S*0.12)
			for _, y := range []float64{0.45, 0.9} {
				z.solid(Box{z.W / 2, z.H * y, z.W, z.S * 0.04, 0, 0}, hx("#8a6a48"))
			}
		}
		rows := [][2]float64{{0.72, 0.28}}
		if v == 2 {
			rows = [][2]float64{{0.43, 0.2}, {0.88, 0.2}}
		}
		for _, row := range rows {
			n := 4
			if v == 2 {
				n = 3
			}
			for k := 0; k < n; k++ {
				w, h := z.S*row[1], z.S*row[1]*1.35
				x := z.W * (float64(k) + 0.5) / float64(n)
				y := z.H*row[0] - h/2
				c := z.jit(hx("#c43a18"), 0.08)
				z.jar(x, y, w, h, c, hx("#b0a080"), func(x0, y0, x1, y1 float64) {
					for s := 0; s < 22; s++ {
						px, py := z.rf(x0+w*0.1, x1-w*0.1), z.rf(y0, y1-h*0.05)
						z.fillA(Ellipse{px, py, w * 0.09, w * 0.05, z.rf(0, 3)}, Solid(z.pick(hx("#e85a20"), hx("#f0a020"), hx("#a01a10"), hx("#e8c040"))), 0.8, 2)
					}
				})
			}
		}
		z.finish(0.3, 0.35)
	case 3: // варенье из крыжовника
		z.bokeh([]RGB{hx("#c8d0a0"), hx("#90a870"), hx("#e8e8c8")}, hx("#fffff0"), 30, 0.04)
		z.planks(z.H*0.62, hx("#7a5a3a"), 3)
		for i, x := range []float64{0.32, 0.68} {
			w, h := z.S*0.3, z.S*0.38
			z.jar(z.W*x, z.H*0.65-h/2+z.S*0.05, w, h, hx("#8a9a30"), hx("#d8c050"), func(x0, y0, x1, y1 float64) {
				for s := 0; s < 16; s++ {
					px, py := z.rf(x0+w*0.15, x1-w*0.15), z.rf(y0+h*0.1, y1-h*0.1)
					z.fillA(Circle{px, py, w * 0.08}, Shade(px, py, w*0.08, w*0.08, 0, hx("#b8c850"), 0.4), 0.75, 2)
				}
			})
			if i == 0 {
				z.fill(Leaf(z.W*0.5, z.H*0.82, z.S*0.16, z.S*0.07, 0.3), Solid(hx("#3a6a22")))
			}
		}
		z.finish(0.3, 0.3)
	case 4: // соленья в трёхлитровых банках
		z.bokeh([]RGB{hx("#e0dccc"), hx("#b0aa98"), hx("#f4f0e4")}, hx("#ffffff"), 20, 0.04)
		z.planks(z.H*0.74, hx("#8a6a48"), 3)
		for _, x := range []float64{0.28, 0.72} {
			w, h := z.S*0.4, z.S*0.62
			y := z.H*0.8 - h/2
			z.jar(z.W*x, y, w, h, hx("#b8b87a"), hx("#c8c8c8"), func(x0, y0, x1, y1 float64) {
				for s := 0; s < 7; s++ {
					px := x0 + w*(0.12+0.76*float64(s)/6)
					z.cucumber(px, y0+h*0.08, px+z.rf(-0.03, 0.03)*w, y1-h*0.06, w*0.07, hx("#4a6a2a"))
				}
				for s := 0; s < 3; s++ {
					px, py := z.rf(x0, x1), z.rf(y0, y1)
					z.fillA(NewLine(w*0.01, [2]float64{px, py}, [2]float64{px + w*0.1, py - h*0.2}), Solid(hx("#6a8a3a")), 0.8, 1)
					z.fillA(Ellipse{z.rf(x0, x1), z.rf(y0, y1), w * 0.03, w * 0.04, 0}, Solid(hx("#f0ecd8")), 0.9, 1)
				}
			})
		}
		z.finish(0.2, 0.3)
	}
}

func apples(z *Z, v int) {
	anton, antonB := hx("#d8d060"), hx("#a8b840")
	redA, redB := hx("#c83028"), hx("#e8c040")
	switch v {
	case 0: // антоновка на дереве, ветки на подпорках
		z.vgrad(hx("#8ab8e0"), hx("#dce8e8"))
		z.cloud(z.W*0.3, z.H*0.1, z.S*0.5, hx("#ffffff"), 0.8)
		z.ground(func(x float64) float64 { return z.H * 0.8 }, hx("#5a8a30"), hx("#3a6a22"), z.S*0.05)
		z.fill(Capsule{z.W * 0.45, z.H, z.W * 0.48, z.H * 0.45, z.S * 0.07, z.S * 0.04}, Tex(hx("#5a4a3a"), hx("#3a2e24"), z.S*0.02, z.S*0.1, 3, z.seed))
		for k := 0; k < 6; k++ {
			a := -math.Pi/2 + z.rf(-1.2, 1.2)
			l := z.S * z.rf(0.35, 0.55)
			z.fill(Capsule{z.W * 0.48, z.H * 0.5, z.W*0.48 + math.Cos(a)*l, z.H*0.5 + math.Sin(a)*l, z.S * 0.03, z.S * 0.008}, Solid(hx("#4a3a2c")))
		}
		z.leaves(z.W*0.5, z.H*0.35, z.W*0.55, z.H*0.28, 320, z.S*0.07, leafG...)
		for k := 0; k < 45; k++ {
			a := z.rf(0, 2*math.Pi)
			d := math.Sqrt(z.r.Float64())
			z.apple(z.W*0.5+math.Cos(a)*d*z.W*0.5, z.H*0.4+math.Sin(a)*d*z.H*0.25, z.S*z.rf(0.03, 0.045), anton, antonB, 0.4)
		}
		for _, x := range []float64{0.15, 0.82} {
			z.fill(Capsule{z.W * x, z.H * 0.95, z.W * (x + (0.5-x)*0.3), z.H * 0.45, z.S * 0.012, z.S * 0.01}, Solid(hx("#8a7a5a")))
		}
		z.grass(z.H*0.8, z.H*1.02, 1500, z.S*0.02, z.S*0.07, leafG...)
		z.finish(0.2, 0.3)
	case 1, 3: // корзина антоновки на траве; корзина красных на столе
		a, b, k := anton, antonB, 0.4
		if v == 3 {
			a, b, k = redB, redA, 0.9
			z.bokeh([]RGB{hx("#a0b870"), hx("#d8d8a8"), hx("#6a8a4a")}, hx("#ffffff"), 30, 0.04)
			z.planks(z.H*0.55, hx("#8a6440"), 4)
		} else {
			z.bokeh(leafG, hx("#fffbe0"), 30, 0.04)
			z.grass(z.H*0.35, z.H*1.05, 2200, z.S*0.05, z.S*0.12, leafG...)
		}
		z.basket(z.W*0.5, z.H*0.62, z.S*0.85, z.S*0.5, hx("#a88048"), func() {
			z.pile(z.W*0.5, z.H*0.62-z.S*0.2, z.S*0.36, z.S*0.08, 16, func(x, y float64, i int) {
				z.apple(x, y, z.S*z.rf(0.075, 0.09), a, b, k)
			})
		})
		for i := 0; i < 3; i++ {
			x := z.W * []float64{0.12, 0.85, 0.9}[i]
			y := z.H * []float64{0.85, 0.8, 0.92}[i]
			z.shadow(x, y+z.S*0.07, z.S*0.08, z.S*0.02, 0.4)
			z.apple(x, y, z.S*0.08, a, b, k)
		}
		z.finish(0.3, 0.35)
	case 2: // ящик яблок
		z.bokeh([]RGB{hx("#b8b090"), hx("#8a8a60"), hx("#d8d0b0")}, hx("#ffffff"), 25, 0.04)
		z.ground(func(float64) float64 { return z.H * 0.72 }, hx("#8a7a5a"), hx("#6a5a40"), z.S*0.05)
		cx, top, w, h := z.W*0.5, z.H*0.38, z.S*1.1, z.S*0.5
		z.fill(Box{cx, top, w, z.S * 0.1, 0, 0}, Solid(hx("#3a2a1a")))
		for r := 0; r < 2; r++ {
			for k := 0; k < 9; k++ {
				x := cx - w/2 + w*(float64(k)+0.5+float64(r)*0.5)/9.5
				z.apple(x, top-z.S*0.02+float64(r)*z.S*0.03, z.S*0.065, anton, redA, z.rf(0, 0.5))
			}
		}
		for k := 0; k < 3; k++ {
			y := top + z.S*0.05 + float64(k)*h/3
			z.fill(Box{cx, y + h/6, w, h/3 - z.S*0.02, 0, 0}, Tex(hx("#c8a878"), hx("#a8885a"), z.S*0.3, z.S*0.01, 3, z.seed+k))
		}
		for _, x := range []float64{-0.5, 0.5} {
			z.solid(Box{cx + x*w, top + h/2 + z.S*0.04, z.S * 0.05, h + z.S*0.08, 0, 0}, hx("#9a7a50"))
		}
		z.finish(0.25, 0.35)
	}
}

func dahlias(z *Z, v int) {
	z.bokeh(append(leafDk, hx("#6a8a40")), hx("#fff8e0"), 35, 0.04)
	if v == 0 {
		z.leaves(z.W*0.5, z.H*0.7, z.W*0.6, z.H*0.35, 60, z.S*0.18, leafG...)
		for _, f := range [][4]float64{{0.25, 0.4, 0.16, 0}, {0.55, 0.3, 0.2, 1}, {0.8, 0.5, 0.15, 2}, {0.45, 0.68, 0.14, 1}} {
			c := []RGB{hx("#8a1030"), hx("#e04070"), hx("#c02020")}[int(f[3])]
			z.fill(Capsule{z.W * f[0], z.H * f[1], z.W * f[0], z.H, z.S * 0.01, z.S * 0.012}, Solid(hx("#3a6a22")))
			z.bloom(z.W*f[0], z.H*f[1], z.S*f[2], c, 5, true)
		}
	} else {
		z.leaves(z.W*0.5, z.H*0.8, z.W*0.6, z.H*0.3, 30, z.S*0.25, leafG...)
		z.fill(Capsule{z.W * 0.5, z.H * 0.45, z.W * 0.52, z.H, z.S * 0.015, z.S * 0.018}, Solid(hx("#3a6a22")))
		z.bloom(z.W*0.5, z.H*0.42, z.S*0.38, hx("#f08a20"), 7, true)
		z.sphere(z.W*0.22, z.H*0.7, z.S*0.05, z.S*0.06, 0, hx("#4a7a2a"), 0.2)
		z.sphere(z.W*0.8, z.H*0.25, z.S*0.04, z.S*0.05, 0, hx("#4a7a2a"), 0.2)
	}
	z.finish(0.3, 0.35)
}

func potato(z *Z, v int) {
	pc := hx("#d8b880")
	soil := func(y float64) {
		z.ground(func(float64) float64 { return y }, hx("#6a5038"), hx("#3e2c1e"), z.S*0.05)
		for k := 0; k < 400; k++ {
			x, yy := z.rf(0, z.W), z.rf(y, z.H)
			z.fillA(Blob(z.r, x, yy, z.S*0.01, z.S*0.008, 0.4), Solid(hx("#2e2016")), 0.5, 2)
		}
	}
	switch v {
	case 0: // мелкая, только выкопанная
		soil(0)
		for k := 0; k < 6; k++ {
			x := z.rf(0, z.W)
			z.fill(NewLine(z.S*0.006, [2]float64{x, z.rf(0, z.H)}, [2]float64{x + z.rf(-0.2, 0.2)*z.S, z.rf(0, z.H)}, [2]float64{x + z.rf(-0.3, 0.3)*z.S, z.rf(0, z.H)}), Solid(hx("#6a5a30")))
		}
		z.pile(z.W*0.5, z.H*0.55, z.W*0.35, z.H*0.3, 30, func(x, y float64, i int) {
			z.potato(x, y, z.S*z.rf(0.025, 0.045), z.jit(pc, 0.08))
		})
		z.fill(Capsule{z.W * 0.85, z.H * 0.2, z.W * 0.95, -10, z.S * 0.015, z.S * 0.015}, Solid(hx("#8a6a40")))
		for t := -1.0; t <= 1; t++ {
			z.fill(Capsule{z.W*0.85 + t*z.S*0.03, z.H * 0.2, z.W*0.82 + t*z.S*0.04, z.H * 0.45, z.S * 0.007, z.S * 0.004}, Solid(hx("#9a9aa0")))
		}
		z.finish(0.15, 0.35)
	case 1: // в ведре
		soil(0)
		cx, cy := z.W*0.5, z.H*0.45
		w := z.S * 0.6
		z.shadow(cx+z.S*0.05, cy+z.S*0.36, w*0.5, z.S*0.07, 0.5)
		z.fill(Ellipse{cx, cy - z.S*0.2, w / 2, w * 0.15, 0}, Solid(hx("#303234")))
		z.pile(cx, cy-z.S*0.22, w*0.4, w*0.1, 18, func(x, y float64, i int) {
			z.potato(x, y, z.S*z.rf(0.03, 0.045), z.jit(pc, 0.08))
		})
		body := NewPoly([][2]float64{{cx - w/2, cy - z.S*0.2}, {cx + w/2, cy - z.S*0.2}, {cx + w*0.38, cy + z.S*0.35}, {cx - w*0.38, cy + z.S*0.35}})
		z.fill(Cut{body, Ellipse{cx, cy - z.S*0.2, w / 2, w * 0.15, 0}}, Lin(cx-w/2, 0, cx+w/2, 0, hx("#b8bcc0"), hx("#6a6e72")))
		z.fill(Ellipse{cx, cy + z.S*0.35, w * 0.38, w * 0.08, 0}, Solid(hx("#8a8e92")))
		z.fill(And{Ring{Ellipse{cx, cy - z.S*0.2, w / 2, w * 0.15, 0}, z.S * 0.015}, Box{cx, cy, w * 2, z.S * 0.4, 0, 0}}, Solid(hx("#d0d4d8")))
		z.finish(0.1, 0.35)
	case 2: // мешки вдоль забора
		z.vgrad(hx("#a8c0d8"), hx("#e0e4e0"))
		z.fence(z.H*0.55, z.S*0.3, hx("#a89878"), z.S*0.06)
		z.ground(func(float64) float64 { return z.H * 0.52 }, hx("#7a6a48"), hx("#5a4a30"), z.S*0.05)
		for k := 0; k < 6; k++ {
			x := z.W * (0.1 + 0.16*float64(k))
			h := z.S * z.rf(0.4, 0.48)
			z.fill(Box{x, z.H*0.85 - h/2, z.S * 0.24, h, z.rf(-0.05, 0.05), z.S * 0.08}, func(px, py float64) RGB {
				k := 0.5 + 0.5*math.Sin(px*0.6)*math.Sin(py*0.6)
				return hx("#c8a870").Mix(hx("#9a7a48"), k*0.5+fbm(px/40, py/40, 2, 3)*0.3)
			})
			z.fill(Capsule{x - z.S*0.05, z.H*0.85 - h, x + z.S*0.05, z.H*0.85 - h, z.S * 0.02, z.S * 0.02}, Solid(hx("#6a5a38")))
		}
		z.pile(z.W*0.5, z.H*0.95, z.W*0.4, z.H*0.04, 20, func(x, y float64, i int) {
			z.potato(x, y, z.S*0.035, z.jit(pc, 0.08))
		})
		z.finish(0.2, 0.3)
	case 3: // выкопанные ряды
		z.vgrad(hx("#9ac0e0"), hx("#e8ecec"))
		hor := z.H * 0.28
		z.treeline(hor, z.S*0.08, hx("#3a5a30"), z.seed)
		z.ground(func(float64) float64 { return hor }, hx("#7a5a3a"), hx("#4a3424"), z.S*0.04)
		vx := z.W * 0.5
		for r := -5; r <= 5; r++ {
			for k := 0; k < 30; k++ {
				t := math.Pow(z.r.Float64(), 1.6)
				y := hor + (z.H-hor)*t
				x := vx + float64(r)*z.W*0.12*t*1.8
				z.potato(x+z.rf(-0.02, 0.02)*z.S*t, y, z.S*0.035*t+1, z.jit(pc, 0.1))
			}
		}
		z.finish(0.2, 0.3)
	}
}

func flowerbed(z *Z, v int) {
	z.bokeh(append(leafG, hx("#8ab060")), hx("#ffffff"), 40, 0.035)
	z.leaves(z.W*0.5, z.H*0.7, z.W*0.6, z.H*0.4, 90, z.S*0.12, leafG...)
	switch v {
	case 0: // астры
		for k := 0; k < 22; k++ {
			x, y := z.rf(0.05, 0.95)*z.W, z.rf(0.2, 0.9)*z.H
			c := z.pick(hx("#8a4ac8"), hx("#d860a0"), hx("#f0f0f8"), hx("#b050d0"), hx("#e84a70"))
			r := z.S * z.rf(0.06, 0.1)
			z.flower(x, y, r, 26, c, hx("#f0c030"), 0.12)
			z.flower(x, y, r*0.7, 20, c.Mul(1.1), hx("#f0c030"), 0.12)
		}
	case 1: // флоксы
		for k := 0; k < 9; k++ {
			x, y := z.rf(0.1, 0.9)*z.W, z.rf(0.15, 0.7)*z.H
			c := z.pick(hx("#d870b0"), hx("#9a60d0"), hx("#f4e8f0"), hx("#e05080"))
			z.fill(Capsule{x, y, x, z.H, z.S * 0.01, z.S * 0.012}, Solid(hx("#3a6a22")))
			for s := 0; s < 30; s++ {
				a := z.rf(0, 2*math.Pi)
				d := math.Sqrt(z.r.Float64())
				z.flower(x+math.Cos(a)*d*z.S*0.12, y+math.Sin(a)*d*z.S*0.09, z.S*0.035, 5, z.jit(c, 0.08), c.Mul(0.6), 0.45)
			}
		}
	case 2: // лилии
		for k := 0; k < 5; k++ {
			x, y := z.rf(0.15, 0.85)*z.W, z.rf(0.2, 0.65)*z.H
			z.fill(Capsule{x, y, x + z.rf(-0.05, 0.05)*z.S, z.H, z.S * 0.012, z.S * 0.014}, Solid(hx("#3a6a22")))
			c := z.pick(hx("#f07820"), hx("#f4f0e8"), hx("#f0a0b0"))
			r := z.S * z.rf(0.13, 0.18)
			for p := 0; p < 6; p++ {
				a := float64(p)/6*2*math.Pi + z.rf(0, 0.3)
				z.fill(Leaf(x, y, r, r*0.35, a), Lin(x, y, x+math.Cos(a)*r, y+math.Sin(a)*r, c.Mul(0.8), c))
				for s := 0; s < 4; s++ {
					t := z.rf(0.2, 0.6)
					z.solid(Circle{x + math.Cos(a)*r*t, y + math.Sin(a)*r*t, r * 0.02}, hx("#6a2a1a"))
				}
			}
			for p := 0; p < 6; p++ {
				a := z.rf(0, 2*math.Pi)
				ex, ey := x+math.Cos(a)*r*0.5, y+math.Sin(a)*r*0.5
				z.fill(Capsule{x, y, ex, ey, 1.5, 1.5}, Solid(hx("#c8c880")))
				z.solid(Ellipse{ex, ey, r * 0.05, r * 0.025, a}, hx("#a04a10"))
			}
		}
	}
	z.finish(0.25, 0.35)
}

func pumpkin(z *Z, v int) {
	one := func(x, y, r float64, col RGB) {
		z.shadow(x, y+r*0.75, r*1.3, r*0.2, 0.5)
		for k := -3; k <= 3; k++ {
			kk := float64(k)
			if k < 0 {
				kk = -kk
			}
			w := r * (0.55 - kk*0.06)
			px := x + float64(k)*r*0.28
			z.fill(Ellipse{px, y, w, r * 0.78, 0}, Shade(px, y, w*1.1, r*0.8, 0, z.jit(col, 0.04).Mul(1-kk*0.05), 0.25))
		}
		z.fill(Capsule{x, y - r*0.7, x + r*0.1, y - r*1.05, r * 0.08, r * 0.06}, Solid(hx("#5a6a2a")))
	}
	switch v {
	case 0:
		z.bokeh([]RGB{hx("#8a9a50"), hx("#c8b870"), hx("#5a7a3a")}, hx("#ffffff"), 30, 0.04)
		z.grass(z.H*0.4, z.H*1.02, 2500, z.S*0.05, z.S*0.12, leafY...)
		z.leaves(z.W*0.5, z.H*0.85, z.W*0.55, z.H*0.15, 16, z.S*0.25, leafG...)
		one(z.W*0.5, z.H*0.58, z.S*0.36, hx("#e8801a"))
		z.leaves(z.W*0.15, z.H*0.9, z.W*0.15, z.H*0.1, 5, z.S*0.2, leafG...)
	case 1:
		z.bokeh([]RGB{hx("#c8b080"), hx("#a09060"), hx("#e8dcb8")}, hx("#ffffff"), 30, 0.04)
		z.ground(func(float64) float64 { return z.H * 0.45 }, hx("#d8c080"), hx("#b09050"), z.S*0.03)
		for k := 0; k < 1500; k++ {
			x, y := z.rf(0, z.W), z.rf(z.H*0.45, z.H)
			a := z.rf(-0.5, 0.5)
			z.fill(Capsule{x, y, x + math.Cos(a)*z.S*0.06, y + math.Sin(a)*z.S*0.06, 1.5, 1}, Solid(z.jit(hx("#e0c878"), 0.15)))
		}
		for i, p := range [][3]float64{{0.2, 0.55, 0.14}, {0.78, 0.52, 0.12}, {0.45, 0.6, 0.2}, {0.7, 0.78, 0.16}, {0.25, 0.82, 0.13}} {
			c := []RGB{hx("#e87a18"), hx("#9aa8a8"), hx("#e89020"), hx("#5a7a3a"), hx("#f0b030")}[i]
			one(z.W*p[0], z.H*p[1], z.S*p[2], c)
		}
	}
	z.finish(0.3, 0.35)
}

func (z *Z) pepper(x, y, r float64, col RGB) {
	for _, k := range []float64{-0.45, 0.45, 0} {
		z.fill(Ellipse{x + k*r, y, r * 0.55, r, k * 0.15}, Shade(x+k*r, y, r*0.6, r, 0, col.Mul(1-math.Abs(k)*0.3), 0.8))
	}
	z.fill(NewLine(r*0.12, [2]float64{x, y - r*0.85}, [2]float64{x + r*0.1, y - r*1.2}, [2]float64{x + r*0.3, y - r*1.3}), Solid(hx("#3a6a22")))
}

func peppers(z *Z, v int) {
	switch v {
	case 0:
		z.bokeh([]RGB{hx("#a8c88a"), hx("#e0ecd0"), hx("#6a9a4a")}, hx("#ffffff"), 35, 0.035)
		z.leaves(z.W*0.5, z.H*0.45, z.W*0.55, z.H*0.45, 90, z.S*0.16, leafG...)
		for k := 0; k < 6; k++ {
			c := z.pick(hx("#2f7a22"), hx("#c82018"), hx("#3a8a2a"))
			z.pepper(z.rf(0.2, 0.8)*z.W, z.rf(0.3, 0.85)*z.H, z.S*z.rf(0.08, 0.11), c)
		}
		for k := 0; k < 60; k++ {
			x, y := z.rf(0.1, 0.9)*z.W, z.rf(0.1, 0.9)*z.H
			z.solid(Ellipse{x, y, 3, 2, z.rf(0, 3)}, hx("#b8d050"))
		}
	case 1:
		z.planks(0, hx("#7a5a3a"), 6)
		z.pile(z.W*0.5, z.H*0.5, z.W*0.4, z.H*0.35, 18, func(x, y float64, i int) {
			z.shadow(x, y+z.S*0.09, z.S*0.1, z.S*0.02, 0.4)
			z.pepper(x, y, z.S*z.rf(0.08, 0.1), z.jit(z.pick(hx("#c82018"), hx("#d83018"), hx("#e8a018")), 0.06))
		})
	}
	z.finish(0.3, 0.35)
}

func zucchini(z *Z, v int) {
	zc := func(x, y, l, a, r float64, col RGB) {
		x1, y1 := x-math.Cos(a)*l/2, y-math.Sin(a)*l/2
		x2, y2 := x+math.Cos(a)*l/2, y+math.Sin(a)*l/2
		z.shadow(x, y+r, l*0.5, r*0.4, 0.4)
		z.fill(Capsule{x1, y1, x2, y2, r * 0.8, r}, func(px, py float64) RGB {
			c := col.Mix(col.Mul(1.4), smooth(fbm(px/(r*0.3), py/(r*0.3), 2, 5)*1.8-0.6)*0.4)
			d := ((px-x1)*math.Sin(a) - (py-y1)*math.Cos(a)) / r
			return c.Mul(0.6 + 0.5*smooth(0.8+d*0.6))
		})
		z.fill(Capsule{x1, y1, x1 - math.Cos(a)*r*0.6, y1 - math.Sin(a)*r*0.6, r * 0.25, r * 0.2}, Solid(hx("#6a7a3a")))
	}
	switch v {
	case 0:
		z.planks(0, hx("#9a7a58"), 5)
		z.pile(z.W*0.5, z.H*0.5, z.W*0.35, z.H*0.3, 9, func(x, y float64, i int) {
			col := hx("#2a5a1e")
			if i%3 == 1 {
				col = hx("#e8c830")
			}
			zc(x, y, z.S*z.rf(0.4, 0.55), z.rf(-0.6, 0.6), z.S*0.07, col)
		})
	case 1:
		z.ground(func(float64) float64 { return 0 }, hx("#c8b070"), hx("#8a7040"), z.S*0.03)
		zc(z.W*0.35, z.H*0.75, z.S*0.6, 0.3, z.S*0.08, hx("#3a6a24"))
		zc(z.W*0.6, z.H*0.82, z.S*0.5, -0.2, z.S*0.07, hx("#2a5a1e"))
		for k := 0; k < 14; k++ {
			x, y := z.rf(0.1, 0.9)*z.W, z.rf(0, 0.6)*z.H
			z.fill(Leaf(x, y+z.S*0.2, z.S*0.4, z.S*0.4, -math.Pi/2+z.rf(-0.8, 0.8)), Solid(z.jit(hx("#3a7a2a"), 0.15)))
		}
		for k := 0; k < 3; k++ {
			x, y := z.rf(0.2, 0.8)*z.W, z.rf(0.2, 0.5)*z.H
			z.bloom(x, y, z.S*0.08, hx("#f4b820"), 1, true)
		}
	}
	z.finish(0.3, 0.3)
}

func (z *Z) sunflower(x, y, r, tilt float64, fresh bool) {
	petal, disc := hx("#f4c418"), hx("#4a2a12")
	if !fresh {
		petal, disc = hx("#b89030"), hx("#3a2a18")
	}
	for k := 0; k < 26; k++ {
		a := float64(k)/26*2*math.Pi + z.rf(-0.05, 0.05)
		px, py := x+math.Cos(a)*r*0.9, y+math.Sin(a)*r*0.9*tilt
		z.fill(Leaf(px-math.Cos(a)*r*0.2, py-math.Sin(a)*r*0.2*tilt, r*z.rf(0.55, 0.7), r*0.22, math.Atan2(math.Sin(a)*tilt, math.Cos(a))), Solid(z.jit(petal, 0.08)))
	}
	z.fill(Ellipse{x, y, r, r * tilt, 0}, Solid(disc))
	for k := 0; k < 300; k++ {
		t := math.Sqrt(float64(k) / 300)
		a := float64(k) * 2.39996
		z.solid(Circle{x + math.Cos(a)*r*0.95*t, y + math.Sin(a)*r*0.95*t*tilt, r * 0.035}, disc.Mul(1.5+0.4*math.Sin(float64(k))))
	}
}

func sunflowers(z *Z, v int) {
	if v == 0 {
		z.vgrad(hx("#3a7ac8"), hx("#a8d0f0"))
		z.cloud(z.W*0.7, z.H*0.12, z.S*0.5, hx("#ffffff"), 0.9)
		for _, f := range [][3]float64{{0.3, 0.3, 0.17}, {0.7, 0.22, 0.14}, {0.5, 0.48, 0.12}} {
			x, y := z.W*f[0], z.H*f[1]
			z.fill(Capsule{x, y, x + z.S*0.02, z.H, z.S * 0.02, z.S * 0.025}, Solid(hx("#4a7a2a")))
			for l := 0; l < 3; l++ {
				ly := y + z.S*(0.3+0.25*float64(l))
				z.fill(Leaf(x, ly, z.S*0.25, z.S*0.2, math.Pi*float64(l%2)+z.rf(-0.3, 0.3)), Solid(z.jit(hx("#3a7a2a"), 0.1)))
			}
			z.sunflower(x, y, z.S*f[2], 1, true)
		}
		z.fence(z.H*1.01, z.H*0.28, hx("#8a9aa8"), z.S*0.08)
	} else {
		z.vgrad(hx("#8a98a8"), hx("#d8d8d0"))
		z.treeline(z.H*0.6, z.S*0.1, hx("#4a5a40"), z.seed)
		z.ground(func(float64) float64 { return z.H * 0.6 }, hx("#8a8a50"), hx("#6a6a38"), z.S*0.05)
		for k := 0; k < 7; k++ {
			x := z.W * (0.08 + 0.14*float64(k))
			y := z.H * z.rf(0.28, 0.4)
			z.fill(Capsule{x, y, x + z.rf(-0.02, 0.02)*z.S, z.H, z.S * 0.015, z.S * 0.02}, Solid(hx("#6a7a3a")))
			z.fill(Leaf(x, y+z.S*0.3, z.S*0.2, z.S*0.14, math.Pi*float64(k%2)+0.4), Solid(hx("#8a8a40")))
			z.sunflower(x, y+z.S*0.08, z.S*0.12, 0.45, false)
		}
	}
	z.finish(0.2, 0.3)
}

func carrots(z *Z, v int) {
	z.bokeh(leafG, hx("#fffbe0"), 30, 0.04)
	z.grass(z.H*0.3, z.H*1.05, 2500, z.S*0.04, z.S*0.1, leafG...)
	for k := 0; k < 8; k++ {
		x := z.W*0.5 + z.rf(-0.3, 0.3)*z.S
		y := z.H*0.6 + z.rf(-0.1, 0.15)*z.S
		a := z.rf(-0.4, 0.4) + math.Pi/2*0.2
		l := z.S * z.rf(0.3, 0.42)
		ex, ey := x+math.Cos(a)*l, y+math.Sin(a)*l*0.4
		for f := 0; f < 12; f++ {
			fa := a + math.Pi + z.rf(-0.7, 0.7)
			fl := z.S * z.rf(0.15, 0.28)
			z.fill(Capsule{x, y, x + math.Cos(fa)*fl, y + math.Sin(fa)*fl - z.S*0.05, 2.5, 1.5}, Solid(z.jit(hx("#4a8a2a"), 0.15)))
		}
		z.fill(Capsule{x, y, ex, ey, z.S * 0.045, z.S * 0.006}, Shade(x+(ex-x)/2, y+(ey-y)/2, l*0.6, z.S*0.05, math.Atan2(ey-y, ex-x), hx("#f07818"), 0.2))
		for s := 0; s < 6; s++ {
			t := z.rf(0.15, 0.85)
			px, py := x+(ex-x)*t, y+(ey-y)*t
			w := z.S * 0.045 * (1 - t) * 0.8
			z.fillA(Capsule{px - math.Sin(a)*w, py + math.Cos(a)*w, px + math.Sin(a)*w, py - math.Cos(a)*w, 1.2, 1.2}, Solid(hx("#a04a10")), 0.5, 1)
		}
	}
	z.finish(0.3, 0.3)
}

func roses(z *Z, v int) {
	z.bokeh([]RGB{hx("#5a6a50"), hx("#8a8a78"), hx("#3a4a30")}, hx("#ffffff"), 30, 0.04)
	z.ground(func(float64) float64 { return z.H * 0.75 }, hx("#4a3a2a"), hx("#2e2418"), z.S*0.04)
	z.leaves(z.W*0.45, z.H*0.5, z.W*0.35, z.H*0.3, 120, z.S*0.08, leafDk...)
	for k := 0; k < 6; k++ {
		x, y := z.W*0.45+z.rf(-0.3, 0.3)*z.S, z.H*0.45+z.rf(-0.2, 0.2)*z.S
		z.bloom(x, y, z.S*z.rf(0.06, 0.09), z.pick(hx("#c81838"), hx("#e05070"), hx("#f0e0d8")), 4, false)
	}
	// Укрывной спанбонд наполовину накинут.
	z.fillA(Blob(z.r, z.W*0.85, z.H*0.55, z.S*0.4, z.S*0.35, 0.3), func(x, y float64) RGB {
		return hx("#f4f4f0").Mix(hx("#c8c8c0"), fbm(x/60, y/20, 3, 8))
	}, 0.85, 4)
	for k := 0; k < 6; k++ {
		x := z.W*0.7 + z.rf(0, 0.3)*z.W
		z.fillA(NewLine(3, [2]float64{x, z.H * 0.3}, [2]float64{x + z.rf(-0.05, 0.05)*z.S, z.H * 0.8}), Solid(hx("#b8b8b0")), 0.5, 3)
	}
	z.finish(0.1, 0.35)
}

func (z *Z) mushroom(x, y, h float64, cap RGB) {
	w := h * 0.28
	z.shadow(x, y, h*0.35, h*0.06, 0.45)
	z.fill(Capsule{x, y - w*0.3, x + h*0.03, y - h*0.8, w, w * 0.65}, func(px, py float64) RGB {
		c := hx("#ece4d4")
		if fbm(px/4, py/10, 2, 2) > 0.72 {
			c = hx("#4a4038")
		}
		return c.Mul(0.8 + 0.3*clamp((px-x+w)/(2*w), 0, 1))
	})
	cy := y - h*0.78
	capS := And{Ellipse{x, cy, h * 0.5, h * 0.35, 0}, Box{x, cy - h*0.25, h * 1.2, h * 0.5, 0, 0}}
	z.fill(capS, Shade(x, cy, h*0.55, h*0.38, 0, cap, 0.3))
	z.fill(Ellipse{x, cy, h * 0.48, h * 0.05, 0}, Solid(hx("#d8c8a0")))
}

func mushrooms(z *Z, v int) {
	switch v {
	case 0:
		z.bokeh([]RGB{hx("#4a5a30"), hx("#6a7a40"), hx("#2e3a20")}, hx("#fff8e0"), 30, 0.04)
		z.ground(func(float64) float64 { return z.H * 0.55 }, hx("#4a6a28"), hx("#2e4a1e"), z.S*0.03)
		for k := 0; k < 120; k++ {
			x, y := z.rf(0, z.W), z.rf(z.H*0.55, z.H)
			z.fill(Leaf(x, y, z.S*0.05, z.S*0.025, z.rf(0, 6.3)), Solid(z.jit(z.pick(hx("#d8a020"), hx("#c86018"), hx("#a88a30")), 0.15)))
		}
		for _, m := range [][3]float64{{0.35, 0.78, 0.36}, {0.55, 0.85, 0.44}, {0.72, 0.72, 0.26}, {0.2, 0.9, 0.2}} {
			z.mushroom(z.W*m[0], z.H*m[1], z.S*m[2], z.jit(hx("#8a5028"), 0.1))
		}
		z.grass(z.H*0.6, z.H*1.02, 900, z.S*0.03, z.S*0.08, leafY...)
	case 1:
		z.planks(0, hx("#6a5038"), 5)
		z.basket(z.W*0.5, z.H*0.6, z.S*0.9, z.S*0.55, hx("#a88048"), func() {
			z.pile(z.W*0.5, z.H*0.6-z.S*0.2, z.S*0.36, z.S*0.08, 14, func(x, y float64, i int) {
				c := z.jit(z.pick(hx("#8a5028"), hx("#6a3a1a"), hx("#c07030")), 0.1)
				z.sphere(x, y, z.S*0.08, z.S*0.05, z.rf(-0.3, 0.3), c, 0.3)
			})
		})
		for _, x := range []float64{0.12, 0.88} {
			z.mushroom(z.W*x, z.H*0.95, z.S*0.22, hx("#7a4a24"))
		}
	}
	z.finish(0.25, 0.4)
}

func cat(z *Z, v int) {
	z.vplanks(0, z.W, 0, z.H*0.55, hx("#8aa0a8"), z.S*0.1)
	z.solid(Box{z.W * 0.8, z.H * 0.28, z.S * 0.38, z.H * 0.56, 0, 0}, hx("#5a4030"))
	z.solid(Box{z.W * 0.8, z.H * 0.28, z.S * 0.32, z.H * 0.5, 0, 0}, hx("#7a5a40"))
	// Крыльцо уходит вглубь.
	for k := 0; k < 7; k++ {
		y0 := z.H*0.55 + z.H*0.45*math.Pow(float64(k)/7, 1.3)
		y1 := z.H*0.55 + z.H*0.45*math.Pow(float64(k+1)/7, 1.3)
		c := z.jit(hx("#b08a5a"), 0.08)
		z.fill(Box{z.W / 2, (y0 + y1) / 2, z.W * 1.1, y1 - y0 - 2, 0, 0}, Tex(c, c.Mul(0.75), z.S*0.3, z.S*0.01, 3, k))
	}
	z.glow(z.W*0.4, z.H*0.7, z.S*0.35, hx("#ffd890"), 0.25)
	cx, cy := z.W*0.45, z.H*0.72
	fur, belly := hx("#d8843a"), hx("#f4dcb8")
	z.shadow(cx+z.S*0.05, cy+z.S*0.17, z.S*0.28, z.S*0.05, 0.5)
	z.fill(Capsule{cx + z.S*0.15, cy + z.S*0.14, cx + z.S*0.32, cy + z.S*0.02, z.S * 0.035, z.S * 0.03}, Solid(fur.Mul(0.85)))
	z.fill(Ellipse{cx, cy, z.S * 0.2, z.S * 0.2, 0}, Shade(cx, cy, z.S*0.22, z.S*0.22, 0, fur, 0.1))
	z.fill(Ellipse{cx, cy + z.S*0.03, z.S * 0.1, z.S * 0.15, 0}, Solid(belly))
	for s := 0; s < 5; s++ {
		yy := cy - z.S*0.1 + float64(s)*z.S*0.05
		z.fillA(NewLine(z.S*0.015, [2]float64{cx - z.S*0.19, yy}, [2]float64{cx - z.S*0.12, yy + z.S*0.01}), Solid(fur.Mul(0.6)), 0.8, 2)
		z.fillA(NewLine(z.S*0.015, [2]float64{cx + z.S*0.19, yy}, [2]float64{cx + z.S*0.12, yy + z.S*0.01}), Solid(fur.Mul(0.6)), 0.8, 2)
	}
	hx0, hy := cx, cy-z.S*0.25
	for _, s := range []float64{-1, 1} {
		ear := NewPoly([][2]float64{{hx0 + s*z.S*0.03, hy - z.S*0.06}, {hx0 + s*z.S*0.12, hy - z.S*0.17}, {hx0 + s*z.S*0.13, hy - z.S*0.02}})
		z.fill(ear, Solid(fur.Mul(0.9)))
		z.fill(NewPoly([][2]float64{{hx0 + s*z.S*0.06, hy - z.S*0.06}, {hx0 + s*z.S*0.115, hy - z.S*0.13}, {hx0 + s*z.S*0.115, hy - z.S*0.05}}), Solid(hx("#e8a0a0")))
	}
	z.fill(Ellipse{hx0, hy, z.S * 0.13, z.S * 0.11, 0}, Shade(hx0, hy, z.S*0.14, z.S*0.12, 0, fur, 0.1))
	z.fill(Ellipse{hx0, hy + z.S*0.05, z.S * 0.06, z.S * 0.04, 0}, Solid(belly))
	for _, s := range []float64{-1, 1} {
		ex := hx0 + s*z.S*0.05
		z.solid(Ellipse{ex, hy - z.S*0.01, z.S * 0.022, z.S * 0.018, 0}, hx("#9ab830"))
		z.solid(Ellipse{ex, hy - z.S*0.01, z.S * 0.005, z.S * 0.016, 0}, hx("#101010"))
		z.solid(Circle{ex + z.S*0.006, hy - z.S*0.018, z.S * 0.004}, hx("#ffffff"))
		for w := -1.0; w <= 1; w++ {
			z.fillA(NewLine(1.5, [2]float64{hx0 + s*z.S*0.04, hy + z.S*0.045}, [2]float64{hx0 + s*z.S*0.16, hy + z.S*0.03 + w*z.S*0.02}), Solid(hx("#f8f0e0")), 0.8, 1)
		}
	}
	z.solid(NewPoly([][2]float64{{hx0 - z.S*0.012, hy + z.S*0.03}, {hx0 + z.S*0.012, hy + z.S*0.03}, {hx0, hy + z.S*0.045}}), hx("#d87078"))
	z.fill(Ellipse{z.W * 0.8, z.H * 0.9, z.S * 0.1, z.S * 0.035, 0}, Solid(hx("#d0d0d8")))
	z.finish(0.35, 0.35)
}

func herbs(z *Z, v int) {
	z.vplanks(0, z.W, 0, z.H, hx("#6a4a30"), z.S*0.14)
	z.glow(z.W*0.3, z.H*0.2, z.S*0.6, hx("#ffc070"), 0.25)
	z.fill(Box{z.W / 2, z.H * 0.08, z.W * 1.1, z.S * 0.08, 0, 0}, Tex(hx("#8a6440"), hx("#5a4028"), z.S*0.3, z.S*0.02, 3, 4))
	for k := 0; k < 5; k++ {
		x := z.W * (0.12 + 0.19*float64(k))
		ty := z.H * z.rf(0.2, 0.3)
		z.fill(NewLine(2, [2]float64{x, z.H * 0.1}, [2]float64{x, ty}), Solid(hx("#d8c8a0")))
		col := z.pick(hx("#6a8a50"), hx("#7a9a6a"), hx("#5a7a40"))
		l := z.H * z.rf(0.35, 0.5)
		for s := 0; s < 14; s++ {
			a := math.Pi/2 + z.rf(-0.25, 0.25)
			ex, ey := x+math.Cos(a)*l, ty+math.Sin(a)*l
			z.fill(Capsule{x, ty, ex, ey, 1.5, 1}, Solid(hx("#6a5a3a")))
			for f := 0; f < 7; f++ {
				t := z.rf(0.2, 1)
				px, py := x+(ex-x)*t, ty+(ey-ty)*t
				z.fill(Leaf(px, py, z.S*0.035, z.S*0.018, a+z.rf(-1.2, 1.2)), Solid(z.jit(col, 0.12)))
			}
			if k%2 == 1 {
				z.solid(Circle{ex, ey, z.S * 0.012}, hx("#b890c0"))
			}
		}
		z.solid(Box{x, ty, z.S * 0.03, z.S * 0.02, 0, 3}, hx("#c83030"))
	}
	z.finish(0.4, 0.4)
}

func beets(z *Z, v int) {
	z.ground(func(float64) float64 { return 0 }, hx("#5a4030"), hx("#3a281a"), z.S*0.05)
	z.pile(z.W*0.5, z.H*0.55, z.W*0.42, z.H*0.35, 9, func(x, y float64, i int) {
		for l := 0; l < 6; l++ {
			a := -math.Pi/2 + z.rf(-1.2, 1.2)
			ll := z.S * z.rf(0.2, 0.3)
			c := hx("#3a6a2a")
			if l == 0 {
				c = hx("#b8a040")
			}
			lf := Leaf(x, y-z.S*0.04, ll, ll*0.5, a)
			z.fill(lf, Solid(z.jit(c, 0.12)))
			z.fill(Capsule{x, y - z.S*0.04, x + math.Cos(a)*ll*0.9, y - z.S*0.04 + math.Sin(a)*ll*0.9, 2.5, 1}, Solid(hx("#a01a3a")))
		}
		r := z.S * z.rf(0.06, 0.08)
		z.sphere(x, y, r, r*0.95, 0, hx("#6a1030"), 0.3)
		z.fill(Capsule{x, y + r*0.9, x + z.rf(-0.02, 0.02)*z.S, y + r*1.8, r * 0.1, 1}, Solid(hx("#6a1030")))
	})
	z.finish(0.2, 0.3)
}

func cabbage(z *Z, v int) {
	z.ground(func(float64) float64 { return 0 }, hx("#5a4432"), hx("#3a2a1c"), z.S*0.05)
	if v == 0 {
		z.pile(z.W*0.5, z.H*0.5, z.W*0.45, z.H*0.4, 7, func(x, y float64, i int) {
			r := z.S * z.rf(0.13, 0.16)
			for l := 0; l < 9; l++ {
				a := float64(l)/9*2*math.Pi + z.rf(-0.2, 0.2)
				z.fill(Leaf(x, y, r*2.1, r*1.5, a), func(px, py float64) RGB {
					return hx("#6a9a7a").Mix(hx("#3a6a4a"), clamp(math.Hypot(px-x, py-y)/(r*2), 0, 1))
				})
			}
			z.sphere(x, y, r, r*0.95, 0, hx("#b8d890"), 0.25)
			for l := 0; l < 3; l++ {
				a := z.rf(0, 6.3)
				z.fillA(Ring{Ellipse{x + math.Cos(a)*r*0.4, y + math.Sin(a)*r*0.4, r * 0.8, r * 0.7, a}, 2}, Solid(hx("#8ab870")), 0.6, 2)
			}
		})
	} else {
		for row := 0; row < 3; row++ {
			for k := 0; k < 5; k++ {
				x := z.W * (0.1 + 0.2*float64(k) + z.rf(-0.02, 0.02))
				y := z.H * (0.2 + 0.3*float64(row))
				for l := 0; l < 6; l++ {
					a := float64(l)/6*2*math.Pi + z.rf(0, 0.5)
					z.fill(Ellipse{x + math.Cos(a)*z.S*0.05, y + math.Sin(a)*z.S*0.04, z.S * 0.055, z.S * 0.04, a}, Solid(z.jit(hx("#6a9a6a"), 0.1)))
				}
			}
		}
	}
	z.finish(0.2, 0.3)
}

func currants(z *Z, v int) {
	z.bokeh(leafG, hx("#fffbe8"), 35, 0.04)
	z.leaves(z.W*0.5, z.H*0.45, z.W*0.6, z.H*0.5, 50, z.S*0.2, leafG...)
	for k := 0; k < 8; k++ {
		x, y := z.rf(0.1, 0.75)*z.W, z.rf(0.15, 0.6)*z.H
		z.fill(NewLine(z.S*0.005, [2]float64{x, y}, [2]float64{x + z.S*0.05, y + z.S*0.2}), Solid(hx("#6a7a3a")))
		for b := 0; b < 8; b++ {
			t := float64(b) / 8
			z.sphere(x+z.S*0.05*t+z.rf(-0.02, 0.02)*z.S, y+z.S*0.2*t+z.S*0.02, z.S*0.028, z.S*0.028, 0, z.jit(hx("#1a1024"), 0.15), 1)
		}
	}
	cx, cy := z.W*0.82, z.H*0.78
	z.fill(Ellipse{cx, cy, z.S * 0.2, z.S * 0.07, 0}, Solid(hx("#1a1020")))
	z.fill(And{Cut{NewPoly([][2]float64{{cx - z.S*0.2, cy}, {cx + z.S*0.2, cy}, {cx + z.S*0.16, z.H + 20}, {cx - z.S*0.16, z.H + 20}}), Ellipse{cx, cy, z.S * 0.2, z.S * 0.07, 0}}, Box{cx, cy + z.S*0.3, z.S, z.S * 0.6, 0, 0}}, Lin(cx-z.S*0.2, 0, cx+z.S*0.2, 0, hx("#f0f0f0"), hx("#a8a8b0")))
	for b := 0; b < 25; b++ {
		z.sphere(cx+z.rf(-0.17, 0.17)*z.S, cy+z.rf(-0.04, 0.03)*z.S, z.S*0.022, z.S*0.022, 0, hx("#1a1024"), 1)
	}
	z.finish(0.2, 0.35)
}

func cherries(z *Z, v int) {
	z.bokeh([]RGB{hx("#7aa8d8"), hx("#a8c8e8"), hx("#5a8a4a")}, hx("#ffffff"), 30, 0.04)
	z.fill(NewLine(z.S*0.03, [2]float64{-10, z.H * 0.15}, [2]float64{z.W * 0.5, z.H * 0.2}, [2]float64{z.W + 10, z.H * 0.1}), Solid(hx("#4a3024")))
	z.leaves(z.W*0.5, z.H*0.2, z.W*0.5, z.H*0.12, 40, z.S*0.2, leafG...)
	for k := 0; k < 9; k++ {
		x := z.rf(0.1, 0.9) * z.W
		top := z.H * 0.18
		for p := -1.0; p <= 1; p += 2 {
			ex, ey := x+p*z.S*0.05+z.rf(-0.02, 0.02)*z.S, z.H*z.rf(0.4, 0.75)
			z.fill(NewLine(z.S*0.006, [2]float64{x, top}, [2]float64{(x + ex) / 2, (top+ey)/2 - z.S*0.02}, [2]float64{ex, ey}), Solid(hx("#6a8a3a")))
			z.sphere(ex, ey+z.S*0.045, z.S*0.055, z.S*0.05, 0, z.jit(hx("#a0101c"), 0.1), 1.1)
		}
	}
	z.finish(0.2, 0.3)
}

func peonies(z *Z, v int) {
	z.bokeh(append(leafDk, hx("#6a8a40")), hx("#fff8f0"), 35, 0.04)
	if v == 0 {
		z.leaves(z.W*0.5, z.H*0.65, z.W*0.6, z.H*0.4, 90, z.S*0.16, leafG...)
		for k := 0; k < 7; k++ {
			x, y := z.rf(0.12, 0.88)*z.W, z.rf(0.25, 0.75)*z.H
			z.bloom(x, y, z.S*z.rf(0.1, 0.14), z.pick(hx("#e880a8"), hx("#f4b8c8"), hx("#c83060")), 5, false)
		}
		for k := 0; k < 4; k++ {
			z.sphere(z.rf(0.1, 0.9)*z.W, z.rf(0.3, 0.8)*z.H, z.S*0.04, z.S*0.04, 0, hx("#c05878"), 0.4)
		}
	} else {
		z.leaves(z.W*0.5, z.H*0.85, z.W*0.6, z.H*0.2, 30, z.S*0.2, leafG...)
		z.bloom(z.W*0.5, z.H*0.48, z.S*0.36, hx("#f6d8e0"), 7, false)
	}
	z.finish(0.25, 0.35)
}

func blossom(z *Z, v int) {
	switch v {
	case 0, 1:
		z.bokeh([]RGB{hx("#6aa0e0"), hx("#a8d0f4"), hx("#d8e8f8")}, hx("#ffffff"), 25, 0.04)
		var branch func(x, y, a, l, w float64, d int)
		branch = func(x, y, a, l, w float64, d int) {
			ex, ey := x+math.Cos(a)*l, y+math.Sin(a)*l
			z.fill(Capsule{x, y, ex, ey, w, w * 0.7}, Solid(hx("#4a3428")))
			if d == 0 {
				return
			}
			branch(ex, ey, a+z.rf(0.2, 0.6), l*0.7, w*0.7, d-1)
			branch(ex, ey, a-z.rf(0.2, 0.6), l*0.7, w*0.7, d-1)
		}
		size := z.S * 0.04
		if v == 1 {
			size = z.S * 0.09
			branch(-10, z.H*0.7, -0.5, z.S*0.5, z.S*0.03, 3)
		} else {
			branch(-10, z.H*0.9, -0.6, z.S*0.45, z.S*0.035, 5)
			branch(z.W+10, z.H*0.8, -2.5, z.S*0.4, z.S*0.03, 5)
		}
		n := 140
		if v == 1 {
			n = 30
		}
		z.leaves(z.W*0.5, z.H*0.45, z.W*0.5, z.H*0.4, n/3, size*1.3, hx("#8ac850"), hx("#a0d860"))
		for k := 0; k < n; k++ {
			x, y := z.rf(0.05, 0.95)*z.W, z.rf(0.08, 0.85)*z.H
			z.flower(x, y, size, 5, z.pick(hx("#fbf4f4"), hx("#f8dce4"), hx("#f4c8d4")), hx("#e8d060"), 0.42)
		}
	case 2: // липа
		z.bokeh([]RGB{hx("#7a9a4a"), hx("#b8c880"), hx("#4a6a30")}, hx("#fffbe0"), 35, 0.04)
		z.leaves(z.W*0.5, z.H*0.5, z.W*0.6, z.H*0.6, 70, z.S*0.2, leafG...)
		for k := 0; k < 12; k++ {
			x, y := z.rf(0.1, 0.9)*z.W, z.rf(0.1, 0.8)*z.H
			z.fill(Leaf(x, y, z.S*0.16, z.S*0.05, z.rf(0, 6.3)), Solid(hx("#d8e0a0")))
			for f := 0; f < 7; f++ {
				z.flower(x+z.rf(-0.05, 0.05)*z.S, y+z.rf(0.02, 0.1)*z.S, z.S*0.022, 5, hx("#f0e890"), hx("#c8b040"), 0.45)
			}
		}
		bx, by := z.W*0.62, z.H*0.45
		z.fillA(Ellipse{bx - z.S*0.01, by - z.S*0.03, z.S * 0.025, z.S * 0.015, -0.4}, Solid(RGB{1, 1, 1}), 0.6, 2)
		z.fill(Ellipse{bx, by, z.S * 0.03, z.S * 0.018, 0.2}, func(x, y float64) RGB {
			if int((x-bx)/(z.S*0.012)+10)%2 == 0 {
				return hx("#1a1a10")
			}
			return hx("#e8b020")
		})
	}
	z.finish(0.15, 0.3)
}

func tulips(z *Z, v int) {
	z.vgrad(hx("#d8e4ec"), hx("#c8d0c8"))
	z.vplanks(0, z.W*0.3, 0, z.H*0.7, hx("#a88a60"), z.S*0.1)
	for k := 0; k < 4; k++ {
		y := z.H * (0.55 + 0.05*float64(k))
		z.solid(Box{z.W * 0.15, y, z.W * 0.35, z.S * 0.04, 0, 0}, hx("#8a6a48").Mul(1-0.08*float64(k)))
	}
	z.ground(func(float64) float64 { return z.H * 0.72 }, hx("#4a3424"), hx("#2e2016"), z.S*0.04)
	type tl struct{ x, y float64 }
	ts := []tl{}
	for k := 0; k < 16; k++ {
		ts = append(ts, tl{z.rf(0.15, 0.95) * z.W, z.rf(0.35, 0.7) * z.H})
	}
	sort.Slice(ts, func(i, j int) bool { return ts[i].y < ts[j].y })
	for _, t := range ts {
		z.fill(Capsule{t.x, t.y, t.x + z.rf(-0.02, 0.02)*z.S, z.H * 0.85, z.S * 0.007, z.S * 0.009}, Solid(hx("#4a7a3a")))
		z.fill(Leaf(t.x, z.H*0.85, z.S*0.25, z.S*0.07, -math.Pi/2+z.rf(-0.5, 0.5)), Solid(hx("#5a8a5a")))
		c := z.pick(hx("#d81818"), hx("#f0c020"), hx("#e84060"))
		r := z.S * 0.05
		for _, k := range []float64{-0.5, 0.5, 0} {
			z.fill(Ellipse{t.x + k*r*0.8, t.y - r*0.8, r * 0.6, r * 1.1, k * 0.25}, Shade(t.x+k*r*0.8, t.y-r*0.8, r*0.7, r*1.1, 0, c.Mul(1-math.Abs(k)*0.25), 0.3))
		}
	}
	z.grass(z.H*0.8, z.H*1.03, 700, z.S*0.03, z.S*0.08, leafG...)
	z.finish(0.2, 0.3)
}

func seedlings(z *Z, v int) {
	// Окно: светлое небо за стеклом, рама, подоконник.
	z.bokeh([]RGB{hx("#d8e4f0"), hx("#f4f8fc"), hx("#b8c8d8")}, hx("#ffffff"), 20, 0.05)
	if v == 0 {
		for _, x := range []float64{0.02, 0.5, 0.98} {
			z.solid(Box{z.W * x, z.H * 0.35, z.S * 0.05, z.H * 0.8, 0, 0}, hx("#f4f2ec"))
		}
		z.solid(Box{z.W / 2, z.H * 0.33, z.W, z.S * 0.04, 0, 0}, hx("#f4f2ec"))
	}
	sill := z.H * 0.72
	if v == 1 {
		sill = z.H * 0.6
	}
	z.fill(Box{z.W / 2, (sill + z.H) / 2, z.W * 1.1, z.H - sill, 0, 0}, Lin(0, sill, 0, z.H, hx("#f0eee8"), hx("#c8c4bc")))
	n, w := 6, z.S*0.13
	if v == 1 {
		n, w = 3, z.S*0.26
	}
	for k := 0; k < n; k++ {
		x := z.W * (float64(k) + 0.5) / float64(n)
		y := sill + w*0.2
		h := w * 1.1
		z.shadow(x, y+w*0.05, w*0.5, w*0.08, 0.3)
		body := NewPoly([][2]float64{{x - w/2, y - h}, {x + w/2, y - h}, {x + w*0.38, y}, {x - w*0.38, y}})
		cup := z.pick(hx("#e8e8e4"), hx("#8a6a4a"))
		z.fill(body, Lin(x-w/2, 0, x+w/2, 0, cup, cup.Mul(0.75)))
		z.fill(Ellipse{x, y - h, w / 2, w * 0.12, 0}, Solid(hx("#3a2a1e")))
		for s := 0; s < 2+k%2; s++ {
			sx := x + z.rf(-0.2, 0.2)*w
			top := y - h - w*z.rf(0.6, 1.2)
			z.fill(Capsule{sx, y - h, sx + z.rf(-0.1, 0.1)*w, top, w * 0.025, w * 0.02}, Solid(hx("#8ab860")))
			z.fill(Leaf(sx, top, w*0.4, w*0.18, -0.4), Solid(hx("#6aa840")))
			z.fill(Leaf(sx, top, w*0.4, w*0.18, math.Pi+0.4), Solid(hx("#5a9a38")))
			if k%3 == 0 {
				z.fill(Leaf(sx, top, w*0.3, w*0.2, -math.Pi/2), Solid(hx("#4a8a30")))
			}
		}
	}
	z.finish(0.05, 0.25)
}
