package pyproc

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/viper"
)

func TestSetDefaults(t *testing.T) {
	v := viper.New()
	setDefaults(v)

	if v.GetInt("pool.workers") != 4 {
		t.Fatalf("unexpected default workers: %d", v.GetInt("pool.workers"))
	}
	if v.GetInt("protocol.request_timeout") != 60 {
		t.Fatalf("unexpected request timeout seconds: %d", v.GetInt("protocol.request_timeout"))
	}
	if v.GetString("logging.level") != "info" {
		t.Fatalf("unexpected logging level: %s", v.GetString("logging.level"))
	}
}

func TestLoadConfigOverrides(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "config.yaml")
	content := []byte(`
pool:
  workers: 2
  start_timeout: 5
protocol:
  request_timeout: 10
logging:
  level: debug
`)
	if err := os.WriteFile(configPath, content, 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.Pool.Workers != 2 {
		t.Fatalf("expected 2 workers, got %d", cfg.Pool.Workers)
	}
	if cfg.Pool.StartTimeout != 5*time.Second {
		t.Fatalf("expected 5s start timeout, got %s", cfg.Pool.StartTimeout)
	}
	if cfg.Protocol.RequestTimeout != 10*time.Second {
		t.Fatalf("expected 10s request timeout, got %s", cfg.Protocol.RequestTimeout)
	}
	if cfg.Logging.Level != "debug" {
		t.Fatalf("expected debug level, got %s", cfg.Logging.Level)
	}
}
