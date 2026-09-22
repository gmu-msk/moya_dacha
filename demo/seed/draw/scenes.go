package main

// Сцены: что изображено на каждой фотографии поста. Подпись поста
// в demo/stories/000-dacha/seed.sql говорит о том же, что нарисовано.

import (
	"math"
	"sort"
)

func strawberry(z *Z, v int) {
	switch v {
	case 0: // миска клубники на столе
		z.bokeh(leafG, hx("#fff6d0"), 40, 0.03)
		z.planks(z.H*0.55, hx("#8a6440"), 4)
		cx, cy := z.W*0.5, z.H*0.62
		z.shadow(cx+z.S*0.04, cy+z.S*0.2, z.S*0.45, z.S*0.08, 0.5)
		z.fill(Ellipse{cx, cy, z.S * 0.46, z.S * 0.3, 0}, Lin(0, cy-z.S*0.3, 0, cy+z.S*0.3, hx("#f4f1ea"), hx("#b9b4a8")))
		rim := Ellipse{cx, cy - z.S*0.05, z.S * 0.46, z.S * 0.15, 0}
		front := Box{cx, cy + z.S*0.25, z.S, z.S * 0.6, 0, 0}
		z.fill(rim, Lin(0, cy-z.S*0.2, 0, cy, hx("#6a1a1a"), hx("#3a0c0c")))
		z.fill(Ring{rim, z.S * 0.012}, Solid(hx("#2d4f8a")))
		type berry struct{ x, y, s, rot float64 }
		bs := []berry{}
		for k := 0; k < 30; k++ {
			a := z.rf(0, 2*math.Pi)
			d := math.Sqrt(z.r.Float64())
			bs = append(bs, berry{cx + math.Cos(a)*d*z.S*0.36, cy - z.S*0.08 + math.Sin(a)*d*z.S*0.09 - (1-d)*z.S*0.07, z.S * z.rf(0.14, 0.19), z.rf(-1.2, 1.2) + math.Pi})
		}
		sort.Slice(bs, func(i, j int) bool { return bs[i].y < bs[j].y })
		for _, b := range bs {
			z.strawberry(b.x, b.y, b.s, b.rot, z.rf(0.85, 1))
		}
		body := Ellipse{cx, cy + z.S*0.02, z.S * 0.46, z.S * 0.3, 0}
		z.fill(And{body, front}, func(x, y float64) RGB {
			return hx("#f2efe6").Mix(hx("#a8a397"), clamp((y-cy)/(z.S*0.3), 0, 1))
		})
		z.fill(And{Ring{rim, z.S * 0.012}, front}, Solid(hx("#2d4f8a")))
		z.finish(0.4, 0.35)
	case 1: // грядка клубники с усами
		z.ground(func(float64) float64 { return 0 }, hx("#5b4632"), hx("#3a2b1e"), z.S*0.08)
		for k := 0; k < 900; k++ {
			x, y := z.rf(0, z.W), z.rf(0, z.H)
			a := z.rf(0, math.Pi)
			z.fill(Capsule{x, y, x + math.Cos(a)*z.S*0.04, y + math.Sin(a)*z.S*0.04, 2, 1.5}, Solid(z.jit(hx("#c9b27a"), 0.15)))
		}
		for row := 0; row < 3; row++ {
			for k := 0; k < 4; k++ {
				x := z.W * (0.14 + 0.24*float64(k) + z.rf(-0.03, 0.03))
				y := z.H * (0.2 + 0.33*float64(row))
				sz := z.S * 0.12 * (0.8 + 0.2*float64(row))
				for l := 0; l < 7; l++ {
					a := z.rf(0, 2*math.Pi)
					lx, ly := x+math.Cos(a)*sz*0.6, y+math.Sin(a)*sz*0.4
					for f := -1.0; f <= 1; f++ {
						z.fill(Leaf(lx, ly, sz*0.6, sz*0.45, a+f*0.6), Solid(z.jit(hx("#3f7f2c"), 0.15)))
					}
				}
				z.fill(NewLine(z.S*0.004, [2]float64{x, y}, [2]float64{x + z.S*0.1, y + z.S*0.05}, [2]float64{x + z.S*0.16, y + z.S*0.02}), Solid(hx("#8a3b2a")))
				z.leaves(x+z.S*0.16, y+z.S*0.02, z.S*0.02, z.S*0.015, 4, z.S*0.035, leafY...)
			}
		}
		z.finish(0.2, 0.3)
	case 2: // первые ягоды на кусте
		z.bokeh(append(leafG, hx("#9bbf5a")), hx("#ffffff"), 40, 0.04)
		z.leaves(z.W*0.5, z.H*0.3, z.W*0.55, z.H*0.3, 30, z.S*0.28, leafG...)
		for _, b := range [][3]float64{{0.35, 0.55, 1}, {0.55, 0.62, 0.95}, {0.7, 0.5, 0.5}, {0.45, 0.75, 0.9}} {
			x, y := z.W*b[0], z.H*b[1]
			z.fill(NewLine(z.S*0.012, [2]float64{x, 0}, [2]float64{x + z.S*0.05, y - z.S*0.3}, [2]float64{x, y - z.S*0.1}), Solid(hx("#5a7a2e")))
			z.strawberry(x, y, z.S*0.2, z.rf(-0.3, 0.3), b[2])
		}
		z.flower(z.W*0.2, z.H*0.42, z.S*0.07, 5, hx("#f8f8f2"), hx("#e8c93a"), 0.42)
		z.finish(0.3, 0.3)
	}
}

func lake(z *Z, v int) {
	hor := z.H * 0.56
	top, mid, low := hx("#2b3f73"), hx("#d8708a"), hx("#ffb55a")
	if v == 1 {
		top, mid, low = hx("#5d8ecf"), hx("#9cc2e8"), hx("#dbe8f0")
	}
	z.c.Each(func(x, y float64, _ RGB) RGB {
		t := clamp(y/hor, 0, 1)
		if t < 0.6 {
			return top.Mix(mid, t/0.6)
		}
		return mid.Mix(low, (t-0.6)/0.4)
	})
	sunX := z.W * 0.62
	if v == 0 {
		z.glow(sunX, hor-z.S*0.04, z.S*0.25, hx("#ffcc70"), 0.6)
		z.solid(Circle{sunX, hor - z.S*0.04, z.S * 0.05}, hx("#fff0c0"))
		for i := 0; i < 5; i++ {
			z.cloud(z.rf(0, 1)*z.W, z.rf(0.1, 0.4)*z.H, z.S*z.rf(0.3, 0.6), hx("#f0a0a8").Mix(hx("#503a6a"), z.r.Float64()*0.5), 0.55)
		}
	} else {
		for i := 0; i < 6; i++ {
			z.cloud(z.rf(0, 1)*z.W, z.rf(0.08, 0.35)*z.H, z.S*z.rf(0.3, 0.6), hx("#ffffff"), 0.8)
		}
	}
	tree := hx("#1c2618")
	if v == 1 {
		tree = hx("#3a5a2c")
	}
	z.treeline(hor, z.S*0.12, tree, z.seed)
	// Вода — отражение того, что выше горизонта, с рябью.
	src := append([]RGB(nil), z.c.P...)
	sd := z.seed
	z.c.Each(func(x, y float64, p RGB) RGB {
		if y < hor {
			return p
		}
		d := y - hor
		wob := (fbm(x/(z.S*0.3), y/(z.S*0.006), 3, sd) - 0.5) * z.S * 0.03 * (0.3 + d/z.H)
		my := clampi(int(hor-d*0.9), 0, z.c.H-1)
		mx := clampi(int(x+wob), 0, z.c.W-1)
		c := src[my*z.c.W+mx].Mul(0.72)
		streak := fbm(x/(z.S*0.5), y/(z.S*0.004), 2, sd+9)
		return c.Mix(c.Mul(1.3), smooth(streak*2-1)*0.5)
	})
	// Камыш на переднем плане.
	reed := hx("#15130d")
	if v == 1 {
		reed = hx("#4b5a2a")
	}
	for i := 0; i < 50; i++ {
		x := z.rf(0, 0.3) * z.W
		if i%2 == 1 {
			x = z.W - x
		}
		h := z.rf(0.15, 0.45) * z.H
		lean := z.rf(-0.1, 0.1) * h
		z.fill(Capsule{x, z.H + 5, x + lean, z.H - h, z.S * 0.005, z.S * 0.002}, Solid(reed))
		if i%5 == 0 {
			z.fill(Capsule{x + lean*0.93, z.H - h*0.93, x + lean*0.8, z.H - h*0.8, z.S * 0.012, z.S * 0.012}, Solid(reed.Mix(hx("#4a3020"), 0.5)))
		}
	}
	if v == 0 {
		z.finish(0.5, 0.45)
	} else {
		z.finish(0.1, 0.3)
	}
}

func house(z *Z, v int) {
	switch v {
	case 0: // туманное утро
		z.vgrad(hx("#c9d3d6"), hx("#e6e3d8"))
		z.treeline(z.H*0.5, z.S*0.18, hx("#7d8c80"), z.seed)
		hx0, hy := z.W*0.52, z.H*0.72
		w, h := z.S*0.62, z.S*0.36
		wall := hx("#5e7d62")
		z.vplanks(hx0-w/2, hx0+w/2, hy-h, hy, wall, z.S*0.035)
		z.fill(NewPoly([][2]float64{{hx0 - w*0.6, hy - h}, {hx0, hy - h - z.S*0.28}, {hx0 + w*0.6, hy - h}}), Lin(0, hy-h-z.S*0.28, 0, hy-h, hx("#8a3a2e"), hx("#6a2a22")))
		z.fill(NewPoly([][2]float64{{hx0 - w*0.12, hy - h - z.S*0.03}, {hx0, hy - h - z.S*0.16}, {hx0 + w*0.12, hy - h - z.S*0.03}}), Solid(hx("#d9d2bf")))
		for _, wx := range []float64{-0.25, 0.25} {
			x := hx0 + w*wx
			z.solid(Box{x, hy - h*0.55, z.S * 0.13, z.S * 0.15, 0, 0}, hx("#f2efe6"))
			z.fill(Box{x, hy - h*0.55, z.S * 0.1, z.S * 0.12, 0, 0}, Lin(0, hy-h, 0, hy, hx("#9fb2b8"), hx("#4a5a60")))
			z.solid(Box{x, hy - h*0.55, z.S * 0.1, z.S * 0.008, 0, 0}, hx("#f2efe6"))
			z.solid(Box{x, hy - h*0.55, z.S * 0.008, z.S * 0.12, 0, 0}, hx("#f2efe6"))
		}
		z.solid(Box{hx0, hy + z.S*0.01, w * 1.05, z.S * 0.03, 0, 0}, hx("#6b5a48"))
		for i, bx := range []float64{0.12, 0.86, 0.93} {
			x := z.W * bx
			z.fill(Capsule{x, z.H, x + z.rf(-0.02, 0.02)*z.S, z.H * 0.15, z.S * 0.02, z.S * 0.012}, func(px, py float64) RGB {
				if fbm(px/6, py/25, 2, i) > 0.72 {
					return hx("#2a2a26")
				}
				return hx("#ecebe4")
			})
			z.leaves(x, z.H*0.25, z.S*0.2, z.S*0.2, 140, z.S*0.05, leafY...)
		}
		z.grass(z.H*0.72, z.H*1.02, 2500, z.S*0.03, z.S*0.12, leafG...)
		z.fence(z.H*0.97, z.S*0.2, hx("#b8a882"), z.S*0.06)
		sd := z.seed
		z.c.Each(func(x, y float64, p RGB) RGB {
			f := 0.55*(1-y/z.H) + 0.25*fbm(x/(z.S*0.4), y/(z.S*0.15), 3, sd)
			return p.Mix(hx("#e8ecea"), clamp(f, 0, 0.75))
		})
		z.finish(0.15, 0.25)
	case 1:
		shed(z)
	}
}
