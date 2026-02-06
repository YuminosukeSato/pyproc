package pyproc

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func tempSocketPath(t *testing.T, prefix string) string {
	t.Helper()
	base := filepath.Join(os.TempDir(), "pyproc")
	if err := os.MkdirAll(base, 0755); err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	name := fmt.Sprintf("%s-%d.sock", prefix, time.Now().UnixNano())
	return filepath.Join(base, name)
}
