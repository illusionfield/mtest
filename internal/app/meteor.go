package app

import (
	"bufio"
	"context"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/illusionfield/mtest/internal/process"
)

func (a *App) startMeteor(ctx context.Context) error {
	args := []string{"test-packages", "--driver-package", "test-in-console", "-p", strconv.Itoa(a.port), a.cfg.PackageName}
	if a.cfg.Once {
		args = append(args, "--once")
	}
	if a.cfg.Release != "" {
		args = append(args, "--release", a.cfg.Release)
	}
	if a.cfg.Inspect {
		args = append(args, "--inspect")
	}
	if a.cfg.InspectBrk {
		args = append(args, "--inspect-brk")
	}
	if a.cfg.SettingsPath != "" {
		args = append(args, "--settings", a.cfg.SettingsPath)
	}
	if a.cfg.TestAppPath != "" {
		args = append(args, "--test-app-path", a.cfg.TestAppPath)
	}

	log.WithField("args", strings.Join(args, " ")).Trace("prepared meteor arguments")

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		comspec := os.Getenv("COMSPEC")
		if comspec == "" {
			comspec = "cmd.exe"
		}
		cmd = exec.CommandContext(ctx, comspec, append([]string{"/c", "meteor"}, args...)...)
	} else {
		cmd = exec.CommandContext(ctx, "meteor", args...)
	}

	log.WithField("command", cmd.String()).Debug("initialising meteor command")

	process.Configure(cmd)
	cmd.Stdin = os.Stdin

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}
	log.WithField("pid", cmd.Process.Pid).Debug("meteor process started")

	a.meteorCmd = cmd

	go a.streamOutput(ctx, stdout, os.Stdout, true)
	go a.streamOutput(ctx, stderr, os.Stderr, false)

	go func() {
		log.Trace("waiting for meteor command to exit")
		a.meteorExit <- cmd.Wait()
	}()

	return nil
}

func (a *App) streamOutput(ctx context.Context, r io.Reader, w io.Writer, detectReady bool) {
	reader := bufio.NewReader(r)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			log.WithField("line", strings.TrimSpace(line)).Trace("meteor output line")
			if _, writeErr := w.Write([]byte(line)); writeErr != nil {
				log.WithError(writeErr).Debug("failed to write meteor output")
			}
			if detectReady && a.containsReadyMarker(line) {
				log.Debug("ready marker identified in meteor output; starting browser")
				a.startChromeOnce.Do(func() {
					go a.startBrowser(ctx)
				})
			}
		}

		if err != nil {
			if err != io.EOF {
				log.WithError(err).Debug("meteor output read error")
			} else {
				log.Trace("meteor output reached EOF")
			}
			return
		}
	}
}
