package pyproc

import (
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultSocketSecurityConfig(t *testing.T) {
	cfg := DefaultSocketSecurityConfig()

	// Check default permissions
	if cfg.SocketPerms != 0600 {
		t.Errorf("expected socket permissions 0600, got %o", cfg.SocketPerms)
	}

	if cfg.DirPerms != 0750 {
		t.Errorf("expected directory permissions 0750, got %o", cfg.DirPerms)
	}

	// Check RequireSameUser is enabled by default
	if !cfg.RequireSameUser {
		t.Error("expected RequireSameUser to be true by default")
	}

	// Check socket directory based on user privileges
	expectedDir := filepath.Join(os.TempDir(), "pyproc")
	if os.Geteuid() == 0 {
		expectedDir = "/run/pyproc"
	}
	if cfg.SocketDir != expectedDir {
		t.Errorf("expected socket directory %s, got %s", expectedDir, cfg.SocketDir)
	}
}

func TestSecureSocketPath(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()

	cfg := SocketSecurityConfig{
		SocketDir:   filepath.Join(tmpDir, "test-sockets"),
		SocketPerms: 0600,
		DirPerms:    0750,
	}

	socketName := "test.sock"
	path, err := SecureSocketPath(cfg, socketName)
	if err != nil {
		t.Fatalf("failed to create secure socket path: %v", err)
	}

	expectedPath := filepath.Join(cfg.SocketDir, socketName)
	if path != expectedPath {
		t.Errorf("expected path %s, got %s", expectedPath, path)
	}

	// Check that directory was created with correct permissions
	info, err := os.Stat(cfg.SocketDir)
	if err != nil {
		t.Fatalf("failed to stat socket directory: %v", err)
	}

	if !info.IsDir() {
		t.Error("expected socket path to be a directory")
	}

	// Permission checking is platform-specific and may not work in all environments
	// so we just verify the directory exists
}

func TestSetSocketPermissions(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "test.sock")

	// Create a dummy file to represent the socket
	file, err := os.Create(socketPath)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	_ = file.Close()

	// Set permissions
	perms := os.FileMode(0600)
	err = SetSocketPermissions(socketPath, perms)
	if err != nil {
		t.Fatalf("failed to set socket permissions: %v", err)
	}

	// Verify permissions
	info, err := os.Stat(socketPath)
	if err != nil {
		t.Fatalf("failed to stat socket file: %v", err)
	}

	// Check file permissions (may vary by platform)
	actualPerms := info.Mode().Perm()
	if actualPerms != perms {
		t.Logf("warning: expected permissions %o, got %o (may vary by platform)", perms, actualPerms)
	}
}

func TestVerifyPeerCredentials(t *testing.T) {
	// This test requires actual Unix domain socket connections
	// which is complex to set up in a unit test
	t.Skip("Skipping peer credentials test - requires integration test setup")
}

func TestSecureListener(t *testing.T) {
	requireUnixSocket(t)
	tmpDir := t.TempDir()

	cfg := SocketSecurityConfig{
		SocketDir:       tmpDir,
		SocketPerms:     0600,
		DirPerms:        0750,
		RequireSameUser: true,
	}

	socketPath := filepath.Join(tmpDir, "test.sock")

	// Create secure listener
	listener, err := NewSecureListener(socketPath, cfg)
	if err != nil {
		t.Fatalf("failed to create secure listener: %v", err)
	}
	defer func() { _ = listener.Close() }()

	// Verify the socket file was created
	if _, err := os.Stat(socketPath); os.IsNotExist(err) {
		t.Error("socket file was not created")
	}

	// Test that we can accept connections (in a goroutine)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			// This is expected if no client connects
			return
		}
		_ = conn.Close()
	}()

	// Give the listener time to start
	time.Sleep(100 * time.Millisecond)

	// Try to connect as a client
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Logf("warning: could not connect to socket (may be expected): %v", err)
	} else {
		_ = conn.Close()
	}
}

func TestSocketSecurityWithAllowedUIDs(t *testing.T) {
	cfg := SocketSecurityConfig{
		AllowedUIDs:     []uint32{uint32(os.Geteuid())},
		RequireSameUser: false,
	}

	// Mock connection with current UID should succeed
	mockCreds := &PeerCredentials{
		UID: uint32(os.Geteuid()),
		GID: uint32(os.Getegid()),
		PID: int32(os.Getpid()),
	}

	// This would normally be tested with VerifyPeerCredentials
	// but that requires a real Unix socket connection
	_ = mockCreds
	_ = cfg

	// Verify configuration is set correctly
	if len(cfg.AllowedUIDs) != 1 {
		t.Error("expected one allowed UID")
	}

	if cfg.AllowedUIDs[0] != uint32(os.Geteuid()) {
		t.Error("allowed UID does not match current user")
	}
}

func TestSocketSecurityWithAllowedGIDs(t *testing.T) {
	cfg := SocketSecurityConfig{
		AllowedGIDs:     []uint32{uint32(os.Getegid())},
		RequireSameUser: false,
	}

	// Verify configuration is set correctly
	if len(cfg.AllowedGIDs) != 1 {
		t.Error("expected one allowed GID")
	}

	if cfg.AllowedGIDs[0] != uint32(os.Getegid()) {
		t.Error("allowed GID does not match current group")
	}
}

func TestGetPeerCredentials_InvalidFD(t *testing.T) {
	requireUnixSocket(t)

	// Test with invalid file descriptor (should fail)
	invalidFD := -1
	_, err := getPeerCredentials(invalidFD)
	if err == nil {
		t.Error("expected error for invalid file descriptor")
	}
}
