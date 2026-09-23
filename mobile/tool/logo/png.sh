#!/usr/bin/env bash
# Печатает PNG-иконки приложения из ikonka.svg во все плотности Android.
#
# Нужны только для Android до версии 8: начиная с 8 лаунчер собирает
# иконку из векторных слоёв res/drawable/ic_launcher_*.xml. PNG
# коммитятся; запускать скрипт нужно, только если поменялся ikonka.svg.
#
# Рисует rsvg-convert (macOS: brew install librsvg), а если его нет —
# Chromium без окна (путь к нему в переменной CHROME) и Pillow для python3.
set -euo pipefail

here=$(cd "$(dirname "$0")" && pwd)
res="$here/../../android/app/src/main/res"
svg="$here/ikonka.svg"

sizes="mdpi:48 hdpi:72 xhdpi:96 xxhdpi:144 xxxhdpi:192"

if command -v rsvg-convert >/dev/null; then
  for pair in $sizes; do
    rsvg-convert -w "${pair##*:}" -h "${pair##*:}" \
      -o "$res/mipmap-${pair%%:*}/ic_launcher.png" "$svg"
  done
elif [ -n "${CHROME:-}" ]; then
  # Chromium снимает окно чуть ниже заданного, поэтому картинка рисуется
  # крупно в большом окне, а потом вырезается и уменьшается (Pillow).
  tmp=$(mktemp -d)
  printf '<html><body style="margin:0"><img src="file://%s" style="width:512px;height:512px;display:block"></body></html>' \
    "$svg" > "$tmp/page.html"
  "$CHROME" --headless --no-sandbox --disable-gpu --allow-file-access-from-files \
    --default-background-color=00000000 --hide-scrollbars \
    --window-size=900,900 --screenshot="$tmp/big.png" "file://$tmp/page.html" 2>/dev/null
  python3 - "$tmp/big.png" "$res" $sizes <<'PY'
import sys
from PIL import Image
big = Image.open(sys.argv[1]).crop((0, 0, 512, 512))
for pair in sys.argv[3:]:
    density, size = pair.split(':')
    big.resize((int(size), int(size)), Image.LANCZOS).save(
        f'{sys.argv[2]}/mipmap-{density}/ic_launcher.png', optimize=True)
PY
  rm -rf "$tmp"
else
  echo "нужен rsvg-convert или переменная CHROME с путём к Chromium" >&2
  exit 1
fi
ls -l "$res"/mipmap-*/ic_launcher.png
