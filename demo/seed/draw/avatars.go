package main

// Аватары дачников, 512×512. Большинство — портреты, а трое, как бывает
// и в жизни, поставили на аватар закат, ромашку и соты.

import "math"

type face struct {
	skin, hair, shirt, bg RGB
	hairStyle             string // short, bald, long, bun, scarf, cap, panama, curly
	beard, glasses, smile bool
	age                   float64 // 0 — молодой, 1 — пожилой: морщинки, седина
}

func avatars() []Pic {
	light, tan, pale := hx("#f2cfae"), hx("#d8a67c"), hx("#f6dcc4")
	grey, dark, brown, blond, red := hx("#c8c4bc"), hx("#2a2018"), hx("#6a4428"), hx("#d8b060"), hx("#a8482a")
	p := func(name string, f face) Pic { return Pic{name, 512, 512, func(z *Z) { portrait(z, f) }} }
	o := func(name string, draw func(z *Z)) Pic { return Pic{name, 512, 512, draw} }
	return []Pic{
		p("avatar-01", face{tan, grey, hx("#3a6a9a"), hx("#8ab86a"), "bald", false, false, true, 0.8}),   // Николай
		p("avatar-02", face{light, brown, hx("#c84a5a"), hx("#a8d0e8"), "bun", false, false, true, 0.6}), // Валентина
		o("avatar-03", avatarSunset), // Пётр
		p("avatar-04", face{pale, hx("#e8e0d8"), hx("#6a4a8a"), hx("#e8b8c8"), "scarf", false, true, true, 1}), // Галина Сергеевна
		p("avatar-05", face{tan, dark, hx("#5a6a3a"), hx("#c8a878"), "short", true, false, true, 0.2}),         // Андрей
		p("avatar-06", face{light, blond, hx("#e8a030"), hx("#9ad08a"), "long", false, false, true, 0.1}),      // Ольга
		p("avatar-07", face{tan, grey, hx("#4a4a50"), hx("#b8c8a0"), "cap", true, false, false, 1}),            // Михалыч
		o("avatar-08", avatarDaisy), // Ирина
		p("avatar-09", face{light, brown, hx("#2a5a8a"), hx("#7ab0d8"), "panama", false, true, true, 0.45}),       // Сергей
		p("avatar-10", face{pale, red, hx("#3a8a6a"), hx("#f0d8a0"), "curly", false, false, true, 0.8}),           // Тамара
		p("avatar-11", face{light, dark, hx("#e8e8e8"), hx("#d8c0f0"), "short", false, false, true, 0}),           // Денис
		p("avatar-12", face{light, hx("#8a5a3a"), hx("#d85a30"), hx("#c8e0a8"), "scarf", false, true, true, 0.7}), // Людмила
		o("avatar-13", avatarHoney), // Алексей
		p("avatar-14", face{pale, hx("#4a2a1a"), hx("#6ab0c8"), hx("#f4c8a8"), "long", false, true, true, 0}), // Катя
		p("avatar-15", face{tan, grey, hx("#2a3a5a"), hx("#a8b8c8"), "short", false, true, false, 0.9}),       // Виктор Палыч
	}
}

func portrait(z *Z, f face) {
	c := 256.0
	z.bokeh([]RGB{f.bg, f.bg.Mul(0.8), f.bg.Mix(RGB{1, 1, 1}, 0.3)}, RGB{1, 1, 1}, 14, 0.05)
	hy := 225.0
	// Длинные волосы и узел платка — за головой.
	switch f.hairStyle {
	case "long":
		z.fill(Box{c, hy + 70, 250, 300, 0, 100}, Shade(c, hy, 150, 200, 0, f.hair, 0.15))
	case "scarf":
		z.fill(Ellipse{c, hy - 5, 140, 150, 0}, Shade(c, hy, 150, 160, 0, f.shirt.Mix(RGB{1, 1, 1}, 0.25), 0.1))
	case "curly":
		for k := 0; k < 26; k++ {
			a := float64(k) / 26 * 2 * math.Pi
			z.sphere(c+math.Cos(a)*110, hy-20+math.Sin(a)*115, 42, 42, 0, z.jit(f.hair, 0.08), 0.2)
		}
	}
	// Плечи и шея.
	z.fill(Ellipse{c, 560, 230, 170, 0}, Shade(c, 520, 250, 200, 0, f.shirt, 0.1))
	z.fill(NewPoly([][2]float64{{c - 45, 395}, {c + 45, 395}, {c, 450}}), Solid(f.skin.Mul(0.85)))
	z.fill(Box{c, 360, 80, 90, 0, 20}, Solid(f.skin.Mul(0.82)))
	// Уши и голова.
	for _, s := range []float64{-1, 1} {
		z.sphere(c+s*96, hy+10, 20, 32, 0, f.skin.Mul(0.95), 0.05)
	}
	z.fill(Ellipse{c, hy, 98, 122, 0}, soft(Shade(c-10, hy-10, 120, 140, 0, f.skin, 0.12), f.skin))
	if f.beard {
		beard := f.hair
		if f.age > 0.5 {
			beard = hx("#d8d4cc")
		}
		z.fill(And{Ellipse{c, hy + 40, 100, 108, 0}, Box{c, hy + 120, 240, 190, 0, 0}}, Shade(c, hy+60, 110, 110, 0, beard, 0.05))
		z.fill(Ellipse{c, hy + 72, 40, 22, 0}, Solid(f.skin.Mul(0.9)))
	}
	// Глаза, брови, нос, рот.
	for _, s := range []float64{-1, 1} {
		ex := c + s*38
		z.solid(Ellipse{ex, hy - 5, 17, 11, 0}, hx("#fbfaf6"))
		z.solid(Circle{ex + 2, hy - 4, 9}, z.pick(hx("#5a7a9a"), hx("#6a4a2a"), hx("#4a6a4a")))
		z.solid(Circle{ex + 2, hy - 4, 4.5}, hx("#141010"))
		z.solid(Circle{ex + 5, hy - 7, 2.5}, hx("#ffffff"))
		z.fillA(NewLine(3, [2]float64{ex - 17, hy - 8}, [2]float64{ex, hy - 13}, [2]float64{ex + 17, hy - 8}), Solid(f.skin.Mul(0.6)), 0.8, 1.5)
		bc := f.hair.Mul(0.8)
		if f.hairStyle == "bald" || f.hairStyle == "cap" {
			bc = hx("#8a8078")
		}
		z.fill(NewLine(7, [2]float64{ex - 20, hy - 30 + s*0}, [2]float64{ex, hy - 36}, [2]float64{ex + 20, hy - 31}), Solid(bc))
		z.fillA(Circle{ex + s*8, hy + 38, 20}, Solid(hx("#e87070")), 0.15, 14)
		if f.age > 0.6 {
			for k := 0; k < 3; k++ {
				z.fillA(NewLine(1.5, [2]float64{ex + s*22, hy - 6 + float64(k)*6}, [2]float64{ex + s*32, hy - 10 + float64(k)*8}), Solid(f.skin.Mul(0.7)), 0.6, 1)
			}
		}
	}
	z.fillA(NewPoly([][2]float64{{c - 3, hy}, {c - 16, hy + 42}, {c + 10, hy + 46}}), Solid(f.skin.Mul(0.75)), 0.6, 4)
	bend := 12.0
	if !f.smile {
		bend = 2
	}
	mouth := [][2]float64{}
	for t := -1.0; t <= 1.001; t += 0.1 {
		mouth = append(mouth, [2]float64{c + t*28, hy + 70 + bend*(1-t*t)})
	}
	if f.beard {
		z.fill(NewLine(9, [2]float64{c - 42, hy + 64}, [2]float64{c, hy + 56}, [2]float64{c + 42, hy + 64}), Solid(hx("#d8d4cc").Mix(f.hair, 1-f.age)))
	}
	z.fill(NewLine(6, mouth...), Solid(hx("#a84a48")))
	if f.glasses {
		for _, s := range []float64{-1, 1} {
			z.fill(Ring{Box{c + s*38, hy - 4, 58, 42, 0, 14}, 5}, Solid(hx("#2a2a2e")))
		}
		z.fill(NewLine(5, [2]float64{c - 9, hy - 8}, [2]float64{c + 9, hy - 8}), Solid(hx("#2a2a2e")))
	}
	// Волосы — шапка над лицом с округлой линией лба, брови остаются видны.
	top := And{Cut{Ellipse{c, hy - 20, 108, 125, 0}, Ellipse{c, hy + 30, 94, 112, 0}}, Box{c, hy - 60, 300, 160, 0, 0}}
	crown := And{Ellipse{c, hy - 20, 108, 125, 0}, Box{c, hy - 110, 300, 170, 0, 0}}
	switch f.hairStyle {
	case "short", "long", "bun", "curly":
		z.fill(top, Shade(c, hy-60, 120, 110, 0, f.hair, 0.2))
		if f.hairStyle == "bun" {
			z.sphere(c, hy-140, 48, 42, 0, f.hair, 0.2)
		}
		if f.hairStyle == "long" {
			z.fill(Leaf(c-100, hy-60, 150, 90, 0.35), Shade(c, hy-60, 120, 110, 0, f.hair, 0.2))
		}
	case "bald":
		for _, s := range []float64{-1, 1} {
			z.fill(Ellipse{c + s*92, hy - 25, 20, 45, s * 0.2}, Shade(c, hy, 120, 120, 0, f.hair, 0.1))
		}
	case "scarf":
		z.fill(And{Ellipse{c, hy - 15, 120, 140, 0}, Box{c, hy - 110, 300, 150, 0, 0}}, Shade(c, hy-60, 130, 120, 0, f.shirt.Mix(RGB{1, 1, 1}, 0.25), 0.1))
		for k := 0; k < 12; k++ {
			z.solid(Circle{c + z.rf(-90, 90), hy - 50 - z.rf(0, 70), 7}, hx("#fff4e0"))
		}
		z.fill(Box{c, hy - 42, 212, 22, 0, 10}, Solid(f.hair))
	case "cap":
		z.fill(crown, Shade(c, hy-60, 120, 110, 0, hx("#3a4a5a"), 0.1))
		z.fill(Ellipse{c + 20, hy - 42, 120, 22, 0.05}, Solid(hx("#2a3440")))
	case "panama":
		z.fill(Ellipse{c, hy - 50, 175, 38, 0}, Solid(hx("#e8e0c8").Mul(0.9)))
		z.fill(crown, Shade(c, hy-80, 120, 110, 0, hx("#f0e8d0"), 0.1))
		z.solid(Box{c, hy - 62, 214, 14, 0, 4}, hx("#6a8a4a"))
	}
	z.finish(0.2, 0.25)
}

func avatarSunset(z *Z) {
	lake(z, 0)
}

func avatarDaisy(z *Z) {
	z.bokeh(leafG, RGB{1, 1, 1}, 20, 0.05)
	z.fill(Capsule{300, 520, 250, 260, 8, 6}, Solid(hx("#4a7a2a")))
	z.flower(250, 240, 200, 22, hx("#fbfbf6"), hx("#f0c020"), 0.16)
	z.finish(0.2, 0.3)
}

func avatarHoney(z *Z) {
	z.c.Each(func(x, y float64, _ RGB) RGB { return hx("#a86a10") })
	r := 58.0
	for row := -1; row < 7; row++ {
		for col := -1; col < 6; col++ {
			x := float64(col)*r*1.75 + float64(row%2)*r*0.87
			y := float64(row) * r * 1.5
			pts := [][2]float64{}
			for k := 0; k < 6; k++ {
				a := math.Pi/6 + float64(k)*math.Pi/3
				pts = append(pts, [2]float64{x + math.Cos(a)*r*0.92, y + math.Sin(a)*r*0.92})
			}
			c := z.jit(hx("#f0a820"), 0.12)
			z.fill(NewPoly(pts), Shade(x, y, r, r, 0, c, 0.6))
		}
	}
	bx, by := 290.0, 270.0
	for _, s := range []float64{-1, 1} {
		z.fillA(Ellipse{bx + s*30, by - 50, 45, 26, s * 0.5}, Solid(RGB{1, 1, 1}), 0.7, 3)
	}
	z.fill(Ellipse{bx, by, 70, 45, 0.2}, func(x, y float64) RGB {
		if int((x-bx)/22+10)%2 == 0 {
			return hx("#1a140c")
		}
		return hx("#f0c020")
	})
	z.sphere(bx-75, by-15, 30, 28, 0, hx("#1a140c"), 0.5)
	z.finish(0.2, 0.3)
}

// soft — объём лица мягче, чем у яблока: тени не уходят в черноту.
func soft(p Paint, base RGB) Paint {
	return func(x, y float64) RGB { return base.Mix(p(x, y), 0.45) }
}
