package config

import (
	"errors"
	"os"

	"github.com/spf13/pflag"

	"github.com/illusionfield/mtest/internal/logging"
)

const defaultVerbosity = 3

var (
	// ErrMissingPackageName indicates that the mandatory --package flag was omitted.
	ErrMissingPackageName = errors.New("missing package name; supply --package/-p")
	// ErrHelpRequested mirrors pflag.ErrHelp so callers can handle --help uniformly.
	ErrHelpRequested = errors.New("help requested")
)

// Config captures the command-line options that control the test runner.
// The structure is exported so other packages can introspect configuration values.
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

// Result aggregates the parsed configuration together with metadata about CLI actions.
type Result struct {
	Config      Config
	ShowVersion bool
}

// Parse interprets command-line arguments and returns the resulting configuration.
// It honors the go.dev/doc/comment guidelines, mirroring the original behavior while
// surfacing structured errors for callers.
func Parse(args []string) (Result, error) {
	var (
		cfg         Config
		showVersion bool
	)

	flagSet := pflag.NewFlagSet("mtest", pflag.ContinueOnError)
	flagSet.SortFlags = false
	flagSet.SetOutput(os.Stdout)

	flagSet.StringVarP(&cfg.PackageName, "package", "p", "", "Meteor package name to test (required)")
	flagSet.StringVarP(&cfg.Release, "release", "r", "", "Meteor release to use")
	flagSet.StringVarP(&cfg.SettingsPath, "settings", "s", "", "Settings JSON file path")
	flagSet.StringVarP(&cfg.TestAppPath, "test-app-path", "t", "", "Test app path")
	flagSet.BoolVarP(&cfg.Once, "once", "o", false, "Exit after the first test run finishes")
	flagSet.BoolVarP(&cfg.Inspect, "inspect", "i", false, "Pass --inspect to meteor")
	flagSet.BoolVarP(&cfg.InspectBrk, "inspect-brk", "b", false, "Pass --inspect-brk to meteor")
	flagSet.IntVar(&cfg.Port, "port", 0, "Port to use for the test app (defaults to random free port between 10000-11999)")
	flagSet.VarP(logging.NewVerbosityValue(&cfg.Verbosity, defaultVerbosity), "verbose", "v", logging.VerbosityUsage)
	if f := flagSet.Lookup("verbose"); f != nil {
		f.NoOptDefVal = "+"
	}
	flagSet.BoolVarP(&showVersion, "version", "V", false, "Print version and exit")

	if err := flagSet.Parse(args); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return Result{}, ErrHelpRequested
		}
		return Result{}, err
	}

	if showVersion {
		return Result{Config: cfg, ShowVersion: true}, nil
	}

	if cfg.PackageName == "" {
		return Result{}, ErrMissingPackageName
	}

	return Result{Config: cfg}, nil
}
