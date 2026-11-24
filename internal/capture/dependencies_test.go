package capture

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Runtime capture must stay in-process on every platform, including files that
// cannot be executed by this build host. Explicit native OS APIs are allowed.
func TestCaptureHasNoCommandDependencies(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, i := range f.Imports {
			path, _ := strconv.Unquote(i.Path.Value)
			if path == "os/exec" {
				t.Fatalf("%s introduces external screenshot tools", file)
			}
		}
	}
}
func TestDistributedPluginsNeedNoCommandTools(t *testing.T) {
	for _, root := range []string{"../../plugins", "../../examples/plugins"} {
		err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
			if err != nil {
				return err
			}
			for _, i := range f.Imports {
				pkg, _ := strconv.Unquote(i.Path.Value)
				if pkg == "os/exec" {
					t.Errorf("maintained plugin %s depends on command execution", path)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
func TestHostExecOnlyStartsPackagedPlugins(t *testing.T) {
	roots := []string{"../plugin", "../desktop", "../bootstrap", "../ocr", "../ocrsetup", "../selection"}
	for _, root := range roots {
		files, err := filepath.Glob(filepath.Join(root, "*.go"))
		if err != nil {
			t.Fatal(err)
		}
		for _, file := range files {
			if strings.HasSuffix(file, "_test.go") {
				continue
			}
			f, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
			if err != nil {
				t.Fatal(err)
			}
			ast.Inspect(f, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				id, ok := sel.X.(*ast.Ident)
				if !ok || id.Name != "exec" {
					return true
				}
				if sel.Sel.Name == "Command" || sel.Sel.Name == "LookPath" {
					t.Errorf("unexpected command lookup in %s", file)
				}
				if sel.Sel.Name == "CommandContext" && filepath.Base(file) != "runtime.go" {
					t.Errorf("unpackaged command in %s", file)
				}
				return true
			})
		}
	}
}
