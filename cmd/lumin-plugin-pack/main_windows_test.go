package main

import "testing"

func TestWindowsOutputVolumes(t *testing.T) {
	// These paths reproduce GitHub Actions' C: temp / D: workspace layout.
	for _, out := range []string{
		`D:\a\Lumin\Lumin\build\plugin.luminext`,
		`C:\packages\plugin.luminext`,
		`\\server\share\packages\plugin.luminext`,
	} {
		if err := validateOutputLocation(`C:\Users\runner\Temp\plugin`, out); err != nil {
			t.Errorf("outside output %q: %v", out, err)
		}
	}
	for _, out := range []string{
		`C:\Users\runner\Temp\plugin\out.luminext`,
		`c:\users\runner\temp\plugin\nested\out.luminext`,
	} {
		if err := validateOutputLocation(`C:\Users\runner\Temp\plugin`, out); err == nil {
			t.Errorf("accepted nested output %q", out)
		}
	}
}
