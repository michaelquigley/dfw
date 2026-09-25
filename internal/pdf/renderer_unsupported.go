//go:build !linux

package pdf

import "context"

// Render is not available on this platform yet.
func Render(context.Context, string, Document) ([]byte, error) {
	return nil, Failure(Unsupported, "PDF export is currently supported on Linux only")
}
