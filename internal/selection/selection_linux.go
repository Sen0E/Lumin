package selection

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/godbus/dbus/v5"
)

const accessibleInterface = "org.a11y.atspi.Accessible"
const textInterface = "org.a11y.atspi.Text"
const focusedState = uint32(1 << 12)
const activeState = uint32(1 << 1)
const defunctState = uint32(1 << 6)

type objectRef struct {
	Bus  string
	Path dbus.ObjectPath
}
type accessibleReader interface {
	children(context.Context, objectRef) []objectRef
	state(context.Context, objectRef) uint32
	selected(context.Context, objectRef) (string, error)
}
type atspiReader struct{ conn *dbus.Conn }

// Wayland PRIMARY supports editors such as Monaco without relying on their
// accessibility mode. AT-SPI remains available on other desktops/backends.
func readNative(ctx context.Context) (string, error) {
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		return chooseLinuxSelection(ctx, readWaylandPrimary, readAccessible)
	}
	return readAccessible(ctx)
}

func readAccessible(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	session, err := dbus.ConnectSessionBus(dbus.WithContext(ctx))
	if err != nil {
		return "", fmt.Errorf("%w：无法连接桌面会话总线", ErrUnavailable)
	}
	defer session.Close()
	var address string
	err = session.Object("org.a11y.Bus", "/org/a11y/bus").CallWithContext(ctx, "org.a11y.Bus.GetAddress", 0).Store(&address)
	if err != nil || address == "" {
		return "", fmt.Errorf("%w：桌面辅助功能总线不可用", ErrUnavailable)
	}
	conn, err := dbus.Connect(address, dbus.WithContext(ctx))
	if err != nil {
		return "", fmt.Errorf("%w：无法连接辅助功能总线", ErrUnavailable)
	}
	defer conn.Close()
	reader := atspiReader{conn}
	root := objectRef{"org.a11y.atspi.Registry", "/org/a11y/atspi/accessible/root"}
	return readActiveSelection(ctx, reader, root)
}

func (r atspiReader) call(ctx context.Context, ref objectRef, method string, args ...any) *dbus.Call {
	// A broken application must not consume the entire operation's deadline.
	child, cancel := context.WithTimeout(ctx, 250*time.Millisecond)
	defer cancel()
	return r.conn.Object(ref.Bus, ref.Path).CallWithContext(child, method, 0, args...)
}
func (r atspiReader) children(ctx context.Context, ref objectRef) []objectRef {
	var children []objectRef
	if ref.Bus == "" || !ref.Path.IsValid() {
		return nil
	}
	_ = r.call(ctx, ref, accessibleInterface+".GetChildren").Store(&children)
	if len(children) > 2048 {
		children = children[:2048]
	}
	return children
}
func (r atspiReader) state(ctx context.Context, ref objectRef) uint32 {
	var states []uint32
	if err := r.call(ctx, ref, accessibleInterface+".GetState").Store(&states); err != nil || len(states) == 0 {
		return 0
	}
	return states[0]
}
func (r atspiReader) selected(ctx context.Context, ref objectRef) (string, error) {
	var role uint32
	if err := r.call(ctx, ref, accessibleInterface+".GetRole").Store(&role); err != nil {
		return "", ErrUnavailable
	}
	if role == 40 {
		return "", ErrEmpty
	} // ATSPI_ROLE_PASSWORD_TEXT
	var count int32
	if err := r.call(ctx, ref, textInterface+".GetNSelections").Store(&count); err != nil {
		return "", ErrUnavailable
	}
	if count == 0 {
		return "", ErrEmpty
	}
	if count != 1 {
		return "", fmt.Errorf("%w：请选择一段连续文字", ErrUnavailable)
	}
	var start, end int32
	if err := r.call(ctx, ref, textInterface+".GetSelection", int32(0)).Store(&start, &end); err != nil {
		return "", ErrUnavailable
	}
	if start < 0 || end <= start {
		return "", ErrEmpty
	}
	if int64(end)-int64(start) > MaxCharacters {
		return "", ErrTooLarge
	}
	var text string
	if err := r.call(ctx, ref, textInterface+".GetText", start, end).Store(&text); err != nil {
		return "", ErrUnavailable
	}
	return validate(text)
}

func readActiveSelection(ctx context.Context, r accessibleReader, root objectRef) (string, error) {
	// Find active top-level surfaces first. Never scan selections in inactive
	// apps: their highlighted text can survive switching to another window.
	var queue []objectRef
	for _, app := range r.children(ctx, root) {
		for _, frame := range r.children(ctx, app) {
			if r.state(ctx, frame)&activeState != 0 {
				queue = append(queue, frame)
			}
		}
	}
	type node struct {
		ref     objectRef
		parents []objectRef
	}
	pending := make([]node, 0, len(queue))
	for _, ref := range queue {
		pending = append(pending, node{ref: ref})
	}
	visited := map[objectRef]bool{}
	for len(pending) > 0 && len(visited) < 2048 {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		n := pending[0]
		pending = pending[1:]
		if visited[n.ref] {
			continue
		}
		visited[n.ref] = true
		state := r.state(ctx, n.ref)
		if state&defunctState != 0 {
			continue
		}
		if state&focusedState != 0 {
			text, err := r.selected(ctx, n.ref)
			if err != ErrUnavailable {
				return text, err
			}
			// Some browser focus objects delegate Text to an ancestor. Never
			// return a selection from an unrelated, unfocused sibling.
			for i := len(n.parents) - 1; i >= 0; i-- {
				text, err = r.selected(ctx, n.parents[i])
				if err != ErrUnavailable {
					return text, err
				}
			}
			return "", ErrUnavailable
		}
		parents := append(append([]objectRef(nil), n.parents...), n.ref)
		for _, child := range r.children(ctx, n.ref) {
			pending = append(pending, node{child, parents})
		}
		if len(pending) > 4096 {
			return "", ErrUnavailable
		}
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return "", ErrUnavailable
}
