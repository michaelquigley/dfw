//go:build linux

package webview

import (
	"debug/elf"
	"os"
	"testing"
)

func TestJSCMissingPrivateAPIIsANoop(t *testing.T) {
	if got := configureJSCSignalEntry(nil); got != 0 {
		t.Fatalf("missing private API selected signal %d", got)
	}
}

func TestJSCPrivateAPIIsNotALinkDependency(t *testing.T) {
	path, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	binary, err := elf.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = binary.Close() }()
	imports, err := binary.ImportedSymbols()
	if err != nil {
		t.Fatal(err)
	}
	for _, symbol := range imports {
		if symbol.Name == "JSConfigureSignalForGC" {
			t.Fatal("private API became a required dynamic import")
		}
	}
}
