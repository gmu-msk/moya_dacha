package media

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	_ "image/png" // чтобы image.Decode понимал PNG
	"math"
	"net/http"

	"golang.org/x/image/draw"
)

// ErrNotAnImage — прислали не картинку или не тот формат.
var ErrNotAnImage = errors.New("файл не является картинкой JPEG или PNG")

// jpegQuality — с каким качеством пересохраняются картинки. 85 — та
// точка, после которой разница видна только измерением, а вес растёт.
const jpegQuality = 85

// Image — картинка, приведённая к виду, в котором сервис её хранит.
type Image struct {
	Content []byte
	Width   int
	Height  int
}

// Normalize приводит загруженную картинку к виду, в котором она хранится:
// не больше maxSide по большей стороне, всегда JPEG.
//
// Картинка не просто проверяется, а пересобирается: из файла уходит всё,
// что картинкой не является, — EXIF с координатами дачи в том числе.
// Поэтому поворот из EXIF применяется здесь, до того как сам EXIF
// будет отброшен: иначе снятое вертикально легло бы набок
// (specs/003-posts.md, требование 7).
func Normalize(raw []byte, maxSide int) (Image, error) {
	switch http.DetectContentType(raw) {
	case "image/jpeg", "image/png":
	default:
		return Image{}, ErrNotAnImage
	}

	src, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return Image{}, ErrNotAnImage
	}
	src = applyOrientation(src, readOrientation(raw))

	bounds := src.Bounds()
	width, height := fit(bounds.Dx(), bounds.Dy(), maxSide)

	// Рисуем на белом: у PNG бывает прозрачность, а в JPEG её нет —
	// без белой подложки прозрачные места стали бы чёрными.
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(dst, dst.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, bounds, draw.Over, nil)

	var out bytes.Buffer
	if err := jpeg.Encode(&out, dst, &jpeg.Options{Quality: jpegQuality}); err != nil {
		return Image{}, err
	}
	return Image{Content: out.Bytes(), Width: width, Height: height}, nil
}

// fit вписывает размеры в квадрат со стороной max, сохраняя пропорции.
// Картинку меньше квадрата не увеличивает.
func fit(width, height, max int) (int, int) {
	if width <= max && height <= max {
		return width, height
	}

	scale := math.Min(float64(max)/float64(width), float64(max)/float64(height))
	return atLeastOne(int(math.Round(float64(width) * scale))),
		atLeastOne(int(math.Round(float64(height) * scale)))
}

func atLeastOne(size int) int {
	if size < 1 {
		return 1
	}
	return size
}
