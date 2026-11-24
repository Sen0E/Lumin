package plugin

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestWindowsJobHelper(t *testing.T) {
	mode := os.Getenv("LUMIN_JOB_HELPER")
	if mode == "" {
		t.Skip("subprocess helper")
	}
	if mode == "parent" {
		exe, err := os.Executable()
		if err != nil {
			os.Exit(2)
		}
		child := exec.Command(exe, "-test.run=^TestWindowsJobHelper$")
		child.Env = append(os.Environ(), "LUMIN_JOB_HELPER=child")
		if err = child.Start(); err != nil {
			os.Exit(3)
		}
		if err = os.WriteFile(os.Getenv("LUMIN_JOB_PID_FILE"), []byte(strconv.Itoa(child.Process.Pid)), 0600); err != nil {
			os.Exit(4)
		}
	}
	time.Sleep(time.Minute)
}
func TestWindowsJobCancellationKillsDescendants(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	pidFile := filepath.Join(t.TempDir(), "child.pid")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, exe, "-test.run=^TestWindowsJobHelper$")
	cmd.Env = append(os.Environ(), "LUMIN_JOB_HELPER=parent", "LUMIN_JOB_PID_FILE="+pidFile)
	done := make(chan error, 1)
	go func() { done <- runProcess(cmd) }()
	var pid int
	deadline := time.Now().Add(5 * time.Second)
	for pid == 0 && time.Now().Before(deadline) {
		if b, err := os.ReadFile(pidFile); err == nil {
			pid, _ = strconv.Atoi(string(b))
		}
		select {
		case err := <-done:
			t.Fatalf("helper exited before spawning descendant: %v", err)
		case <-time.After(10 * time.Millisecond):
		}
	}
	if pid == 0 {
		t.Fatal("descendant not started")
	}
	child, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(child)
	cancel()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("plugin did not stop")
	}
	status, err := windows.WaitForSingleObject(child, 3000)
	if err != nil || status != windows.WAIT_OBJECT_0 {
		t.Fatalf("descendant survived job cancellation: %d %v", status, err)
	}
}
