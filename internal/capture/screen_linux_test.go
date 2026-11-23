package capture

import "testing"

func TestPortalURI(t *testing.T) {
	for _, uri := range []string{"https://example.com/x", "file://remote/tmp/x", "file:relative", "file:///tmp/x?secret=x", "file:///tmp/x#fragment"} {
		if _, err := fileURI(uri); err == nil {
			t.Fatalf("accepted %s", uri)
		}
	}
	got, err := fileURI("file:///tmp/a%20b.png")
	if err != nil || got != "/tmp/a b.png" {
		t.Fatalf("%q %v", got, err)
	}
}
