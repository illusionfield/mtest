package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
)

var (
	Version string
	Commit  string
)

const (
	defaultPortMin     = 10000
	defaultPortMax     = 12000
	testStatusInterval = 500 * time.Millisecond
	consoleSentinel    = "##_meteor_magic##state: done"
)

var readyMarkers = []string{"10015", "test-in-console listening"}

type Config struct {
	PackageName  string
	Release      string
	SettingsPath string
	TestAppPath  string
	Once         bool
	Inspect      bool
	InspectBrk   bool
	Port         int
	Verbosity    int
}

type App struct {
	cfg         Config
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

func main() {
	var (
		flagPackage    string
		flagRelease    string
		flagSettings   string
		flagTestApp    string
		flagOnce       bool
		flagInspect    bool
		flagInspectBrk bool
		flagPort       int
		flagVerbose    int
		flagVersion    bool
	)

	pflag.StringVarP(&flagPackage, "package", "p", "", "Meteor package name to test (required)")
	pflag.StringVarP(&flagRelease, "release", "r", "", "Meteor release to use")
	pflag.StringVarP(&flagSettings, "settings", "s", "", "Settings JSON file path")
	pflag.StringVarP(&flagTestApp, "test-app-path", "t", "", "Test app path")
	pflag.BoolVarP(&flagOnce, "once", "o", false, "Exit after the first test run finishes")
	pflag.BoolVarP(&flagInspect, "inspect", "i", false, "Pass --inspect to meteor")
	pflag.BoolVarP(&flagInspectBrk, "inspect-brk", "b", false, "Pass --inspect-brk to meteor")
	pflag.IntVar(&flagPort, "port", 0, "Port to use for the test app (defaults to random free port between 10000-11999)")
	pflag.CountVarP(&flagVerbose, "verbose", "v", "Verbosity level: 0 error, 1 warn, 2 info, 3 debug")
	pflag.BoolVarP(&flagVersion, "version", "V", false, "Print version and exit")
	pflag.Parse()

	if !pflag.CommandLine.Changed("verbose") {
		flagVerbose = 2
	}
	setupLogging(flagVerbose)

	if flagVersion {
		printVersion()
		return
	}

	if flagPackage == "" {
		log.Warn("no package name provided (--package is required)")
		os.Exit(1)
	}

	cfg := Config{
		PackageName:  flagPackage,
		Release:      flagRelease,
		SettingsPath: flagSettings,
		TestAppPath:  flagTestApp,
		Once:         flagOnce,
		Inspect:      flagInspect,
		InspectBrk:   flagInspectBrk,
		Port:         flagPort,
		Verbosity:    flagVerbose,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	app := &App{
		cfg:         cfg,
		testsResult: make(chan int, 1),
		meteorExit:  make(chan error, 1),
	}

	exitCode := app.Run(ctx)
	app.shutdown()
	os.Exit(exitCode)
}

// ============================================================================
// CLI + Logging setup (pflag + logrus)
// ============================================================================
func setupLogging(verbosity int) {
	// 0 none, 1 error, 2 warn, 3 info, 4 debug, 5 trace
	switch {
	case verbosity <= 0:
		log.SetOutput(io.Discard)
		log.SetLevel(log.PanicLevel)
	case verbosity == 1:
		log.SetLevel(log.ErrorLevel)
	case verbosity == 2:
		log.SetLevel(log.WarnLevel)
	case verbosity == 3:
		log.SetLevel(log.InfoLevel)
	case verbosity >= 5:
		log.SetLevel(log.TraceLevel)
	default:
		log.SetLevel(log.DebugLevel)
	}
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
	})
}

func printVersion() {
	ver := Version
	if ver == "" {
		ver = "dev"
	}
	sha := Commit
	if sha == "" {
		sha = "local"
	}
	fmt.Printf("mtest %s (commit %s)\n", ver, sha)
}

func (a *App) Run(ctx context.Context) int {
	port, err := a.resolvePort()
	if err != nil {
		log.Errorf("failed to determine port: %v", err)
		return 1
	}
	a.port = port

	meteorCtx, meteorCancel := context.WithCancel(context.Background())
	defer meteorCancel()

	if err := a.startMeteor(meteorCtx); err != nil {
		log.Errorf("failed to start meteor: %v", err)
		return 1
	}
	log.Infof("Meteor process started on port %d", a.port)

	for {
		select {
		case <-ctx.Done():
			log.Info("Signal received, shutting down...")
			return 0
		case err := <-a.meteorExit:
			if err != nil {
				if exitErr := new(exec.ExitError); errors.As(err, &exitErr) {
					if exitErr.ExitCode() != 0 {
						log.Errorf("Meteor process exited with code %d", exitErr.ExitCode())
					}
				} else {
					log.Errorf("Meteor process exited: %v", err)
				}
				return 1
			}
			log.Info("Meteor process exited cleanly")
			return 0
		case code := <-a.testsResult:
			log.Infof("Tests finished with status %d", code)
			if a.cfg.Once {
				return code
			}
		}
	}
}

func (a *App) resolvePort() (int, error) {
	if a.cfg.Port > 0 {
		return a.cfg.Port, nil
	}
	ports := make([]int, 0, defaultPortMax-defaultPortMin)
	for p := defaultPortMin; p < defaultPortMax; p++ {
		ports = append(ports, p)
	}
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(ports), func(i, j int) { ports[i], ports[j] = ports[j], ports[i] })
	for _, p := range ports {
		l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", p))
		if err == nil {
			_ = l.Close()
			return p, nil
		}
	}
	return 0, errors.New("no available port found in range 10000-11999")
}

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

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		comspec := os.Getenv("COMSPEC")
		if comspec == "" {
			comspec = "cmd.exe"
		}
		allArgs := append([]string{"/c", "meteor"}, args...)
		cmd = exec.CommandContext(ctx, comspec, allArgs...)
	} else {
		cmd = exec.CommandContext(ctx, "meteor", args...)
	}
	setupProcessGroup(cmd)
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

	a.meteorCmd = cmd

	go a.streamOutput(ctx, stdout, os.Stdout, true)
	go a.streamOutput(ctx, stderr, os.Stderr, false)

	go func() {
		err := cmd.Wait()
		a.meteorExit <- err
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
			if _, writeErr := w.Write([]byte(line)); writeErr != nil {
				log.Debugf("failed to write meteor output: %v", writeErr)
			}
			if detectReady && a.containsReadyMarker(line) {
				a.startChromeOnce.Do(func() {
					go a.startBrowser(ctx)
				})
			}
		}
		if err != nil {
			if !errors.Is(err, io.EOF) {
				log.Debugf("meteor output read error: %v", err)
			}
			return
		}
	}
}

func (a *App) containsReadyMarker(line string) bool {
	for _, marker := range readyMarkers {
		if strings.Contains(line, marker) {
			return true
		}
	}
	return false
}

func (a *App) startBrowser(ctx context.Context) {
	log.Debug("Starting headless browser for test monitoring")
	launch := launcher.New().
		Headless(true).
		NoSandbox(true).
		Set("disable-setuid-sandbox").
		Set("disable-dev-shm-usage")

	controlURL, err := launch.Launch()
	if err != nil {
		log.Errorf("failed to launch browser: %v", err)
		return
	}

	browser := rod.New().ControlURL(controlURL).NoDefaultDevice()
	if err := browser.Connect(); err != nil {
		log.Errorf("failed to connect to browser: %v", err)
		return
	}

	page, err := browser.Page(proto.TargetCreateTarget{})
	if err != nil {
		log.Errorf("failed to open page: %v", err)
		return
	}

	a.consoleMu.Lock()
	a.browser = browser
	a.page = page
	a.consoleMu.Unlock()

	stopConsole := page.EachEvent(func(e *proto.RuntimeConsoleAPICalled) {
		msg := consoleMessage(page, e.Args)
		if msg == "" || msg == consoleSentinel {
			return
		}
		fmt.Println(msg)
	})
	defer stopConsole()

	url := fmt.Sprintf("http://localhost:%d", a.port)
	if err := a.waitForPage(ctx, page, url); err != nil {
		log.Errorf("failed to load test runner page: %v", err)
		return
	}

	fmt.Println("Running tests...")

	code, err := a.waitForTests(ctx, page)
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			log.Errorf("failed while waiting for tests: %v", err)
		}
		return
	}

	select {
	case a.testsResult <- code:
	default:
	}

	if !a.cfg.Once && code != 0 {
		log.Warnf("tests finished with %d failure(s)", code)
	}
}

func (a *App) waitForPage(ctx context.Context, page *rod.Page, url string) error {
	var lastErr error
	for attempt := 0; attempt < 10; attempt++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		err := page.Navigate(url)
		if err == nil {
			if err = page.WaitLoad(); err == nil {
				return nil
			}
		}
		lastErr = err
		time.Sleep(time.Second)
	}
	if lastErr == nil {
		lastErr = errors.New("unknown navigation error")
	}
	return fmt.Errorf("unable to load %s: %w", url, lastErr)
}

func (a *App) waitForTests(ctx context.Context, page *rod.Page) (int, error) {
	ticker := time.NewTicker(testStatusInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-ticker.C:
			done, err := testsDone(page)
			if err != nil {
				log.Debugf("check tests done failed: %v", err)
				continue
			}
			if done {
				failures, err := fetchFailures(page)
				if err != nil {
					return 1, err
				}
				return failures, nil
			}
		}
	}
}

func testsDone(page *rod.Page) (bool, error) {
	res, err := page.Eval(`() => {
		if (typeof Package !== 'undefined' && Package['test-in-console']) {
			return Package['test-in-console'].TEST_STATUS.DONE === true;
		}
		return false;
	}`)
	if err != nil {
		return false, err
	}
	if res == nil {
		return false, nil
	}
	return res.Value.Bool(), nil
}

func fetchFailures(page *rod.Page) (int, error) {
	res, err := page.Eval(`() => {
		if (typeof Package !== 'undefined' && Package['test-in-console']) {
			return Package['test-in-console'].TEST_STATUS.FAILURES || 0;
		}
		return 0;
	}`)
	if err != nil {
		return 0, err
	}
	if res == nil {
		return 0, nil
	}
	if res.Type == proto.RuntimeRemoteObjectTypeString {
		str := strings.TrimSpace(res.Value.Str())
		if str == "" {
			return 0, nil
		}
		if n, err := strconv.Atoi(str); err == nil {
			return n, nil
		}
	}
	return res.Value.Int(), nil
}

func consoleMessage(page *rod.Page, args []*proto.RuntimeRemoteObject) string {
	if len(args) == 0 {
		return ""
	}
	parts := make([]string, 0, len(args))
	for _, arg := range args {
		j, err := page.ObjectToJSON(arg)
		if err != nil {
			log.Debugf("console value decode failed: %v", err)
			continue
		}
		val := j.Val()
		switch v := val.(type) {
		case string:
			parts = append(parts, v)
		default:
			buf, err := json.Marshal(v)
			if err != nil {
				parts = append(parts, fmt.Sprint(v))
			} else {
				parts = append(parts, string(buf))
			}
		}
	}
	return strings.TrimSpace(strings.Join(parts, " "))
}

func (a *App) shutdown() {
	a.shutdownOnce.Do(func() {
		if a.meteorCmd != nil {
			if err := terminateProcess(a.meteorCmd); err != nil {
				log.Debugf("meteor termination error: %v", err)
			}
			a.meteorCmd = nil
		}

		a.consoleMu.Lock()
		defer a.consoleMu.Unlock()
		if a.page != nil {
			_ = a.page.Close()
			a.page = nil
		}
		if a.browser != nil {
			_ = a.browser.Close()
			a.browser = nil
		}
	})
}
