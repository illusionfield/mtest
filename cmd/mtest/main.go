package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	log "github.com/sirupsen/logrus"

	"github.com/illusionfield/mtest/internal/app"
	"github.com/illusionfield/mtest/internal/config"
	"github.com/illusionfield/mtest/internal/logging"
)

var (
	Version string
	Commit  string
)

func main() {
	parsed, err := config.Parse(os.Args[1:])
	if err != nil {
		switch {
		case errors.Is(err, config.ErrHelpRequested):
			return
		case errors.Is(err, config.ErrMissingPackageName):
			fmt.Fprintln(os.Stderr, "error:", err)
		default:
			fmt.Fprintln(os.Stderr, "error:", err)
		}
		os.Exit(2)
	}

	logging.Configure(parsed.Config.Verbosity)
	log.WithField("verbosity", parsed.Config.Verbosity).Debug("logging configured")

	log.WithFields(log.Fields{
		"package":       parsed.Config.PackageName,
		"release":       parsed.Config.Release,
		"settings_path": parsed.Config.SettingsPath,
		"test_app":      parsed.Config.TestAppPath,
		"once":          parsed.Config.Once,
		"inspect":       parsed.Config.Inspect,
		"inspect_brk":   parsed.Config.InspectBrk,
		"port":          parsed.Config.Port,
	}).Trace("effective configuration parsed from CLI")

	if parsed.ShowVersion {
		log.Trace("version flag detected; printing version and exiting")
		fmt.Println(versionString())
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	log.Trace("registered signal handlers for SIGINT/SIGTERM")

	application := app.New(parsed.Config)
	log.Debug("constructed application instance")
	exitCode := application.Run(ctx)
	log.WithField("exit_code", exitCode).Debug("application run complete")
	application.Shutdown()
	os.Exit(exitCode)
}

func versionString() string {
	version := Version
	if version == "" {
		version = "dev"
	}

	commit := Commit
	if commit == "" {
		commit = "local"
	}

	return fmt.Sprintf("mtest %s (commit %s)", version, commit)
}
