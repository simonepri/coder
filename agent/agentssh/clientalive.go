package agentssh

import (
	"strconv"
	"strings"
	"time"

	"github.com/gliderlabs/ssh"
	"go.uber.org/atomic"
	gossh "golang.org/x/crypto/ssh"

	"cdr.dev/slog/v3"
)

const (
	// ClientAliveIntervalEnvironmentVariable and
	// ClientAliveCountMaxEnvironmentVariable let a client that recovers from
	// dropped connections ask the agent to close its connection after
	// CountMax keep-alives go unanswered, like OpenSSH's ClientAliveInterval
	// and ClientAliveCountMax. The interval is in seconds. Both are stripped
	// from the session environment.
	ClientAliveIntervalEnvironmentVariable = "CODER_SSH_CLIENT_ALIVE_INTERVAL"
	ClientAliveCountMaxEnvironmentVariable = "CODER_SSH_CLIENT_ALIVE_COUNT_MAX"

	minClientAliveInterval = 5 * time.Second
	clientAliveRequestType = "keepalive@openssh.com"
)

// extractClientAlive returns zero values unless both variables are positive
// integers. The interval is raised to minClientAliveInterval.
func extractClientAlive(env []string) (interval time.Duration, countMax int, filteredEnv []string) {
	var rawInterval, rawCountMax string
	filteredEnv = make([]string, 0, len(env))
	for _, kv := range env {
		if v, ok := strings.CutPrefix(kv, ClientAliveIntervalEnvironmentVariable+"="); ok {
			rawInterval = v
		} else if v, ok := strings.CutPrefix(kv, ClientAliveCountMaxEnvironmentVariable+"="); ok {
			rawCountMax = v
		} else {
			filteredEnv = append(filteredEnv, kv)
		}
	}
	seconds, intervalErr := strconv.ParseUint(rawInterval, 10, 16)
	count, countErr := strconv.ParseUint(rawCountMax, 10, 16)
	if intervalErr != nil || countErr != nil || seconds == 0 || count == 0 {
		return 0, 0, filteredEnv
	}
	return max(time.Duration(seconds)*time.Second, minClientAliveInterval), int(count), filteredEnv
}

// closeOnMissedClientAlive starts at most one keep-alive loop per connection
// and closes the connection once countMax intervals pass without a reply.
func (s *Server) closeOnMissedClientAlive(ctx ssh.Context, logger slog.Logger, interval time.Duration, countMax int) {
	conn, ok := ctx.Value(ssh.ContextKeyConn).(gossh.Conn)
	if !ok {
		return
	}
	if _, running := s.clientAliveConns.LoadOrStore(conn, struct{}{}); running {
		return
	}
	go func() {
		defer s.clientAliveConns.Delete(conn)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		var lastReply atomic.Time
		lastReply.Store(time.Now())
		var inFlight atomic.Bool
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
			if time.Since(lastReply.Load()) >= interval*time.Duration(countMax) {
				logger.Info(ctx, "closing ssh connection after missed client keep-alives",
					slog.F("interval", interval), slog.F("count_max", countMax))
				_ = conn.Close()
				return
			}
			if inFlight.CompareAndSwap(false, true) {
				go func() {
					defer inFlight.Store(false)
					if _, _, err := conn.SendRequest(clientAliveRequestType, true, nil); err == nil {
						lastReply.Store(time.Now())
					}
				}()
			}
		}
	}()
}
