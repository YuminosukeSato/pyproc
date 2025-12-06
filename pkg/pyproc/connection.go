package pyproc

import (
	"context"
	"fmt"
	"net"
	"time"
)

const defaultSleepDuration = 100 * time.Millisecond

// ConnectToWorker connects to a worker via Unix domain socket
func ConnectToWorker(socketPath string, timeout time.Duration) (net.Conn, error) {
	// Set connection timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	for {
		conn, err := net.Dial("unix", socketPath)
		if err == nil {
			return conn, nil
		}

		// Sleep before retry, or return if timeout occurs during sleep
		if err := sleepWithCtx(ctx, defaultSleepDuration); err != nil {
			return nil, fmt.Errorf("failed to connect to worker at %s after %v", socketPath, timeout)
		}
	}
}

func sleepWithCtx(ctx context.Context, d time.Duration) error {
	// Wait a bit before retrying
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
