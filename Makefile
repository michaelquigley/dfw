.DEFAULT_GOAL := build
GOBIN ?= $(shell go env GOPATH)/bin

ifeq ($(filter-out /,$(abspath $(GOBIN))),)
$(error GOBIN is '$(GOBIN)'; it must name a real directory)
endif

.PHONY: build test test-pdf test-jsc clean

build:
	go install ./...

test:
	go test ./... -count=1
	go vet ./...

test-pdf:
	DFW_PDF_NATIVE=1 go test ./internal/pdf -run TestNative -count=1 -v

test-jsc:
	go test -tags dfw_jsc_probe ./webview -run TestNativeJavaScriptSignals -count=1 -v

clean:
	go clean ./...
	rm -f "$(GOBIN)"/*
