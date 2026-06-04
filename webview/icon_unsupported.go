//go:build !linux && !windows

package webview

import (
	"unsafe"

	"github.com/michaelquigley/dfw/internal/core"
)

func applyWindowIcon(_ unsafe.Pointer, iconPNG []byte) error {
	_, err := core.DecodeIconPNG(iconPNG)
	return err
}
