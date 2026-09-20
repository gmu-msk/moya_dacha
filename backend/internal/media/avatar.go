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

const (
	// AvatarMaxBytes — сколько может весить загружаемая картинка.
	AvatarMaxBytes = 5 << 20
	// AvatarMaxSide — до какого размера уменьшается аватар.
	AvatarMaxSide = 512
	// AvatarExt — расширение, с которым аватар сохраняется.
	AvatarExt = ".jpg"

	avatarQuality = 85
)

// NormalizeAvatar приводит загруженную картинку к виду, в котором аватар
// хранится: не больше AvatarMaxSide по стороне, всегда JPEG.
//
// Картинка не просто проверяется, а пересобирается: из файла уходит всё,
// что картинкой не является, — EXIF с координатами дачи в том числе
// (specs/002-profile.md, требование 7).
func NormalizeAvatar(raw []byte) ([]byte, error) {
	switch http.DetectContentType(raw) {
	case "image/jpeg", "image/png":
	default:
		return nil, ErrNotAnImage
	}

	src, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, ErrNotAnImage
	}

	bounds := src.Bounds()
	width, height := fit(bounds.Dx(), bounds.Dy(), AvatarMaxSide)

	// Рисуем на белом: у PNG бывает прозрачность, а в JPEG её нет —
	// без белой подложки прозрачные места стали бы чёрными.
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(dst, dst.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, bounds, draw.Over, nil)

	var out bytes.Buffer
	if err := jpeg.Encode(&out, dst, &jpeg.Options{Quality: avatarQuality}); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
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
