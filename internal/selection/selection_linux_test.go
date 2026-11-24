package selection

import (
	"context"
	"errors"
	"testing"
)

type fakeAccessible struct {
	tree   map[objectRef][]objectRef
	states map[objectRef]uint32
	text   map[objectRef]string
	empty  map[objectRef]bool
	read   []objectRef
}

func (f *fakeAccessible) children(_ context.Context, r objectRef) []objectRef { return f.tree[r] }
func (f *fakeAccessible) state(_ context.Context, r objectRef) uint32         { return f.states[r] }
func (f *fakeAccessible) selected(_ context.Context, r objectRef) (string, error) {
	f.read = append(f.read, r)
	if text, ok := f.text[r]; ok {
		return text, nil
	}
	if f.empty[r] {
		return "", ErrEmpty
	}
	return "", ErrUnavailable
}
func TestActiveFocusSelectionNeverReadsUnrelatedText(t *testing.T) {
	root := objectRef{"registry", "/root"}
	app := objectRef{"app", "/app"}
	inactive := objectRef{"app", "/inactive"}
	frame := objectRef{"app", "/frame"}
	stale := objectRef{"app", "/old"}
	focus := objectRef{"app", "/focus"}
	f := &fakeAccessible{
		tree:   map[objectRef][]objectRef{root: {app}, app: {inactive, frame}, frame: {stale, focus}},
		states: map[objectRef]uint32{frame: activeState, focus: focusedState},
		text:   map[objectRef]string{inactive: "old inactive selection", stale: "old sibling selection", focus: "new selection"},
		empty:  map[objectRef]bool{},
	}
	text, err := readActiveSelection(context.Background(), f, root)
	if err != nil || text != "new selection" {
		t.Fatalf("%q %v", text, err)
	}
	for _, r := range f.read {
		if r == inactive || r == stale {
			t.Fatal("read unrelated selection")
		}
	}
	delete(f.text, focus)
	f.empty[focus] = true
	if _, err := readActiveSelection(context.Background(), f, root); !errors.Is(err, ErrEmpty) {
		t.Fatalf("empty focus used stale selection: %v", err)
	}
	delete(f.empty, focus)
	f.text[frame] = "document selection"
	text, err = readActiveSelection(context.Background(), f, root)
	if err != nil || text != "document selection" {
		t.Fatalf("ancestor delegation: %q %v", text, err)
	}
}
func TestAccessibleCycleAndMissingFocusAreBounded(t *testing.T) {
	root := objectRef{"registry", "/root"}
	app := objectRef{"app", "/app"}
	frame := objectRef{"app", "/frame"}
	f := &fakeAccessible{tree: map[objectRef][]objectRef{root: {app}, app: {frame}, frame: {frame}}, states: map[objectRef]uint32{frame: activeState}}
	if _, err := readActiveSelection(context.Background(), f, root); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
}
