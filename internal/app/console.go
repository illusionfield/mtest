package app

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	log "github.com/sirupsen/logrus"
)

func (a *App) containsReadyMarker(line string) bool {
	for _, marker := range readyMarkers {
		if strings.Contains(line, marker) {
			log.WithField("marker", marker).Trace("ready marker matched in meteor output")
			return true
		}
	}
	return false
}

func consoleMessage(page *rod.Page, args []*proto.RuntimeRemoteObject) string {
	if len(args) == 0 {
		return ""
	}

	parts := make([]string, 0, len(args))
	for _, arg := range args {
		jsonVal, err := page.ObjectToJSON(arg)
		if err != nil {
			log.WithError(err).Debug("console value decode failed")
			continue
		}
		switch v := jsonVal.Val().(type) {
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

	message := strings.TrimSpace(strings.Join(parts, " "))
	if message != "" {
		log.WithField("message", message).Trace("formatted console message")
	}
	return message
}
