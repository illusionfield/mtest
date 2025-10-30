package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	log "github.com/sirupsen/logrus"
)

func (a *App) startBrowser(ctx context.Context) {
	log.Debug("starting headless browser for test monitoring")

	launch := launcher.New().
		Headless(true).
		NoSandbox(true).
		Set("disable-setuid-sandbox").
		Set("disable-dev-shm-usage")

	controlURL, err := launch.Launch()
	if err != nil {
		log.WithError(err).Error("failed to launch browser")
		return
	}
	log.WithField("control_url", controlURL).Debug("browser launcher initialised")

	browser := rod.New().ControlURL(controlURL).NoDefaultDevice()
	if err := browser.Connect(); err != nil {
		log.WithError(err).Error("failed to connect to browser")
		return
	}
	log.Debug("connected to headless browser")

	page, err := browser.Page(proto.TargetCreateTarget{})
	if err != nil {
		log.WithError(err).Error("failed to open page")
		return
	}
	log.Debug("created browser page for TinyTest runner")

	a.consoleMu.Lock()
	a.browser = browser
	a.page = page
	a.consoleMu.Unlock()
	log.Trace("browser state stored on App")

	stopConsole := page.EachEvent(func(e *proto.RuntimeConsoleAPICalled) {
		msg := consoleMessage(page, e.Args)
		if msg == "" || msg == consoleSentinel {
			return
		}
		log.WithField("message", msg).Trace("browser console message")
		fmt.Println(msg)
	})
	defer stopConsole()

	url := fmt.Sprintf("http://localhost:%d", a.port)
	log.WithField("url", url).Debug("navigating to test runner URL")
	if err := a.waitForPage(ctx, page, url); err != nil {
		if !errors.Is(err, context.Canceled) {
			log.WithError(err).Error("failed to load test runner page")
		}
		return
	}
	log.Debug("test runner page ready")

	fmt.Println("Running tests...")
	log.Trace("signalled test start to console")

	code, err := a.waitForTests(ctx, page)
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			log.WithError(err).Error("failed while waiting for tests")
		}
		return
	}

	select {
	case a.testsResult <- code:
		log.WithField("status", code).Trace("dispatched test result to channel")
	default:
		log.Trace("testsResult channel already populated; skipping dispatch")
	}

	if !a.cfg.Once && code != 0 {
		log.WithField("failures", code).Warn("tests finished with failures")
	}
}

func (a *App) waitForPage(ctx context.Context, page *rod.Page, url string) error {
	var lastErr error
	for attempt := 0; attempt < 10; attempt++ {
		select {
		case <-ctx.Done():
			log.Trace("waitForPage aborted because context was cancelled")
			return ctx.Err()
		default:
		}
		log.WithField("attempt", attempt+1).Trace("navigating to test runner page")
		err := page.Navigate(url)
		if err == nil {
			if err = page.WaitLoad(); err == nil {
				log.Debug("test runner page load complete")
				return nil
			}
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = errors.New("unknown navigation error")
	}
	return fmt.Errorf("unable to load %s: %w", url, lastErr)
}
