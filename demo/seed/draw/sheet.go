package main

// Лист миниатюр для просмотра глазами: SHEET=путь.jpg go run .

import (
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"sort"
)

func contactSheet(dir string, names []string, path string) error {
	sort.Strings(names)
	const cols, tw, th = 6, 300, 300
	rows := (len(names) + cols - 1) / cols
	sheet := image.NewRGBA(image.Rect(0, 0, cols*tw, rows*th))
	for i, n := range names {
		f, err := os.Open(filepath.Join(dir, n+".jpg"))
		if err != nil {
			return err
		}
		img, err := jpeg.Decode(f)
		f.Close()
		if err != nil {
			return err
		}
		b := img.Bounds()
		ox, oy := (i%cols)*tw, (i/cols)*th
		// Вписать с сохранением пропорций.
		k := max(b.Dx()/(tw-4), b.Dy()/(th-4)) + 1
		for y := 0; y < b.Dy()/k; y++ {
			for x := 0; x < b.Dx()/k; x++ {
				sx := b.Min.X + x*k
				sy := b.Min.Y + y*k
				sheet.Set(ox+x, oy+y, img.At(sx, sy))
			}
		}
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return jpeg.Encode(f, sheet, &jpeg.Options{Quality: 85})
}
