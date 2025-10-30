package app

import "time"

const (
	defaultPortMin     = 10000
	defaultPortMax     = 12000
	testStatusInterval = 500 * time.Millisecond
	consoleSentinel    = "##_meteor_magic##state: done"
)

var readyMarkers = []string{"10015", "test-in-console listening"}
