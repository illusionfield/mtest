//go:build !windows

package process

import (
	"os/exec"
	"testing"
)

func TestConfigureSetsProcessGroup(t *testing.T) {
	cmd := exec.Command("sleep", "1")

	Configure(cmd)

	if cmd.SysProcAttr == nil || !cmd.SysProcAttr.Setpgid {
		t.Fatalf("expected SysProcAttr.Setpgid to be true, got %+v", cmd.SysProcAttr)
	}
}

func TestTerminateWithNilCommand(t *testing.T) {
	if err := Terminate(nil); err != nil {
		t.Fatalf("expected nil error terminating nil command, got %v", err)
	}
}

func TestTerminateWithNilProcess(t *testing.T) {
	cmd := exec.Command("sleep", "1")
	if err := Terminate(cmd); err != nil {
		t.Fatalf("expected nil error terminating command without process, got %v", err)
	}
}
