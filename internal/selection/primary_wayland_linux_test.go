package selection

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// Raw nonblocking sockets avoid net.FileConn's platform-introspection
// sockopts, unavailable in the restricted test runner. Payloads and FDs still
// pass through real AF_UNIX sockets, not a mocked clipboard API.
type fixtureSocket struct{ *os.File }

func (s fixtureSocket) sendFD(data []byte, fd int) (n int, err error) {
	raw, err := s.SyscallConn()
	if err != nil {
		return 0, err
	}
	var callErr error
	err = raw.Write(func(sock uintptr) bool {
		n, callErr = unix.SendmsgN(int(sock), data, unix.UnixRights(fd), nil, 0)
		return callErr != unix.EAGAIN
	})
	if err != nil {
		return 0, err
	}
	return n, callErr
}
func (s fixtureSocket) receive(data, oob []byte) (n, on int, err error) {
	raw, err := s.SyscallConn()
	if err != nil {
		return 0, 0, err
	}
	var callErr error
	err = raw.Read(func(sock uintptr) bool {
		n, on, _, _, callErr = unix.Recvmsg(int(sock), data, oob, 0)
		return callErr != unix.EAGAIN
	})
	if err != nil {
		return 0, 0, err
	}
	return n, on, callErr
}
func waylandSocketPair(t *testing.T) (fixtureSocket, fixtureSocket) {
	t.Helper()
	fds, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_STREAM|unix.SOCK_CLOEXEC|unix.SOCK_NONBLOCK, 0)
	if err != nil {
		t.Fatal(err)
	}
	client := fixtureSocket{os.NewFile(uintptr(fds[0]), "wayland-client")}
	server := fixtureSocket{os.NewFile(uintptr(fds[1]), "wayland-server")}
	t.Cleanup(func() { client.Close(); server.Close() })
	return client, server
}

// The fixture is a compositor peer, including real SCM_RIGHTS FD transfer.
// It advertises a different CLIPBOARD first, to catch accidental old-text reads.
func compositorFixture(ctx context.Context, conn fixtureSocket, protocol, mode, text string) error {
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	p := wlPeer{conn: conn, ctx: ctx}
	for _, want := range []struct {
		object uint32
		opcode uint16
	}{{1, 1}, {1, 0}} {
		e, err := readWaylandEvent(conn)
		if err != nil {
			return err
		}
		if e.object != want.object || e.opcode != want.opcode {
			return errors.New("invalid registry handshake")
		}
	}
	global := func(name uint32, iface string, version uint32) error {
		data := append(wlUint(name), wlString(iface)...)
		data = append(data, wlUint(version)...)
		msg := wlMessage(2, 0, data)
		// Deliberately split messages inside the header to exercise framing.
		if _, err := conn.Write(msg[:3]); err != nil {
			return err
		}
		_, err := conn.Write(msg[3:])
		return err
	}
	if err := global(11, "wl_seat", 9); err != nil {
		return err
	}
	version := uint32(1)
	if protocol == "zwlr_data_control_manager_v1" {
		version = 2
	}
	if protocol != "" {
		if err := global(12, protocol, version); err != nil {
			return err
		}
	}
	if err := p.send(3, 0, wlUint(0)); err != nil {
		return err
	}
	if protocol == "" {
		return nil
	}
	for i := 0; i < 4; i++ {
		e, err := readWaylandEvent(conn)
		if err != nil {
			return err
		}
		if i < 2 {
			if e.object != 2 || e.opcode != 0 {
				return errors.New("invalid registry bind")
			}
			d := wlDecoder{data: e.data}
			d.uint()
			iface := d.text()
			v := d.uint()
			id := d.uint()
			if d.err != nil || i == 0 && (iface != "wl_seat" || v != 1 || id != 4) || i == 1 && (iface != protocol || v != version || id != 5) {
				return errors.New("wrong interface version or ID")
			}
		} else if i == 2 {
			if e.object != 5 || e.opcode != 1 {
				return errors.New("missing get_data_device")
			}
		} else if e.object != 1 || e.opcode != 0 {
			return errors.New("missing device sync")
		}
	}
	sendOffer := func(id uint32, mime string, opcode uint16) error {
		if err := p.send(6, 0, wlUint(id)); err != nil {
			return err
		}
		if err := p.send(id, 0, wlString(mime)); err != nil {
			return err
		}
		return p.send(6, opcode, wlUint(id))
	}
	if err := sendOffer(0xff000001, "text/plain;charset=utf-8", 1); err != nil {
		return err
	}
	switch mode {
	case "unsupported":
		return p.send(7, 0, wlUint(0))
	case "empty":
		return p.send(6, 3, wlUint(0))
	case "image":
		return sendOffer(0xff000002, "image/png", 3)
	}
	if err := sendOffer(0xff000002, "text/plain;charset=utf-8", 3); err != nil {
		return err
	}
	// recvmsg on the header collects the FD; receive has no FD payload word.
	header := [8]byte{}
	var rights []int
	defer func() {
		for _, fd := range rights {
			unix.Close(fd)
		}
	}()
	for offset := 0; offset < 8; {
		oob := make([]byte, unix.CmsgSpace(4))
		n, on, err := conn.receive(header[offset:], oob)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrUnexpectedEOF
		}
		offset += n
		messages, err := unix.ParseSocketControlMessage(oob[:on])
		if err != nil {
			return err
		}
		for _, m := range messages {
			fd, err := unix.ParseUnixRights(&m)
			if err != nil {
				return err
			}
			rights = append(rights, fd...)
		}
	}
	object := binaryUint(header[:4])
	tag := binaryUint(header[4:])
	size := int(tag >> 16)
	if object != 0xff000002 || uint16(tag) != 0 || len(rights) != 1 || size < 8 {
		return errors.New("read CLIPBOARD instead of PRIMARY, or invalid FD transfer")
	}
	payload := make([]byte, size-8)
	if _, err := io.ReadFull(conn, payload); err != nil {
		return err
	}
	d := wlDecoder{data: payload}
	if d.text() != "text/plain;charset=utf-8" || d.err != nil || len(d.data) != 0 {
		return errors.New("invalid MIME/FD encoding")
	}
	if mode == "stall" {
		<-ctx.Done()
		return nil
	}
	_, err := unix.Write(rights[0], []byte(text))
	return err
}

func binaryUint(b []byte) uint32 { return binary.NativeEndian.Uint32(b) }

func TestWaylandPrimaryRealFDTransferAndClipboardIsolation(t *testing.T) {
	for _, protocol := range []string{"zwlr_data_control_manager_v1", "ext_data_control_manager_v1", ""} {
		for _, mode := range []string{"text", "empty", "unsupported", "image"} {
			t.Run(protocol+"/"+mode, func(t *testing.T) {
				client, server := waylandSocketPair(t)
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				done := make(chan error, 1)
				go func() {
					defer server.Close()
					done <- compositorFixture(ctx, server, protocol, mode, "VS Code 中 fresh selected text")
				}()
				text, err := readWaylandPrimaryConn(ctx, client)
				want := ErrUnavailable
				if protocol != "" && mode == "text" {
					want = nil
					if text != "VS Code 中 fresh selected text" {
						t.Errorf("wrong selection: %q", text)
					}
				}
				if protocol != "" && mode == "empty" {
					want = ErrEmpty
				}
				if !errors.Is(err, want) {
					t.Errorf("got %q %v, want %v", text, err, want)
				}
				if err := <-done; err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
func TestWaylandPrimaryCanceledDuringFDRead(t *testing.T) {
	client, server := waylandSocketPair(t)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		defer server.Close()
		done <- compositorFixture(ctx, server, "ext_data_control_manager_v1", "stall", "")
	}()
	_, err := readWaylandPrimaryConn(ctx, client)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
func TestLinuxPrimaryFallbackPolicy(t *testing.T) {
	for _, problem := range []error{nil, ErrEmpty, ErrTooLarge, ErrUnavailable} {
		called := false
		primary := func(context.Context) (string, error) { return "primary", problem }
		accessible := func(context.Context) (string, error) { called = true; return "accessible", nil }
		text, err := chooseLinuxSelection(context.Background(), primary, accessible)
		if problem == ErrUnavailable {
			if !called || err != nil || text != "accessible" {
				t.Fatal("missing accessibility fallback")
			}
		} else if called || !errors.Is(err, problem) {
			t.Fatal("empty selection replaced with another source")
		}
	}
}
func TestWaylandDecoderRejectsMalformedStrings(t *testing.T) {
	for _, data := range [][]byte{wlUint(0), wlUint(^uint32(0)), append(wlUint(2), []byte("xx\x00\x00")...)} {
		d := wlDecoder{data: data}
		d.text()
		if d.err == nil {
			t.Fatal("accepted invalid wire string")
		}
	}
	if _, err := validate(strings.Repeat("x", MaxCharacters+1)); !errors.Is(err, ErrTooLarge) {
		t.Fatal(err)
	}
}
