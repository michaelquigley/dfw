//go:build !linux

package webview

func configureNativeJSCSignal() int { return 0 }
