package core

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/png"
)

// DecodeIconPNG decodes PNG icon bytes into an NRGBA image. It returns a nil
// image (and nil error) when iconPNG is empty.
func DecodeIconPNG(iconPNG []byte) (*image.NRGBA, error) {
	if len(iconPNG) == 0 {
		return nil, nil
	}

	src, err := png.Decode(bytes.NewReader(iconPNG))
	if err != nil {
		return nil, fmt.Errorf("dfw: decode icon png: %w", err)
	}

	bounds := src.Bounds()
	if bounds.Empty() {
		return nil, errors.New("dfw: icon png has empty bounds")
	}

	dst := image.NewNRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	draw.Draw(dst, dst.Bounds(), src, bounds.Min, draw.Src)
	return dst, nil
}
