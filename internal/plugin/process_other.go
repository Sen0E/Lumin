//go:build !linux && !darwin && !windows

package plugin

import "os/exec"

func runProcess(cmd *exec.Cmd) error { return cmd.Run() }
