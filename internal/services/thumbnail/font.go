package thumbnail

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

const (
	fontThumbWidth  = 512
	fontThumbHeight = 256
	fontSampleSize  = 36
	fontDPI         = 72
	fontSampleText  = "The quick brown fox\njumps over the lazy dog"
)

// renderFont loads a TTF/OTF font and renders a sample string. The output
// is a PNG so antialiasing and stem clarity survive without quality loss.
// We don't try to be clever about font metrics — pick a baseline that puts
// the text near the visual centre and let the renderer crop naturally.
func (g *Generator) renderFont(ctx context.Context, in Input) (Result, error) {
	data, err := io.ReadAll(in.Reader)
	if err != nil {
		return Result{}, fmt.Errorf("font read: %w", err)
	}
	face, err := loadFontFace(data, fontSampleSize)
	if err != nil {
		return Result{}, fmt.Errorf("font load: %w", err)
	}
	defer face.Close()

	img := image.NewRGBA(image.Rect(0, 0, fontThumbWidth, fontThumbHeight))
	bg := color.RGBA{R: 0x16, G: 0x16, B: 0x18, A: 0xff}
	for y := 0; y < fontThumbHeight; y++ {
		for x := 0; x < fontThumbWidth; x++ {
			img.SetRGBA(x, y, bg)
		}
	}

	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(color.RGBA{R: 0xee, G: 0xee, B: 0xee, A: 0xff}),
		Face: face,
	}

	lineHeight := face.Metrics().Height.Round() + 4
	startY := fontThumbHeight/2 - lineHeight/2 + face.Metrics().Ascent.Round()/2
	y := startY
	for _, line := range splitLines(fontSampleText) {
		w := d.MeasureString(line).Round()
		x := (fontThumbWidth - w) / 2
		if x < 16 {
			x = 16
		}
		d.Dot = fixed.P(x, y)
		d.DrawString(line)
		y += lineHeight
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return Result{}, fmt.Errorf("font encode: %w", err)
	}
	key, err := g.putThumbBytes(ctx, buf.Bytes())
	if err != nil {
		return Result{}, err
	}
	return Result{ObjectKey: key, ContentType: "image/png", Category: "font"}, nil
}

// loadFontFace parses the supplied bytes as TTF/OTF and returns a face at
// the requested point size. TTC (collections) aren't supported in v1.
func loadFontFace(data []byte, size float64) (font.Face, error) {
	parsed, err := opentype.Parse(data)
	if err != nil {
		return nil, err
	}
	return opentype.NewFace(parsed, &opentype.FaceOptions{
		Size:    size,
		DPI:     fontDPI,
		Hinting: font.HintingFull,
	})
}

func splitLines(s string) []string {
	out := []string{}
	last := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[last:i])
			last = i + 1
		}
	}
	out = append(out, s[last:])
	return out
}
