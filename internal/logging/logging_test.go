package logging

import (
	"io"
	"testing"

	log "github.com/sirupsen/logrus"
)

func TestConfigureAdjustsLevel(t *testing.T) {
	logger := log.StandardLogger()

	originalLevel := logger.GetLevel()
	originalOut := logger.Out
	originalFormatter := logger.Formatter
	originalReportCaller := logger.ReportCaller

	t.Cleanup(func() {
		logger.SetLevel(originalLevel)
		logger.SetOutput(originalOut)
		logger.SetFormatter(originalFormatter)
		logger.SetReportCaller(originalReportCaller)
	})

	Configure(0)
	if logger.GetLevel() != log.PanicLevel {
		t.Fatalf("verbosity 0 expected level PanicLevel, got %v", logger.GetLevel())
	}
	if logger.Out != io.Discard {
		t.Fatalf("verbosity 0 expected output io.Discard")
	}

	Configure(2)
	if logger.GetLevel() != log.WarnLevel {
		t.Fatalf("verbosity 2 expected level WarnLevel, got %v", logger.GetLevel())
	}

	Configure(5)
	if logger.GetLevel() != log.TraceLevel {
		t.Fatalf("verbosity 5 expected level TraceLevel, got %v", logger.GetLevel())
	}
	if !logger.ReportCaller {
		t.Fatalf("verbosity 5 expected ReportCaller to be enabled")
	}
}

func TestVerbosityValue(t *testing.T) {
	var level int
	v := NewVerbosityValue(&level, 3)
	if level != 3 {
		t.Fatalf("default level = %d, want 3", level)
	}

	if err := v.Set("+"); err != nil {
		t.Fatalf("increment error: %v", err)
	}
	if level != 4 {
		t.Fatalf("after + expected 4, got %d", level)
	}

	if err := v.Set("+"); err != nil {
		t.Fatalf("second increment error: %v", err)
	}
	if level != 5 {
		t.Fatalf("after second + expected 5, got %d", level)
	}

	if err := v.Set("+"); err != nil {
		t.Fatalf("third increment error: %v", err)
	}
	if level != 5 {
		t.Fatalf("increment should cap at 5, got %d", level)
	}

	if err := v.Set("1"); err != nil {
		t.Fatalf("numeric set error: %v", err)
	}
	if level != 1 {
		t.Fatalf("after numeric set expected 1, got %d", level)
	}

	if err := v.Set("trace"); err != nil {
		t.Fatalf("named set error: %v", err)
	}
	if level != 5 {
		t.Fatalf("trace should map to 5, got %d", level)
	}

	if err := v.Set("unknown"); err != nil {
		t.Fatalf("unknown set error: %v", err)
	}
	if level != 5 {
		t.Fatalf("unknown value should keep level 5, got %d", level)
	}

	if got := v.String(); got != "5" {
		t.Fatalf("String() = %q, want 5", got)
	}
	if got := v.Type(); got != "verbosity" {
		t.Fatalf("Type() = %q, want verbosity", got)
	}
}
