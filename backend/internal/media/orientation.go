package media

import (
	"encoding/binary"
	"image"
)

// Телефон почти никогда не поворачивает снятый кадр: он пишет в EXIF, как
// его держали, и ждёт, что картинку развернут при показе. Мы EXIF
// выбрасываем вместе с остальными метаданными, поэтому поворот нужно
// применить к пикселям, иначе вертикальное фото ляжет набок
// (specs/003-posts.md, требование 6).
//
// Готовой библиотеки для этого в зависимостях нет, а нужен ровно один тег
// из заголовка — читаем его сами.

// Значения тега Orientation из EXIF.
const (
	orientationNormal        = 1
	orientationFlipH         = 2
	orientationRotate180     = 3
	orientationFlipV         = 4
	orientationTransverseCW  = 5 // зеркально и на 90° по часовой
	orientationRotate90CW    = 6
	orientationTransverseCCW = 7 // зеркально и на 90° против часовой
	orientationRotate90CCW   = 8
)

// readOrientation достаёт из JPEG тег Orientation. Если его нет или файл
// не JPEG, возвращает orientationNormal: разворачивать нечего.
func readOrientation(raw []byte) int {
	exif, ok := exifSegment(raw)
	if !ok {
		return orientationNormal
	}
	return orientationTag(exif)
}

// exifSegment ищет в JPEG сегмент APP1 с заголовком "Exif\0\0" и
// возвращает то, что идёт после него, — блок TIFF.
func exifSegment(raw []byte) ([]byte, bool) {
	if len(raw) < 4 || raw[0] != 0xFF || raw[1] != 0xD8 {
		return nil, false
	}

	for offset := 2; offset+4 <= len(raw); {
		if raw[offset] != 0xFF {
			return nil, false
		}
		marker := raw[offset+1]
		// Начало картинки — дальше метаданных не будет.
		if marker == 0xDA || marker == 0xD9 {
			return nil, false
		}

		size := int(binary.BigEndian.Uint16(raw[offset+2:]))
		if size < 2 || offset+2+size > len(raw) {
			return nil, false
		}
		body := raw[offset+4 : offset+2+size]

		const header = "Exif\x00\x00"
		if marker == 0xE1 && len(body) > len(header) && string(body[:len(header)]) == header {
			return body[len(header):], true
		}
		offset += 2 + size
	}
	return nil, false
}

// orientationTag читает тег 0x0112 из нулевого IFD блока TIFF.
func orientationTag(tiff []byte) int {
	if len(tiff) < 8 {
		return orientationNormal
	}

	var order binary.ByteOrder
	switch string(tiff[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return orientationNormal
	}

	ifd := int(order.Uint32(tiff[4:8]))
	if ifd+2 > len(tiff) {
		return orientationNormal
	}

	count := int(order.Uint16(tiff[ifd:]))
	for i := range count {
		// Запись IFD — двенадцать байт: тег, тип, число значений, значение.
		entry := ifd + 2 + i*12
		if entry+12 > len(tiff) {
			return orientationNormal
		}
		if order.Uint16(tiff[entry:]) != 0x0112 {
			continue
		}

		value := int(order.Uint16(tiff[entry+8:]))
		if value < orientationNormal || value > orientationRotate90CCW {
			return orientationNormal
		}
		return value
	}
	return orientationNormal
}

// applyOrientation разворачивает картинку так, как её задумал снимавший.
func applyOrientation(src image.Image, orientation int) image.Image {
	if orientation == orientationNormal {
		return src
	}

	bounds := src.Bounds()
	width, height := bounds.Dx(), bounds.Dy()

	// У четырёх из восьми вариантов стороны меняются местами.
	turned := orientation >= orientationTransverseCW
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	if turned {
		dst = image.NewRGBA(image.Rect(0, 0, height, width))
	}

	for y := range height {
		for x := range width {
			at := src.At(bounds.Min.X+x, bounds.Min.Y+y)
			switch orientation {
			case orientationFlipH:
				dst.Set(width-1-x, y, at)
			case orientationRotate180:
				dst.Set(width-1-x, height-1-y, at)
			case orientationFlipV:
				dst.Set(x, height-1-y, at)
			case orientationTransverseCW:
				dst.Set(y, x, at)
			case orientationRotate90CW:
				dst.Set(height-1-y, x, at)
			case orientationTransverseCCW:
				dst.Set(height-1-y, width-1-x, at)
			case orientationRotate90CCW:
				dst.Set(y, width-1-x, at)
			default:
				dst.Set(x, y, at)
			}
		}
	}

	return dst
}
