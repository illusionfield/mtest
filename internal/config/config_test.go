package config

import "testing"

func TestParseRequiresPackage(t *testing.T) {
	_, err := Parse(nil)
	if err == nil || err != ErrMissingPackageName {
		t.Fatalf("expected ErrMissingPackageName, got %v", err)
	}
}

func TestParseDefaults(t *testing.T) {
	args := []string{"--package", "dummy"}
	result, err := Parse(args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.ShowVersion {
		t.Fatalf("expected ShowVersion to be false")
	}

	cfg := result.Config
	if cfg.PackageName != "dummy" {
		t.Errorf("PackageName = %q, want %q", cfg.PackageName, "dummy")
	}
	if cfg.Verbosity != defaultVerbosity {
		t.Errorf("Verbosity = %d, want %d", cfg.Verbosity, defaultVerbosity)
	}
}

func TestParseVersionFlag(t *testing.T) {
	args := []string{"--package", "dummy", "--version"}
	result, err := Parse(args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.ShowVersion {
		t.Fatalf("expected ShowVersion to be true")
	}
}

func TestParseVerbosityIncrement(t *testing.T) {
	args := []string{"--package", "dummy", "-v"}
	result, err := Parse(args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Config.Verbosity != 4 {
		t.Fatalf("Verbosity = %d, want %d", result.Config.Verbosity, 4)
	}
}

func TestParseVerbositySaturatesAtTrace(t *testing.T) {
	args := []string{"--package", "dummy", "-v", "-v", "-v"}
	result, err := Parse(args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Config.Verbosity != 5 {
		t.Fatalf("Verbosity = %d, want %d", result.Config.Verbosity, 5)
	}
}

func TestParseVerbosityExplicitLevel(t *testing.T) {
	args := []string{"--package", "dummy", "--verbose=1"}
	result, err := Parse(args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Config.Verbosity != 1 {
		t.Fatalf("Verbosity = %d, want %d", result.Config.Verbosity, 1)
	}
}
