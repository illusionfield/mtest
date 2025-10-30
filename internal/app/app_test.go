package app

import (
	"fmt"
	"net"
	"testing"

	"github.com/illusionfield/mtest/internal/config"
)

func TestResolvePortUsesConfiguredValue(t *testing.T) {
	cfg := config.Config{Port: 10555}
	application := New(cfg)

	port, err := application.resolvePort()
	if err != nil {
		t.Fatalf("resolvePort returned error: %v", err)
	}
	if port != cfg.Port {
		t.Fatalf("expected port %d, got %d", cfg.Port, port)
	}
}

func TestResolvePortDiscoversFreePort(t *testing.T) {
	cfg := config.Config{Port: 0}
	application := New(cfg)

	port, err := application.resolvePort()
	if err != nil {
		t.Fatalf("resolvePort returned error: %v", err)
	}
	if port < defaultPortMin || port >= defaultPortMax {
		t.Fatalf("port %d outside expected range [%d,%d)", port, defaultPortMin, defaultPortMax)
	}

	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		t.Fatalf("expected port %d to be available: %v", port, err)
	}
	_ = listener.Close()
}

func TestContainsReadyMarker(t *testing.T) {
	application := New(config.Config{})

	line := "Meteor ready on 10015"
	if !application.containsReadyMarker(line) {
		t.Fatalf("expected containsReadyMarker to detect marker in %q", line)
	}

	nonMatch := "starting up"
	if application.containsReadyMarker(nonMatch) {
		t.Fatalf("expected containsReadyMarker to return false for %q", nonMatch)
	}
}
