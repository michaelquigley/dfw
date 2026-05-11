//go:build !linux && !windows

package dfw

import "unsafe"

func applyWindowIcon(_ unsafe.Pointer, iconPNG []byte) error {
	_, err := decodeIconPNG(iconPNG)
	return err
}
