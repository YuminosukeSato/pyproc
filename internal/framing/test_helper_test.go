package framing_test

import (
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func requireUnixSocket(t *testing.T) {
	t.Helper()
	dir := filepath.Join(os.TempDir(), "pyproc-framing-probe")
	_ = os.MkdirAll(dir, 0o755)
	path := filepath.Join(dir, "probe.sock")
	l, err := net.Listen("unix", path)
	if err != nil {
		t.Skipf("unix domain sockets not permitted: %v", err)
		return
	}
	_ = l.Close()
	_ = os.Remove(path)
	python := exec.Command("python3", "-c", "import socket,sys;s=socket.socket(socket.AF_UNIX);s.bind(sys.argv[1]);s.close()", path)
	if err := python.Run(); err != nil {
		t.Skipf("python unix socket bind not permitted: %v", err)
	}
	_ = os.Remove(path)
}
