package app

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	log "github.com/sirupsen/logrus"
)

func (a *App) waitForTests(ctx context.Context, page *rod.Page) (int, error) {
	ticker := time.NewTicker(testStatusInterval)
	defer ticker.Stop()
	log.Trace("entered waitForTests loop")

	for {
		select {
		case <-ctx.Done():
			log.Trace("waitForTests exiting due to context cancellation")
			return 0, ctx.Err()
		case <-ticker.C:
			log.Trace("polling TinyTest status")
			done, err := testsDone(page)
			if err != nil {
				log.WithError(err).Debug("checking test completion failed")
				continue
			}
			if done {
				log.Debug("detected that tests completed; fetching failure count")
				failures, err := fetchFailures(page)
				if err != nil {
					return 1, err
				}
				log.WithField("failures", failures).Debug("retrieved TinyTest failure count")
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
		log.Trace("testsDone evaluation returned nil result")
		return false, nil
	}
	done := res.Value.Bool()
	log.WithField("done", done).Trace("testsDone evaluation result")
	return done, nil
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
		log.Trace("fetchFailures evaluation returned nil result")
		return 0, nil
	}
	if res.Type == proto.RuntimeRemoteObjectTypeString {
		str := strings.TrimSpace(res.Value.Str())
		if str == "" {
			log.Trace("fetchFailures received empty string result")
			return 0, nil
		}
		if n, err := strconv.Atoi(str); err == nil {
			log.WithField("failures", n).Trace("fetchFailures parsed integer string")
			return n, nil
		}
	}
	failures := res.Value.Int()
	log.WithField("failures", failures).Trace("fetchFailures returning numeric value")
	return failures, nil
}
