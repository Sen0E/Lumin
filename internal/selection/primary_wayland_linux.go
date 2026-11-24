package selection

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/unix"
)

// Read only primary_selection via ext-data-control-v1 or wlr-data-control v2.
// This is native Wayland IPC, not wl-paste or the regular clipboard. A private
// connection per request avoids retaining stale offers after an owner changes.
func readWaylandPrimary(ctx context.Context) (string, error) {
	display := os.Getenv("WAYLAND_DISPLAY")
	if display == "" {
		return "", ErrUnavailable
	}
	if !filepath.IsAbs(display) {
		dir := os.Getenv("XDG_RUNTIME_DIR")
		if dir == "" {
			return "", ErrUnavailable
		}
		display = filepath.Join(dir, display)
	}
	// Monaco publishes Linux selection through a 100ms scheduled update.
	// Keep source focus while allowing that update to reach the compositor.
	timer := time.NewTimer(120 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-timer.C:
	}
	dialer := net.Dialer{}
	conn, err := dialer.DialContext(ctx, "unix", display)
	if err != nil {
		return "", ErrUnavailable
	}
	defer conn.Close()
	return readWaylandPrimaryConn(ctx, waylandSocket{conn.(*net.UnixConn)})
}

type wlEvent struct {
	object uint32
	opcode uint16
	data   []byte
}
type wlConnection interface {
	io.ReadWriteCloser
	SetDeadline(time.Time) error
	sendFD([]byte, int) (int, error)
}
type waylandSocket struct{ *net.UnixConn }

func (s waylandSocket) sendFD(data []byte, fd int) (int, error) {
	n, _, err := s.WriteMsgUnix(data, unix.UnixRights(fd), nil)
	return n, err
}

type wlPeer struct {
	conn wlConnection
	ctx  context.Context
}

func wlUint(n uint32) []byte { return binary.NativeEndian.AppendUint32(nil, n) }
func wlString(s string) []byte {
	out := wlUint(uint32(len(s) + 1))
	out = append(out, []byte(s)...)
	out = append(out, 0)
	for len(out)%4 != 0 {
		out = append(out, 0)
	}
	return out
}
func wlMessage(object uint32, opcode uint16, data []byte) []byte {
	out := wlUint(object)
	out = binary.NativeEndian.AppendUint32(out, uint32(len(data)+8)<<16|uint32(opcode))
	return append(out, data...)
}
func (p wlPeer) send(object uint32, opcode uint16, data []byte) error {
	_, err := io.Copy(p.conn, bytes.NewReader(wlMessage(object, opcode, data)))
	return err
}
func readWaylandEvent(reader io.Reader) (wlEvent, error) {
	header := [8]byte{}
	if _, err := io.ReadFull(reader, header[:]); err != nil {
		return wlEvent{}, err
	}
	tag := binary.NativeEndian.Uint32(header[4:])
	size := int(tag >> 16)
	if size < 8 || size%4 != 0 {
		return wlEvent{}, ErrUnavailable
	}
	ev := wlEvent{binary.NativeEndian.Uint32(header[:4]), uint16(tag), make([]byte, size-8)}
	if _, err := io.ReadFull(reader, ev.data); err != nil {
		return wlEvent{}, err
	}
	return ev, nil
}
func (p wlPeer) next() (wlEvent, error) {
	ev, err := readWaylandEvent(p.conn)
	if err == nil && ev.object == 1 && ev.opcode == 0 {
		return wlEvent{}, ErrUnavailable
	}
	return ev, err
}

type wlDecoder struct {
	data []byte
	err  error
}

func (d *wlDecoder) uint() uint32 {
	if len(d.data) < 4 {
		d.err = ErrUnavailable
		return 0
	}
	n := binary.NativeEndian.Uint32(d.data[:4])
	d.data = d.data[4:]
	return n
}
func (d *wlDecoder) text() string {
	n := d.uint()
	padded := (uint64(n) + 3) &^ uint64(3)
	if n == 0 || padded > uint64(len(d.data)) {
		d.err = ErrUnavailable
		return ""
	}
	if d.data[n-1] != 0 {
		d.err = ErrUnavailable
		return ""
	}
	s := string(d.data[:n-1])
	d.data = d.data[padded:]
	return s
}

type wlGlobal struct {
	name, version uint32
	iface         string
}

func readWaylandPrimaryConn(ctx context.Context, conn wlConnection) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	text, err := waylandPrimary(wlPeer{conn: conn, ctx: ctx})
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	// FD polling and context timers can observe the same deadline in either
	// order. Normalize an expired read even before the context timer runs.
	if err != nil {
		if end, ok := ctx.Deadline(); ok && !time.Now().Before(end) {
			return "", context.DeadlineExceeded
		}
	}
	return text, err
}
func waylandPrimary(p wlPeer) (string, error) {
	// Client IDs: display=1, registry=2, registry sync=3, seat=4,
	// manager=5, device=6, device sync=7. Server owns the offer IDs.
	if err := p.send(1, 1, wlUint(2)); err != nil {
		return "", err
	}
	if err := p.send(1, 0, wlUint(3)); err != nil {
		return "", err
	}
	var seat, manager wlGlobal
	for count := 0; ; count++ {
		if count > 4096 {
			return "", ErrUnavailable
		}
		e, err := p.next()
		if err != nil {
			return "", err
		}
		if e.object == 3 && e.opcode == 0 {
			break
		}
		if e.object != 2 || e.opcode != 0 {
			continue
		}
		d := wlDecoder{data: e.data}
		g := wlGlobal{name: d.uint()}
		g.iface = d.text()
		g.version = d.uint()
		if d.err != nil {
			return "", d.err
		}
		if g.iface == "wl_seat" && seat.name == 0 {
			seat = g
		}
		if g.iface == "ext_data_control_manager_v1" {
			g.version = 1
			manager = g
		}
		if g.iface == "zwlr_data_control_manager_v1" && g.version >= 2 && manager.iface != "ext_data_control_manager_v1" {
			g.version = 2
			manager = g
		}
	}
	if seat.name == 0 || manager.name == 0 {
		return "", ErrUnavailable
	}
	bind := func(g wlGlobal, id, version uint32) error {
		data := append(wlUint(g.name), wlString(g.iface)...)
		data = append(data, wlUint(version)...)
		data = append(data, wlUint(id)...)
		return p.send(2, 0, data)
	}
	if err := bind(seat, 4, 1); err != nil {
		return "", err
	}
	if err := bind(manager, 5, manager.version); err != nil {
		return "", err
	}
	if err := p.send(5, 1, append(wlUint(6), wlUint(4)...)); err != nil {
		return "", err
	}
	if err := p.send(1, 0, wlUint(7)); err != nil {
		return "", err
	}
	offers := map[uint32][]string{}
	for count := 0; count < 4096; count++ {
		e, err := p.next()
		if err != nil {
			return "", err
		}
		d := wlDecoder{data: e.data}
		if e.object == 7 && e.opcode == 0 {
			return "", ErrUnavailable
		} // No PRIMARY support.
		if e.object == 6 {
			switch e.opcode {
			case 0:
				id := d.uint()
				if d.err != nil || id == 0 || len(offers) >= 128 {
					return "", ErrUnavailable
				}
				offers[id] = nil
			case 1: // Regular clipboard: deliberately ignore, never request it.
			case 2:
				return "", ErrUnavailable
			case 3:
				id := d.uint()
				if d.err != nil {
					return "", d.err
				}
				if id == 0 {
					return "", ErrEmpty
				}
				for _, mime := range []string{"text/plain;charset=utf-8", "UTF8_STRING", "text/plain"} {
					for _, offered := range offers[id] {
						if offered == mime {
							return receiveWaylandText(p, id, mime)
						}
					}
				}
				return "", ErrUnavailable
			}
		} else if _, ok := offers[e.object]; ok && e.opcode == 0 {
			mime := d.text()
			if d.err != nil || len(offers[e.object]) >= 128 {
				return "", ErrUnavailable
			}
			offers[e.object] = append(offers[e.object], mime)
		}
	}
	return "", ErrUnavailable
}
func receiveWaylandText(p wlPeer, offer uint32, mime string) (string, error) {
	reader, writer, err := os.Pipe()
	if err != nil {
		return "", err
	}
	defer reader.Close()
	defer writer.Close()
	stop := context.AfterFunc(p.ctx, func() { _ = reader.Close() })
	defer stop()
	// A stalled source must not hold up the hotkey. Cancellation also closes
	// the pollable reader, independently of the Wayland connection.
	deadline := time.Now().Add(time.Second)
	if end, ok := p.ctx.Deadline(); ok && end.Before(deadline) {
		deadline = end
	}
	_ = reader.SetReadDeadline(deadline)
	data := wlMessage(offer, 0, wlString(mime))
	n, err := p.conn.sendFD(data, int(writer.Fd()))
	_ = writer.Close()
	if err != nil {
		return "", err
	}
	if n != len(data) {
		return "", io.ErrShortWrite
	}
	b, err := io.ReadAll(io.LimitReader(reader, MaxCharacters*4+1))
	if err != nil {
		return "", err
	}
	if len(b) > MaxCharacters*4 {
		return "", ErrTooLarge
	}
	return validate(string(b))
}

func chooseLinuxSelection(ctx context.Context, primary, accessible func(context.Context) (string, error)) (string, error) {
	start := time.Now()
	primaryCtx, cancel := context.WithTimeout(ctx, 1200*time.Millisecond)
	text, err := primary(primaryCtx)
	cancel()
	Trace("wayland-primary", time.Since(start), err)
	if err == nil || errors.Is(err, ErrEmpty) || errors.Is(err, ErrTooLarge) {
		return text, err
	}
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	return accessible(ctx)
}
