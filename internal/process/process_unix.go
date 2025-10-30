//go:build !windows

package process

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
)

// Configure ensures the command starts in its own process group on Unix-like platforms.
func Configure(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	log.Trace("configuring Unix process attributes for command")
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
}

// Terminate issues a SIGTERM followed by SIGKILL to the command's process group.
func Terminate(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}

	log.WithField("pid", cmd.Process.Pid).Debug("terminating process tree on Unix")

	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err == nil && pgid > 0 {
		log.WithField("pgid", pgid).Trace("sending SIGTERM to process group")
		if err := syscall.Kill(-pgid, syscall.SIGTERM); err != nil && !errors.Is(err, syscall.ESRCH) {
			return err
		}
		time.Sleep(300 * time.Millisecond)
		log.WithField("pgid", pgid).Trace("sending SIGKILL to process group")
		if err := syscall.Kill(-pgid, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
			return err
		}
		return nil
	}

	log.Trace("sending SIGTERM to individual process")
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil && !errors.Is(err, os.ErrProcessDone) && !errors.Is(err, syscall.ESRCH) {
		log.Trace("SIGTERM failed, escalating to SIGKILL")
		if err := cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) && !errors.Is(err, syscall.ESRCH) {
			return err
		}
	}
	return nil
}
