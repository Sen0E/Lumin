package plugin

import (
	"context"
	"os/exec"
	"strconv"
	"syscall"
	"time"
)

func configureProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000}
	cmd.Cancel = func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		kill := exec.CommandContext(ctx, "taskkill.exe", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid))
		kill.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000}
		_ = kill.Run()
		return cmd.Process.Kill()
	}
}
