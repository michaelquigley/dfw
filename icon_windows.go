//go:build windows

package dfw

import (
	"fmt"
	"image"
	"syscall"
	"unsafe"
)

const (
	biBitFields          = 3
	dibRGBColors         = 0
	iconSmall            = 0
	iconBig              = 1
	lcsWindowsColorSpace = 0x57696e20
	smCXIcon             = 11
	smCYIcon             = 12
	smCXSmallIcon        = 49
	smCYSmallIcon        = 50
	wmSetIcon            = 0x0080
)

var (
	gdi32  = syscall.NewLazyDLL("gdi32.dll")
	user32 = syscall.NewLazyDLL("user32.dll")

	procCreateBitmap       = gdi32.NewProc("CreateBitmap")
	procCreateDIBSection   = gdi32.NewProc("CreateDIBSection")
	procDeleteObject       = gdi32.NewProc("DeleteObject")
	procCreateIconIndirect = user32.NewProc("CreateIconIndirect")
	procDestroyIcon        = user32.NewProc("DestroyIcon")
	procGetSystemMetrics   = user32.NewProc("GetSystemMetrics")
	procSendMessageW       = user32.NewProc("SendMessageW")
)

type bitmapV5Header struct {
	Size          uint32
	Width         int32
	Height        int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
	RedMask       uint32
	GreenMask     uint32
	BlueMask      uint32
	AlphaMask     uint32
	CSType        uint32
	Endpoints     [9]int32
	GammaRed      uint32
	GammaGreen    uint32
	GammaBlue     uint32
	Intent        uint32
	ProfileData   uint32
	ProfileSize   uint32
	Reserved      uint32
}

type iconInfo struct {
	Icon     int32
	XHotspot uint32
	YHotspot uint32
	Mask     uintptr
	Color    uintptr
}

func applyWindowIcon(window unsafe.Pointer, iconPNG []byte) error {
	if len(iconPNG) == 0 {
		return nil
	}

	img, err := decodeIconPNG(iconPNG)
	if err != nil {
		return err
	}

	big, err := makeIcon(img, systemMetric(smCXIcon), systemMetric(smCYIcon))
	if err != nil {
		return err
	}
	small, err := makeIcon(img, systemMetric(smCXSmallIcon), systemMetric(smCYSmallIcon))
	if err != nil {
		_, _, _ = procDestroyIcon.Call(big)
		return err
	}

	hwnd := uintptr(window)
	procSendMessageW.Call(hwnd, wmSetIcon, iconBig, big)
	procSendMessageW.Call(hwnd, wmSetIcon, iconSmall, small)
	return nil
}

func makeIcon(src *image.NRGBA, width int, height int) (uintptr, error) {
	if width <= 0 || height <= 0 {
		return 0, fmt.Errorf("dfw: invalid icon size %dx%d", width, height)
	}

	rgba := image.NewNRGBA(image.Rect(0, 0, width, height))
	drawNearest(rgba, src)

	header := bitmapV5Header{
		Size:        uint32(unsafe.Sizeof(bitmapV5Header{})),
		Width:       int32(width),
		Height:      -int32(height),
		Planes:      1,
		BitCount:    32,
		Compression: biBitFields,
		RedMask:     0x00ff0000,
		GreenMask:   0x0000ff00,
		BlueMask:    0x000000ff,
		AlphaMask:   0xff000000,
		CSType:      lcsWindowsColorSpace,
	}

	var bits unsafe.Pointer
	colorBitmap, _, err := procCreateDIBSection.Call(0, uintptr(unsafe.Pointer(&header)), dibRGBColors, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if colorBitmap == 0 {
		return 0, fmt.Errorf("dfw: create icon color bitmap: %w", err)
	}
	defer procDeleteObject.Call(colorBitmap)

	copyBGRA(bits, rgba)

	maskBytes := make([]byte, ((width+15)/16)*2*height)
	maskBitmap, _, err := procCreateBitmap.Call(uintptr(width), uintptr(height), 1, 1, uintptr(unsafe.Pointer(&maskBytes[0])))
	if maskBitmap == 0 {
		return 0, fmt.Errorf("dfw: create icon mask bitmap: %w", err)
	}
	defer procDeleteObject.Call(maskBitmap)

	info := iconInfo{
		Icon:  1,
		Mask:  maskBitmap,
		Color: colorBitmap,
	}
	hicon, _, err := procCreateIconIndirect.Call(uintptr(unsafe.Pointer(&info)))
	if hicon == 0 {
		return 0, fmt.Errorf("dfw: create icon: %w", err)
	}
	return hicon, nil
}

func copyBGRA(dst unsafe.Pointer, src *image.NRGBA) {
	pixels := unsafe.Slice((*byte)(dst), len(src.Pix))
	i := 0
	for y := 0; y < src.Bounds().Dy(); y++ {
		row := src.Pix[y*src.Stride:]
		for x := 0; x < src.Bounds().Dx(); x++ {
			p := row[x*4:]
			pixels[i+0] = p[2]
			pixels[i+1] = p[1]
			pixels[i+2] = p[0]
			pixels[i+3] = p[3]
			i += 4
		}
	}
}

func drawNearest(dst *image.NRGBA, src *image.NRGBA) {
	srcBounds := src.Bounds()
	dstBounds := dst.Bounds()
	for y := 0; y < dstBounds.Dy(); y++ {
		srcY := srcBounds.Min.Y + y*srcBounds.Dy()/dstBounds.Dy()
		for x := 0; x < dstBounds.Dx(); x++ {
			srcX := srcBounds.Min.X + x*srcBounds.Dx()/dstBounds.Dx()
			si := src.PixOffset(srcX, srcY)
			di := dst.PixOffset(x, y)
			copy(dst.Pix[di:di+4], src.Pix[si:si+4])
		}
	}
}

func systemMetric(metric int) int {
	value, _, _ := procGetSystemMetrics.Call(uintptr(metric))
	return int(value)
}
