package pyproc

import (
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

var unixSocketsAvailable bool

func TestMain(m *testing.M) {
	unixSocketsAvailable = probeUnixSocketSupport()
	code := m.Run()
	os.Exit(code)
}

func probeUnixSocketSupport() bool {
	dir := filepath.Join(os.TempDir(), "pyproc-probe")
	_ = os.MkdirAll(dir, 0o755)
	path := filepath.Join(dir, "probe.sock")
	listener, err := net.Listen("unix", path)
	if err != nil {
		return false
	}
	_ = listener.Close()
	_ = os.Remove(path)
	python := exec.Command("python3", "-c", "import socket,sys;s=socket.socket(socket.AF_UNIX);s.bind(sys.argv[1]);s.close()", path)
	if err := python.Run(); err != nil {
		_ = os.Remove(path)
		return false
	}
	_ = os.Remove(path)
	return true
}

func requireUnixSocket(t *testing.T) {
	t.Helper()
	if !unixSocketsAvailable {
		t.Skip("unix domain sockets not permitted in this environment")
	}
}
