package pyproc

import (
	"bytes"
	"encoding/hex"
	"net"
	"path/filepath"
	"testing"
)

func TestNewHMACAuth(t *testing.T) {
	secret := []byte("test-secret-key")
	auth := NewHMACAuth(secret)
	if !bytes.Equal(auth.secret, secret) {
		t.Error("secret mismatch")
	}
}

func TestGenerateSecret(t *testing.T) {
	secret1, err := GenerateSecret()
	if err != nil {
		t.Fatalf("GenerateSecret failed: %v", err)
	}

	if len(secret1) != 32 {
		t.Errorf("expected 32 bytes, got %d", len(secret1))
	}

	secret2, err := GenerateSecret()
	if err != nil {
		t.Fatalf("GenerateSecret failed: %v", err)
	}

	if bytes.Equal(secret1, secret2) {
		t.Error("generated secrets should be unique")
	}
}

func TestSecretFromString(t *testing.T) {
	secret := SecretFromString("my-password")

	if len(secret) != 32 {
		t.Errorf("expected 32 bytes (SHA256 output), got %d", len(secret))
	}

	secret2 := SecretFromString("my-password")
	if !bytes.Equal(secret, secret2) {
		t.Error("same input should produce same secret")
	}

	secret3 := SecretFromString("different-password")
	if bytes.Equal(secret, secret3) {
		t.Error("different input should produce different secret")
	}
}

func TestSecretFromHex(t *testing.T) {
	hexStr := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	secret, err := SecretFromHex(hexStr)
	if err != nil {
		t.Fatalf("SecretFromHex failed: %v", err)
	}

	if len(secret) != 32 {
		t.Errorf("expected 32 bytes, got %d", len(secret))
	}

	expected, _ := hex.DecodeString(hexStr)
	if !bytes.Equal(secret, expected) {
		t.Error("decoded secret mismatch")
	}
}

func TestSecretFromHex_InvalidHex(t *testing.T) {
	_, err := SecretFromHex("not-valid-hex")
	if err == nil {
		t.Error("expected error for invalid hex")
	}
}

func TestHMACAuthClientServer(t *testing.T) {
	requireUnixSocket(t)
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "hmac-test.sock")

	secret := []byte("shared-secret-key-for-testing")
	serverAuth := NewHMACAuth(secret)
	clientAuth := NewHMACAuth(secret)

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer func() { _ = listener.Close() }()

	serverErr := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			serverErr <- err
			return
		}
		defer func() { _ = conn.Close() }()
		serverErr <- serverAuth.AuthenticateServer(conn)
	}()

	clientConn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer func() { _ = clientConn.Close() }()

	err = clientAuth.AuthenticateClient(clientConn)
	if err != nil {
		t.Errorf("client authentication failed: %v", err)
	}

	if err := <-serverErr; err != nil {
		t.Errorf("server authentication failed: %v", err)
	}
}

func TestHMACAuthWrongSecret(t *testing.T) {
	requireUnixSocket(t)
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "w.sock")

	serverAuth := NewHMACAuth([]byte("server-secret"))
	clientAuth := NewHMACAuth([]byte("wrong-client-secret"))

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer func() { _ = listener.Close() }()

	serverErr := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			serverErr <- err
			return
		}
		defer func() { _ = conn.Close() }()
		serverErr <- serverAuth.AuthenticateServer(conn)
	}()

	clientConn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer func() { _ = clientConn.Close() }()

	err = clientAuth.AuthenticateClient(clientConn)
	if err == nil {
		t.Error("expected authentication to fail with wrong secret")
	}

	if err := <-serverErr; err == nil {
		t.Error("expected server to detect authentication failure")
	}
}

func TestNewHMACListener(t *testing.T) {
	requireUnixSocket(t)
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "hmac-listener.sock")

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}

	secret := []byte("test-secret")
	hmacListener := NewHMACListener(listener, secret)
	if hmacListener.auth == nil {
		t.Error("expected non-nil auth in HMACListener")
	}

	_ = hmacListener.Close()
}

func TestSecureConnIsAuthenticated(t *testing.T) {
	conn := &SecureConn{
		authenticated: true,
	}

	if !conn.IsAuthenticated() {
		t.Error("expected IsAuthenticated to return true")
	}

	conn.authenticated = false
	if conn.IsAuthenticated() {
		t.Error("expected IsAuthenticated to return false")
	}
}

func TestHMACListener_Accept(t *testing.T) {
	requireUnixSocket(t)
	socketPath := "/tmp/hmac-accept-test.sock"

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer func() { _ = listener.Close() }()

	secret := []byte("test-secret-12345")
	hmacListener := NewHMACListener(listener, secret)

	acceptErr := make(chan error, 1)
	go func() {
		conn, err := hmacListener.Accept()
		if err != nil {
			acceptErr <- err
			return
		}
		_ = conn.Close()
		acceptErr <- nil
	}()

	clientAuth := NewHMACAuth(secret)
	clientConn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer func() { _ = clientConn.Close() }()

	err = clientAuth.AuthenticateClient(clientConn)
	if err != nil {
		t.Errorf("client auth failed: %v", err)
	}

	if err := <-acceptErr; err != nil {
		t.Errorf("Accept failed: %v", err)
	}
}

func TestDialSecure(t *testing.T) {
	requireUnixSocket(t)
	socketPath := "/tmp/dial-secure-test.sock"

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer func() { _ = listener.Close() }()

	secret := []byte("dial-secure-secret")

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		serverAuth := NewHMACAuth(secret)
		_ = serverAuth.AuthenticateServer(conn)
	}()

	secureConn, err := DialSecure("unix", socketPath, secret)
	if err != nil {
		t.Fatalf("DialSecure failed: %v", err)
	}
	defer func() { _ = secureConn.Close() }()

	if !secureConn.IsAuthenticated() {
		t.Error("expected authenticated connection")
	}
}

func TestDialSecure_ConnectionError(t *testing.T) {
	_, err := DialSecure("unix", "/tmp/nonexistent-socket.sock", []byte("secret"))
	if err == nil {
		t.Error("expected error for nonexistent socket")
	}
}
