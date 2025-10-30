//go:build windows

package process

import (
	"errors"
	"os"
	"os/exec"
	"syscall"

	log "github.com/sirupsen/logrus"
)

// Configure ensures the command starts in its own process group on Windows.
func Configure(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	log.Trace("configuring Windows process attributes for command")
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags = syscall.CREATE_NEW_PROCESS_GROUP
}

// Terminate forcefully kills the command process on Windows platforms.
func Terminate(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	log.WithField("pid", cmd.Process.Pid).Debug("terminating process on Windows")
	if err := cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return err
	}
	return nil
}
