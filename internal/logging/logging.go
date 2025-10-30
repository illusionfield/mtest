package logging

import (
	"io"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
)

const (
	timestampFormat = "2006-01-02 15:04:05"
	// VerbosityUsage describes CLI expectations for the verbosity flag.
	VerbosityUsage = "Verbosity: default info (3). Use -v for debug, -vv for trace, or --verbose=0..5 (0 none, 1 error, 2 warn, 3 info, 4 debug, 5 trace)."
)

// Configure tunes the global logrus logger to match the verbosity requested on the CLI.
func Configure(verbosity int) {
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
		TimestampFormat: timestampFormat,
	})

	log.SetReportCaller(verbosity >= 5)
}

// VerbosityValue implements pflag.Value to support combined semantics for -v/-vv and --verbose=NUM.
type VerbosityValue struct {
	target *int
	limit  int
}

// NewVerbosityValue constructs a VerbosityValue linked to the provided target integer.
// The target is initialised to the supplied default level.
func NewVerbosityValue(target *int, def int) *VerbosityValue {
	if target != nil {
		*target = def
	}
	return &VerbosityValue{target: target, limit: 5}
}

// String serialises the current verbosity level for pflag.
func (v *VerbosityValue) String() string {
	if v == nil || v.target == nil {
		return ""
	}
	return strconv.Itoa(*v.target)
}

// Set interprets flag input, supporting increments, numeric levels, and name aliases.
func (v *VerbosityValue) Set(s string) error {
	if v == nil || v.target == nil {
		return nil
	}

	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}

	if s == "+" {
		if *v.target < v.limit {
			*v.target = *v.target + 1
		}
		return nil
	}

	if n, err := strconv.Atoi(s); err == nil {
		*v.target = n
		return nil
	}

	switch strings.ToLower(s) {
	case "none", "silent", "off":
		*v.target = 0
	case "error", "err":
		*v.target = 1
	case "warn", "warning":
		*v.target = 2
	case "info":
		*v.target = 3
	case "debug":
		*v.target = 4
	case "trace":
		*v.target = 5
	}

	return nil
}

// Type identifies the pflag.Value implementation.
func (v *VerbosityValue) Type() string { return "verbosity" }
