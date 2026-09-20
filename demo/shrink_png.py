#!/usr/bin/env python3
"""Уменьшить PNG прореживанием точек.

Нужно только для того, чтобы снимок экрана влезал в лог прогона строкой
base64: полноразмерный снимок в логе не помещается. Пользуется одной
стандартной библиотекой — на раннере может не оказаться ни ImageMagick,
ни ffmpeg.

    shrink_png.py вход.png выход.png [ширина]
"""

import struct
import sys
import zlib


def read_png(path):
	data = open(path, "rb").read()
	pos, idat, header = 8, b"", None
	while pos < len(data):
		length, kind = struct.unpack(">I4s", data[pos : pos + 8])
		body = data[pos + 8 : pos + 8 + length]
		if kind == b"IHDR":
			header = struct.unpack(">IIBB", body[:10])
		elif kind == b"IDAT":
			idat += body
		pos += length + 12

	width, height, depth, color = header
	if depth != 8 or color not in (2, 6):
		raise SystemExit(f"{path}: поддерживаются только 8-битные RGB и RGBA")
	channels = 3 if color == 2 else 4
	return width, height, channels, zlib.decompress(idat)


def undo_filters(raw, width, height, channels):
	stride = width * channels
	rows, prev, off = [], bytearray(stride), 0
	for _ in range(height):
		filt = raw[off]
		line = bytearray(raw[off + 1 : off + 1 + stride])
		off += stride + 1
		for i in range(stride):
			a = line[i - channels] if i >= channels else 0
			b = prev[i]
			c = prev[i - channels] if i >= channels else 0
			if filt == 1:
				line[i] = (line[i] + a) & 0xFF
			elif filt == 2:
				line[i] = (line[i] + b) & 0xFF
			elif filt == 3:
				line[i] = (line[i] + (a + b) // 2) & 0xFF
			elif filt == 4:
				p = a + b - c
				pa, pb, pc = abs(p - a), abs(p - b), abs(p - c)
				pr = a if (pa <= pb and pa <= pc) else (b if pb <= pc else c)
				line[i] = (line[i] + pr) & 0xFF
		rows.append(line)
		prev = line
	return rows


def chunk(kind, payload):
	crc = zlib.crc32(kind + payload) & 0xFFFFFFFF
	return struct.pack(">I", len(payload)) + kind + payload + struct.pack(">I", crc)


def main():
	src, dst = sys.argv[1], sys.argv[2]
	target = int(sys.argv[3]) if len(sys.argv) > 3 else 300

	width, height, channels, raw = read_png(src)
	rows = undo_filters(raw, width, height, channels)

	step = max(1, round(width / target))
	xs = range(0, width, step)
	ys = range(0, height, step)

	body = bytearray()
	for y in ys:
		body.append(0)
		row = rows[y]
		for x in xs:
			body += row[x * channels : x * channels + 3]

	png = (
		b"\x89PNG\r\n\x1a\n"
		+ chunk(b"IHDR", struct.pack(">IIBBBBB", len(xs), len(ys), 8, 2, 0, 0, 0))
		+ chunk(b"IDAT", zlib.compress(bytes(body), 9))
		+ chunk(b"IEND", b"")
	)
	open(dst, "wb").write(png)


if __name__ == "__main__":
	main()
