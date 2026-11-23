//go:build !linux && !darwin && !windows

package plugin

import "os/exec"

func configureProcess(*exec.Cmd) {}
