package agentssh

import (
	"strconv"
	"strings"
	"time"

	"github.com/gliderlabs/ssh"
	gossh "golang.org/x/crypto/ssh"

	"cdr.dev/slog/v3"
)

const (
	// ClientAliveIntervalEnvironmentVariable and
	// ClientAliveCountMaxEnvironmentVariable let a client that reconnects on
	// its own ask the agent to close its connection after CountMax unanswered
	// keep-alives, one every Interval seconds, like OpenSSH's ClientAlive*.
	ClientAliveIntervalEnvironmentVariable = "CODER_SSH_CLIENT_ALIVE_INTERVAL"
	ClientAliveCountMaxEnvironmentVariable = "CODER_SSH_CLIENT_ALIVE_COUNT_MAX"

	minClientAliveInterval = 5 * time.Second
	clientAliveRequestType = "keepalive@openssh.com"
)

// extractClientAlive strips both variables from env. It returns a zero
// interval unless both are positive integers.
func extractClientAlive(env []string) (interval time.Duration, countMax int, filteredEnv []string) {
	var rawInterval, rawCountMax string
	for _, kv := range env {
		if v, ok := strings.CutPrefix(kv, ClientAliveIntervalEnvironmentVariable+"="); ok {
			rawInterval = v
		} else if v, ok := strings.CutPrefix(kv, ClientAliveCountMaxEnvironmentVariable+"="); ok {
			rawCountMax = v
		} else {
			filteredEnv = append(filteredEnv, kv)
		}
	}
	// 16 bits keeps the duration from overflowing.
	seconds, err := strconv.ParseUint(rawInterval, 10, 16)
	if err != nil || seconds == 0 {
		return 0, 0, filteredEnv
	}
	count, err := strconv.ParseUint(rawCountMax, 10, 16)
	if err != nil || count == 0 {
		return 0, 0, filteredEnv
	}
	return max(time.Duration(seconds)*time.Second, minClientAliveInterval), int(count), filteredEnv
}

// closeOnMissedClientAlive closes the connection once countMax intervals pass
// without a keep-alive reply. It runs at most once per connection.
func (s *Server) closeOnMissedClientAlive(ctx ssh.Context, logger slog.Logger, interval time.Duration, countMax int) {
	conn, ok := ctx.Value(ssh.ContextKeyConn).(gossh.Conn)
	if !ok {
		return
	}
	if _, running := s.clientAliveConns.LoadOrStore(conn, struct{}{}); running {
		return
	}

	replies := make(chan struct{})
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			// Blocks until the client replies or the connection closes.
			if _, _, err := conn.SendRequest(clientAliveRequestType, true, nil); err != nil {
				return
			}
			select {
			case replies <- struct{}{}:
			case <-ctx.Done():
				return
			}
			select {
			case <-ticker.C:
			case <-ctx.Done():
				return
			}
		}
	}()

	go func() {
		defer s.clientAliveConns.Delete(conn)
		timeout := interval * time.Duration(countMax)
		deadline := time.NewTimer(timeout)
		defer deadline.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-replies:
				deadline.Reset(timeout)
			case <-deadline.C:
				logger.Info(ctx, "closing ssh connection after missed client keep-alives",
					slog.F("interval", interval), slog.F("count_max", countMax))
				_ = conn.Close()
				return
			}
		}
	}()
}
