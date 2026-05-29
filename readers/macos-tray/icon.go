package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strconv"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// colorErrorRed is system red used for error states.
var colorErrorRed = color.RGBA{R: 0xDC, G: 0x32, B: 0x32, A: 0xFF}

// colorClaudeOrange is the default healthy-state color.
var colorClaudeOrange = color.RGBA{R: 0xD9, G: 0x77, B: 0x57, A: 0xFF}

// parseHexColor parses a "#RRGGBB" hex string into a color.RGBA.
// Returns Claude orange if parsing fails.
func parseHexColor(hex string) color.RGBA {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		return colorClaudeOrange
	}
	r, err1 := strconv.ParseUint(hex[0:2], 16, 8)
	g, err2 := strconv.ParseUint(hex[2:4], 16, 8)
	b, err3 := strconv.ParseUint(hex[4:6], 16, 8)
	if err1 != nil || err2 != nil || err3 != nil {
		return colorClaudeOrange
	}
	return color.RGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: 0xFF}
}

// renderTextIcon renders text as a colored PNG suitable for systray.SetIcon.
// Height is 44px (@2x for 22pt menu bar). Text is rendered in the given color
// on a transparent background.
func renderTextIcon(text string, clr color.RGBA) ([]byte, error) {
	const height = 44
	const fontSize = 28.0 // points at @2x
	const dpi = 144.0     // @2x Retina

	// Load font face
	f, err := opentype.Parse(gomono.TTF)
	if err != nil {
		return nil, err
	}
	face, err := opentype.NewFace(f, &opentype.FaceOptions{
		Size:    fontSize,
		DPI:     dpi,
		Hinting: font.HintingFull,
	})
	if err != nil {
		return nil, err
	}
	defer face.Close()

	// Measure text width
	advance := font.MeasureString(face, text)
	width := advance.Ceil() + 4 // small padding

	// Create RGBA image
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	// Background is transparent (zero value)

	// Draw text
	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(clr),
		Face: face,
		Dot:  fixed.Point26_6{X: fixed.I(2), Y: fixed.I(height - 10)},
	}
	d.DrawString(text)

	// Encode as PNG
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
