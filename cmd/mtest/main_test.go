package main

import "testing"

func TestVersionStringDefaults(t *testing.T) {
	prevVersion := Version
	prevCommit := Commit
	t.Cleanup(func() {
		Version = prevVersion
		Commit = prevCommit
	})

	Version = ""
	Commit = ""
	got := versionString()
	want := "mtest dev (commit local)"
	if got != want {
		t.Fatalf("versionString() = %q, want %q", got, want)
	}
}

func TestVersionStringValues(t *testing.T) {
	prevVersion := Version
	prevCommit := Commit
	t.Cleanup(func() {
		Version = prevVersion
		Commit = prevCommit
	})

	Version = "1.2.3"
	Commit = "abcdef0"
	got := versionString()
	want := "mtest 1.2.3 (commit abcdef0)"
	if got != want {
		t.Fatalf("versionString() = %q, want %q", got, want)
	}
}
