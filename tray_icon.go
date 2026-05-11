//go:build !windows

package dfw

func trayIconBytes(iconPNG []byte) ([]byte, error) {
	if err := validateTrayIconPNG(iconPNG); err != nil {
		return nil, err
	}
	return iconPNG, nil
}
