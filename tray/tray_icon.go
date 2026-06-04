//go:build !windows

package tray

func trayIconBytes(iconPNG []byte) ([]byte, error) {
	if err := validateTrayIconPNG(iconPNG); err != nil {
		return nil, err
	}
	return iconPNG, nil
}
