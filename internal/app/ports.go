package app

import (
	"errors"
	"fmt"
	"math/rand"
	"net"
	"time"

	log "github.com/sirupsen/logrus"
)

func (a *App) resolvePort() (int, error) {
	if a.cfg.Port > 0 {
		log.WithField("port", a.cfg.Port).Debug("using CLI-specified port")
		return a.cfg.Port, nil
	}

	log.Debug("no port specified; attempting dynamic discovery")

	candidateCount := defaultPortMax - defaultPortMin
	ports := make([]int, 0, candidateCount)
	for p := defaultPortMin; p < defaultPortMax; p++ {
		ports = append(ports, p)
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	rng.Shuffle(len(ports), func(i, j int) { ports[i], ports[j] = ports[j], ports[i] })

	for _, port := range ports {
		log.WithField("port", port).Trace("probing port availability")
		listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err == nil {
			_ = listener.Close()
			log.WithField("port", port).Debug("discovered available port")
			return port, nil
		}
	}
	log.Debug("exhausted port range without finding a free port")
	return 0, errors.New("no available port found in range 10000-11999")
}
