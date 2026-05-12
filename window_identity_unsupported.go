//go:build !linux

package dfw

import "unsafe"

func prepareNativeWindowIdentity(_ string) {}

func applyNativeWindowIdentity(_ unsafe.Pointer, _ string) {}
