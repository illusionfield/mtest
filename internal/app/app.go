package app

import (
	"context"
	"errors"
	"os/exec"
	"sync"

	"github.com/go-rod/rod"
	log "github.com/sirupsen/logrus"

	"github.com/illusionfield/mtest/internal/config"
	"github.com/illusionfield/mtest/internal/process"
)

// App orchestrates the lifecycle of the Meteor package test harness.
type App struct {
	cfg         config.Config
	port        int
	meteorCmd   *exec.Cmd
	testsResult chan int
	meteorExit  chan error

	startChromeOnce sync.Once
	shutdownOnce    sync.Once
	consoleMu       sync.Mutex
	browser         *rod.Browser
	page            *rod.Page
}

// New constructs a ready-to-run App for the provided configuration.
func New(cfg config.Config) *App {
	return &App{
		cfg:         cfg,
		testsResult: make(chan int, 1),
		meteorExit:  make(chan error, 1),
	}
}

// Run executes the configured test workflow and returns the process exit code.
func (a *App) Run(ctx context.Context) int {
	log.Debug("starting app.Run execution")

	log.Debug("resolving port for Meteor test runner")
	port, err := a.resolvePort()
	if err != nil {
		log.WithError(err).Error("failed to determine a free port")
		return 1
	}
	a.port = port
	log.WithField("port", a.port).Debug("port reservation complete")

	meteorCtx, meteorCancel := context.WithCancel(context.Background())
	defer meteorCancel()

	log.Debug("launching Meteor process")
	if err := a.startMeteor(meteorCtx); err != nil {
		log.WithError(err).Error("failed to start meteor")
		return 1
	}
	log.WithField("port", a.port).Info("Meteor process started")

	for {
		select {
		case <-ctx.Done():
			log.Debug("context cancelled, initiating shutdown")
			log.Info("signal received, shutting down")
			return 0
		case err := <-a.meteorExit:
			log.Trace("received meteor exit notification")
			if err != nil {
				var exitErr *exec.ExitError
				if errors.As(err, &exitErr) {
					if exitErr.ExitCode() != 0 {
						log.WithField("code", exitErr.ExitCode()).Error("Meteor process exited unexpectedly")
					}
				} else {
					log.WithError(err).Error("Meteor process exited")
				}
				return 1
			}
			log.Info("Meteor process exited cleanly")
			return 0
		case code := <-a.testsResult:
			log.WithField("status", code).Trace("testsResult received from browser monitoring")
			log.WithField("status", code).Info("tests finished")
			if a.cfg.Once {
				return code
			}
			log.Debug("test run completed in watch mode; awaiting next cycle")
		}
	}
}

// Shutdown releases external resources such as the Meteor process and headless browser.
func (a *App) Shutdown() {
	a.shutdownOnce.Do(func() {
		log.Debug("shutting down application resources")
		if a.meteorCmd != nil {
			if err := process.Terminate(a.meteorCmd); err != nil {
				log.WithError(err).Debug("meteor termination error")
			}
			a.meteorCmd = nil
		}

		a.consoleMu.Lock()
		defer a.consoleMu.Unlock()
		if a.page != nil {
			log.Trace("closing browser page")
			if err := a.page.Close(); err != nil {
				log.WithError(err).Debug("browser page close failed")
			}
			a.page = nil
		}
		if a.browser != nil {
			log.Trace("closing browser instance")
			if err := a.browser.Close(); err != nil {
				log.WithError(err).Debug("browser close failed")
			}
			a.browser = nil
		}
	})
}
