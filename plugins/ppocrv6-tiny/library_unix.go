//go:build linux || darwin

package main

import (
	"github.com/ebitengine/purego"
	"runtime"
)

var libraryName = func() string {
	if runtime.GOOS == "darwin" {
		return "librapidocrlite.dylib"
	}
	return "librapidocrlite.so"
}()

func openLibrary(path string) (func(string) (uintptr, error), func(), error) {
	h, err := purego.Dlopen(path, purego.RTLD_NOW|purego.RTLD_LOCAL)
	if err != nil {
		return nil, nil, err
	}
	return func(s string) (uintptr, error) { return purego.Dlsym(h, s) }, func() { _ = purego.Dlclose(h) }, nil
}
