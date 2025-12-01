package pyproc

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewSocketManager(t *testing.T) {
	cfg := SocketConfig{
		Dir:         "/tmp/test-sockets",
		Prefix:      "pyproc",
		Permissions: 0o600,
	}

	sm := NewSocketManager(cfg)

	if sm.dir != cfg.Dir {
		t.Errorf("expected dir %s, got %s", cfg.Dir, sm.dir)
	}
	if sm.prefix != cfg.Prefix {
		t.Errorf("expected prefix %s, got %s", cfg.Prefix, sm.prefix)
	}
	if sm.permissions != os.FileMode(cfg.Permissions) {
		t.Errorf("expected permissions %o, got %o", cfg.Permissions, sm.permissions)
	}
}

func TestGenerateSocketPath(t *testing.T) {
	sm := &SocketManager{
		dir:    "/tmp/sockets",
		prefix: "test",
	}

	path := sm.GenerateSocketPath("worker-1")
	expected := "/tmp/sockets/test-worker-1.sock"
	if path != expected {
		t.Errorf("expected %s, got %s", expected, path)
	}
}

func TestCleanupSocket(t *testing.T) {
	tmpDir := t.TempDir()
	sm := &SocketManager{dir: tmpDir}

	t.Run("socket does not exist", func(t *testing.T) {
		nonExistentPath := filepath.Join(tmpDir, "nonexistent.sock")
		err := sm.CleanupSocket(nonExistentPath)
		if err != nil {
			t.Errorf("expected no error for non-existent socket, got %v", err)
		}
	})

	t.Run("socket exists and is removed", func(t *testing.T) {
		socketPath := filepath.Join(tmpDir, "test.sock")
		if err := os.WriteFile(socketPath, []byte{}, 0o600); err != nil {
			t.Fatalf("failed to create test socket: %v", err)
		}

		err := sm.CleanupSocket(socketPath)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}

		if _, err := os.Stat(socketPath); !os.IsNotExist(err) {
			t.Error("socket file should have been removed")
		}
	})
}

func TestCleanupAllSockets(t *testing.T) {
	tmpDir := t.TempDir()
	sm := &SocketManager{
		dir:    tmpDir,
		prefix: "pyproc",
	}

	sockets := []string{
		filepath.Join(tmpDir, "pyproc-worker1.sock"),
		filepath.Join(tmpDir, "pyproc-worker2.sock"),
		filepath.Join(tmpDir, "other-worker.sock"),
	}
	for _, s := range sockets {
		if err := os.WriteFile(s, []byte{}, 0o600); err != nil {
			t.Fatalf("failed to create socket %s: %v", s, err)
		}
	}

	err := sm.CleanupAllSockets()
	if err != nil {
		t.Errorf("CleanupAllSockets failed: %v", err)
	}

	if _, err := os.Stat(sockets[0]); !os.IsNotExist(err) {
		t.Error("pyproc-worker1.sock should have been removed")
	}
	if _, err := os.Stat(sockets[1]); !os.IsNotExist(err) {
		t.Error("pyproc-worker2.sock should have been removed")
	}
	if _, err := os.Stat(sockets[2]); os.IsNotExist(err) {
		t.Error("other-worker.sock should NOT have been removed")
	}
}

func TestEnsureSocketDir(t *testing.T) {
	tmpDir := t.TempDir()
	socketDir := filepath.Join(tmpDir, "nested", "sockets")
	sm := &SocketManager{dir: socketDir}

	err := sm.EnsureSocketDir()
	if err != nil {
		t.Fatalf("EnsureSocketDir failed: %v", err)
	}

	info, err := os.Stat(socketDir)
	if err != nil {
		t.Fatalf("failed to stat socket dir: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected socket dir to be a directory")
	}
}

func TestSocketManager_SetSocketPermissions(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "test.sock")
	if err := os.WriteFile(socketPath, []byte{}, 0o644); err != nil {
		t.Fatalf("failed to create test socket: %v", err)
	}

	sm := &SocketManager{permissions: 0o600}
	err := sm.SetSocketPermissions(socketPath)
	if err != nil {
		t.Fatalf("SetSocketPermissions failed: %v", err)
	}

	info, err := os.Stat(socketPath)
	if err != nil {
		t.Fatalf("failed to stat socket: %v", err)
	}

	if info.Mode().Perm() != 0o600 {
		t.Errorf("expected permissions 0600, got %o", info.Mode().Perm())
	}
}
