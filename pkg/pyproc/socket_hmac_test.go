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

	if auth == nil {
		t.Fatal("expected non-nil HMACAuth")
	} else if !bytes.Equal(auth.secret, secret) {
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
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	hmacListener := NewHMACListener(listener, secret)
	defer func() { _ = hmacListener.Close() }()

	connCh := make(chan net.Conn, 1)
	errCh := make(chan error, 1)
	go func() {
		conn, err := hmacListener.Accept()
		connCh <- conn
		errCh <- err
	}()

	clientConn, err := DialSecure("unix", socketPath, secret)
	if err != nil {
		t.Fatalf("DialSecure failed: %v", err)
	}
	defer func() { _ = clientConn.Close() }()

	serverConn := <-connCh
	if err := <-errCh; err != nil {
		t.Fatalf("Accept failed: %v", err)
	}
	if serverConn == nil {
		t.Fatal("expected server connection")
	}

	payload := []byte{1}
	if _, err := serverConn.Write(payload); err != nil {
		t.Fatalf("server write failed: %v", err)
	}
	buf := make([]byte, 1)
	if _, err := clientConn.Read(buf); err != nil {
		t.Fatalf("client read failed: %v", err)
	}
	if !bytes.Equal(buf, payload) {
		t.Fatalf("unexpected payload: %v", buf)
	}
	_ = serverConn.Close()
}

func TestHMACAuthWrongSecret(t *testing.T) {
	requireUnixSocket(t)
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "w.sock")

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	hmacListener := NewHMACListener(listener, []byte("server-secret"))
	errCh := make(chan error, 1)
	go func() {
		_, err := hmacListener.Accept()
		errCh <- err
	}()

	if _, err := DialSecure("unix", socketPath, []byte("wrong-client-secret")); err == nil {
		t.Fatal("expected DialSecure to fail with wrong secret")
	}
	if err := <-errCh; err == nil {
		t.Error("expected server to detect authentication failure")
	}
	_ = hmacListener.Close()
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

	if hmacListener == nil {
		t.Fatal("expected non-nil HMACListener")
	} else {
		if hmacListener.auth == nil {
			t.Error("expected non-nil auth in HMACListener")
		}
		_ = hmacListener.Close()
	}
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
