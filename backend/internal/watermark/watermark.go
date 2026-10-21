package watermark

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/draw"
	_ "image/gif" // 注册 GIF 解码
	"image/jpeg"
	_ "image/png" // 注册 PNG 解码
	"io"
	"log/slog"
	"math"
	"os"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// Burn 在图片上动态烧录平铺倾斜文字水印。字体缺失时降级：返回原始 JPEG 编码并记录告警。
func Burn(ctx context.Context, r io.Reader, text, fontPath string) ([]byte, error) {
	src, _, err := image.Decode(r)
	if err != nil {
		return nil, err
	}
	rgba := toRGBA(src)

	face, err := loadFace(fontPath, 26)
	if err != nil {
		slog.Default().Warn("水印字体缺失，降级为不烧录", "err", err.Error())
		var buf bytes.Buffer
		_ = jpeg.Encode(&buf, rgba, &jpeg.Options{Quality: 85})
		return buf.Bytes(), nil
	}

	tile := renderTextTile(face, text)
	rotated := rotate(tile, -28*math.Pi/180)
	out := tileOverlay(rgba, rotated, 220, 160)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, out, &jpeg.Options{Quality: 85}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func loadFace(path string, size float64) (font.Face, error) {
	if path == "" {
		return nil, errors.New("未配置字体路径")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	f, err := opentype.Parse(data)
	if err != nil {
		return nil, err
	}
	return opentype.NewFace(f, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
}

// renderTextTile 渲染带描边的半透明白字到透明临时图。
func renderTextTile(face font.Face, text string) *image.RGBA {
	d := &font.Drawer{Face: face}
	w := d.MeasureString(text).Round() + 16
	h := int(face.Metrics().Height.Round()) + 12
	tile := image.NewRGBA(image.Rect(0, 0, w, h))

	drawTile := func(col color.Color, offX, offY int) {
		d.Dst = tile
		d.Src = image.NewUniform(col)
		d.Dot = fixed.P(8+offX, 8+int(face.Metrics().Ascent.Round())+offY)
		d.DrawString(text)
	}
	// 阴影/描边
	drawTile(color.RGBA{0, 0, 0, 110}, 1, 1)
	drawTile(color.RGBA{0, 0, 0, 110}, -1, -1)
	drawTile(color.RGBA{255, 255, 255, 150}, 0, 0)
	return tile
}

// rotate 最近邻反向映射旋转，背景透明。
func rotate(src image.Image, angle float64) *image.RGBA {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	cos, sin := math.Cos(angle), math.Sin(angle)
	nw := int(math.Abs(float64(w)*cos)+math.Abs(float64(h)*sin)) + 1
	nh := int(math.Abs(float64(w)*sin)+math.Abs(float64(h)*cos)) + 1
	dst := image.NewRGBA(image.Rect(0, 0, nw, nh))

	cx, cy := float64(w)/2, float64(h)/2
	ncx, ncy := float64(nw)/2, float64(nh)/2
	for y := 0; y < nh; y++ {
		for x := 0; x < nw; x++ {
			sx := (float64(x)-ncx)*cos + (float64(y)-ncy)*sin + cx
			sy := -(float64(x)-ncx)*sin + (float64(y)-ncy)*cos + cy
			ix, iy := int(sx), int(sy)
			if ix >= 0 && ix < w && iy >= 0 && iy < h {
				dst.Set(x, y, src.At(ix, iy))
			}
		}
	}
	return dst
}

func tileOverlay(dst *image.RGBA, tile image.Image, stepX, stepY int) *image.RGBA {
	tb := tile.Bounds()
	W, H := dst.Bounds().Dx(), dst.Bounds().Dy()
	for y := -tb.Dy(); y < H; y += stepY {
		for x := -tb.Dx() / 2; x < W; x += stepX {
			draw.Draw(dst, image.Rect(x, y, x+tb.Dx(), y+tb.Dy()), tile, tb.Min, draw.Over)
		}
	}
	return dst
}

func toRGBA(img image.Image) *image.RGBA {
	if r, ok := img.(*image.RGBA); ok {
		return r
	}
	b := img.Bounds()
	dst := image.NewRGBA(b)
	draw.Draw(dst, b, img, b.Min, draw.Src)
	return dst
}
