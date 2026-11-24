package main

import "golang.org/x/sys/windows"

const libraryName = "rapidocrlite.dll"

func openLibrary(path string) (func(string) (uintptr, error), func(), error) {
	h, err := windows.LoadLibraryEx(path, 0, windows.LOAD_LIBRARY_SEARCH_DLL_LOAD_DIR|windows.LOAD_LIBRARY_SEARCH_SYSTEM32)
	if err != nil {
		return nil, nil, err
	}
	return func(s string) (uintptr, error) { return windows.GetProcAddress(h, s) }, func() { _ = windows.FreeLibrary(h) }, nil
}
