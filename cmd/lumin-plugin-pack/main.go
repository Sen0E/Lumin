// lumin-plugin-pack packs a prepared plugin directory into a validated .luminext.
package main

import (
	"archive/zip"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"lumin/internal/plugin"
)

// validateOutputLocation receives absolute paths. Files on different Windows
// volumes are necessarily outside one another; filepath.Rel rejects that case.
func validateOutputLocation(dir, out string) error {
	if !strings.EqualFold(filepath.VolumeName(dir), filepath.VolumeName(out)) {
		return nil
	}
	rel, err := filepath.Rel(dir, out)
	if err != nil {
		return err
	}
	if rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return errors.New("output must be outside the source directory")
	}
	return nil
}

func pack(dir, out string) error {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	out, err = filepath.Abs(out)
	if err != nil {
		return err
	}
	if filepath.Ext(out) != ".luminext" {
		return errors.New("output must end in .luminext")
	}
	if err := validateOutputLocation(dir, out); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(out), 0755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(out), ".pack-*.luminext")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	z := zip.NewWriter(f)
	count := 0
	err = filepath.WalkDir(dir, func(path string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if e.IsDir() {
			return nil
		}
		info, err := e.Info()
		if err != nil {
			return err
		}
		count++
		if !info.Mode().IsRegular() || info.Size() > 64<<20 || count > 128 {
			return errors.New("invalid file type, size or count")
		}
		name, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		w, err := z.Create(filepath.ToSlash(name))
		if err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		n, err := io.Copy(w, io.LimitReader(in, (64<<20)+1))
		if err != nil {
			return err
		}
		if n > 64<<20 {
			return errors.New("file grew beyond limit")
		}
		return nil
	})
	if err != nil {
		return err
	}
	if err = z.Close(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if _, err = plugin.Inspect(f.Name()); err != nil {
		return err
	}
	// A versioned output must be deliberately removed before rebuilding it.
	if _, err = os.Lstat(out); !errors.Is(err, os.ErrNotExist) {
		return errors.New("output already exists or cannot be inspected")
	}
	return os.Rename(f.Name(), out)
}
func main() {
	dir := flag.String("dir", "", "prepared plugin directory")
	out := flag.String("out", "", "output .luminext")
	flag.Parse()
	if *dir == "" || *out == "" {
		flag.Usage()
		os.Exit(2)
	}
	if err := pack(*dir, *out); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
