package capture

import (
	"context"
	"errors"
	"image"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

func Screen(ctx context.Context) (image.Image, error) {
	dir, err := os.MkdirTemp("", "lumin-screen-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "screen.png")
	// The filename is passed via the child environment, never interpolated into code.
	const script = `$ErrorActionPreference='Stop'; Add-Type -TypeDefinition 'using System.Runtime.InteropServices; public class LuminDpi { [DllImport("user32.dll")] public static extern bool SetProcessDPIAware(); }'; [LuminDpi]::SetProcessDPIAware() | Out-Null; Add-Type -AssemblyName System.Windows.Forms; Add-Type -AssemblyName System.Drawing; $r=[System.Windows.Forms.SystemInformation]::VirtualScreen; if ([long]$r.Width*$r.Height -gt 64000000) { throw 'screen too large' }; $b=New-Object System.Drawing.Bitmap($r.Width,$r.Height); $g=[System.Drawing.Graphics]::FromImage($b); try { $g.CopyFromScreen($r.Left,$r.Top,0,0,$b.Size); $b.Save($env:LUMIN_SCREEN_FILE,[System.Drawing.Imaging.ImageFormat]::Png) } finally { $g.Dispose(); $b.Dispose() }`
	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", script)
	cmd.Env = append(os.Environ(), "LUMIN_SCREEN_FILE="+path)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000}
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, errors.New("Windows 系统截图失败")
	}
	return ReadImage(path)
}
