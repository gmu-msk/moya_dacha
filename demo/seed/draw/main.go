// Рисует фотографии и аватары для демо-стенда (demo/stories/000-dacha).
//
// Настоящим снимкам на стенде не место (demo/seed/README.md), а одинаковые
// заглушки не показывают, как выглядит лента. Поэтому картинки рисуются
// кодом: у каждой своя сцена и свой разброс, результат воспроизводим —
// тот же запуск даёт те же файлы.
//
//	cd demo/seed/draw && go run .                 всё заново
//	cd demo/seed/draw && go run . tomatoes-1 cat-1  только эти
//	OUT=/tmp/x SHEET=/tmp/x.jpg go run .          в другую папку и с листом миниатюр
//
// Картинки кладутся в demo/seed/media и коммитятся: стенд поднимается
// без Go, а demo/seed.sh только копирует готовые файлы.
package main

import (
	"fmt"
	"hash/fnv"
	"image"
	"image/jpeg"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
)

type Pic struct {
	Name string
	W, H int
	Draw func(z *Z)
}

// Размеры как у настоящих: фото постов — 1600 по большей стороне,
// аватары — 512×512 (ADR-0011).
func photo(name string, w, h int, f func(z *Z, v int), v int) Pic {
	return Pic{name, w, h, func(z *Z) { f(z, v) }}
}

func main() {
	out := filepath.Join("..", "media")
	if d := os.Getenv("OUT"); d != "" {
		out = d
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	pics := append(photos(), avatars()...)
	want := map[string]bool{}
	for _, a := range os.Args[1:] {
		want[a] = true
	}

	var wg sync.WaitGroup
	jobs := make(chan Pic)
	for i := 0; i < runtime.NumCPU(); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for p := range jobs {
				if err := render(p, out); err != nil {
					fmt.Fprintln(os.Stderr, p.Name, err)
					os.Exit(1)
				}
			}
		}()
	}
	names := []string{}
	for _, p := range pics {
		if len(want) > 0 && !want[p.Name] {
			continue
		}
		names = append(names, p.Name)
		jobs <- p
	}
	close(jobs)
	wg.Wait()
	sort.Strings(names)
	for _, n := range names {
		fmt.Println(n)
	}
	if sh := os.Getenv("SHEET"); sh != "" {
		if err := contactSheet(out, names, sh); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}

func render(p Pic, dir string) error {
	h := fnv.New32a()
	h.Write([]byte(p.Name))
	seed := int(h.Sum32())
	c := NewCanvas(p.W, p.H)
	z := &Z{c: c, r: rand.New(rand.NewSource(int64(seed))), W: float64(p.W), H: float64(p.H),
		S: math.Min(float64(p.W), float64(p.H)), seed: seed % 100000}
	p.Draw(z)

	img := image.NewRGBA(image.Rect(0, 0, p.W, p.H))
	for i, q := range c.P {
		img.Pix[i*4+0] = u8(q.R)
		img.Pix[i*4+1] = u8(q.G)
		img.Pix[i*4+2] = u8(q.B)
		img.Pix[i*4+3] = 255
	}
	f, err := os.Create(filepath.Join(dir, p.Name+".jpg"))
	if err != nil {
		return err
	}
	defer f.Close()
	return jpeg.Encode(f, img, &jpeg.Options{Quality: 65})
}

func u8(v float64) uint8 { return uint8(clamp(v, 0, 1)*255 + 0.5) }
