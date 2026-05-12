package cmd

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"sync"

	"github.com/michaelquigley/dfw"
)

const (
	appID    = "com.quigley.dfw.example.watch"
	appTitle = "dfw Example Watch"
)

var (
	iconOnce sync.Once
	iconData []byte
	iconErr  error
)

func watchPathArg(args []string) (string, error) {
	if len(args) > 0 {
		return filepath.Abs(args[0])
	}
	return os.Getwd()
}

func applyDevTools(enabled bool) error {
	if !enabled {
		return nil
	}
	return os.Setenv("DFW_DEVTOOLS", "1")
}

func appIconPNG() ([]byte, error) {
	iconOnce.Do(func() {
		iconData, iconErr = generateAppIconPNG(32)
	})
	if iconErr != nil {
		return nil, iconErr
	}
	if len(iconData) == 0 {
		return nil, errors.New("empty generated icon")
	}
	return append([]byte(nil), iconData...), nil
}

func generateAppIconPNG(size int) ([]byte, error) {
	if size <= 0 {
		return nil, errors.New("icon size must be positive")
	}

	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	scaledRect := func(x0, y0, x1, y1 int) image.Rectangle {
		return image.Rect(x0*size/32, y0*size/32, x1*size/32, y1*size/32)
	}

	draw.Draw(img, img.Bounds(), &image.Uniform{C: color.NRGBA{R: 22, G: 28, B: 36, A: 255}}, image.Point{}, draw.Src)
	draw.Draw(img, scaledRect(6, 7, 26, 13), &image.Uniform{C: color.NRGBA{R: 74, G: 199, B: 185, A: 255}}, image.Point{}, draw.Src)
	draw.Draw(img, scaledRect(6, 16, 14, 25), &image.Uniform{C: color.NRGBA{R: 245, G: 176, B: 65, A: 255}}, image.Point{}, draw.Src)
	draw.Draw(img, scaledRect(17, 16, 26, 25), &image.Uniform{C: color.NRGBA{R: 235, G: 94, B: 94, A: 255}}, image.Point{}, draw.Src)

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func windowApp() (dfw.WindowApp, error) {
	icon, err := appIconPNG()
	if err != nil {
		return dfw.WindowApp{}, err
	}
	return dfw.WindowApp{
		AppID:       appID,
		Title:       appTitle,
		InitialSize: image.Pt(1100, 760),
		IconPNG:     icon,
	}, nil
}
