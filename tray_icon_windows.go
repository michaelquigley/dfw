//go:build windows

package dfw

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

func trayIconBytes(iconPNG []byte) ([]byte, error) {
	icon, err := decodeIconPNG(iconPNG)
	if err != nil {
		return nil, fmt.Errorf("dfw: tray icon: %w", err)
	}
	if icon == nil {
		return nil, nil
	}

	var out bytes.Buffer
	write := func(value any) {
		_ = binary.Write(&out, binary.LittleEndian, value)
	}

	width := icon.Bounds().Dx()
	height := icon.Bounds().Dy()

	write(uint16(0)) // reserved
	write(uint16(1)) // image type: icon
	write(uint16(1)) // image count
	out.WriteByte(iconDimensionByte(width))
	out.WriteByte(iconDimensionByte(height))
	out.WriteByte(0) // color count
	out.WriteByte(0) // reserved
	write(uint16(1)) // color planes
	write(uint16(32))
	write(uint32(len(iconPNG)))
	write(uint32(22))
	out.Write(iconPNG)

	return out.Bytes(), nil
}

func iconDimensionByte(value int) byte {
	if value >= 256 {
		return 0
	}
	return byte(value)
}
