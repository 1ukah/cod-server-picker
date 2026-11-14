// Package winexec runs Windows tools without a console window.
// A GUI app that starts a console tool shows a black window for a short time if this is not done.
package winexec

import (
	"os/exec"
	"syscall"
)

const createNoWindow = 0x08000000

// Command is exec.Command with a hidden console window.
func Command(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
	return cmd
}
