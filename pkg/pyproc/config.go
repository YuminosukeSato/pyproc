// Package pyproc provides a Go library for calling Python functions
// without CGO, using Unix Domain Sockets for high-performance IPC.
package pyproc

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// Config holds all configuration for pyproc
type Config struct {
	Pool     PoolConfig     `mapstructure:"pool"`
	Python   PythonConfig   `mapstructure:"python"`
	Socket   SocketConfig   `mapstructure:"socket"`
	Protocol ProtocolConfig `mapstructure:"protocol"`
	Logging  LoggingConfig  `mapstructure:"logging"`
	Metrics  MetricsConfig  `mapstructure:"metrics"`
}

// PoolConfig defines worker pool settings
type PoolConfig struct {
	Workers        int           `mapstructure:"workers"`
	MaxInFlight    int           `mapstructure:"max_in_flight"`
	StartTimeout   time.Duration `mapstructure:"start_timeout"`
	HealthInterval time.Duration `mapstructure:"health_interval"`
	Restart        RestartConfig `mapstructure:"restart"`
}

// RestartConfig defines restart policy
type RestartConfig struct {
	MaxAttempts    int           `mapstructure:"max_attempts"`
	InitialBackoff time.Duration `mapstructure:"initial_backoff"`
	MaxBackoff     time.Duration `mapstructure:"max_backoff"`
	Multiplier     float64       `mapstructure:"multiplier"`
}

// PythonConfig defines Python runtime settings
type PythonConfig struct {
	Executable        string            `mapstructure:"executable"`
	WorkerScript      string            `mapstructure:"worker_script"`
	Env               map[string]string `mapstructure:"env"`
	WorkerConcurrency int               `mapstructure:"worker_concurrency"`
}

// SocketConfig defines Unix domain socket settings
type SocketConfig struct {
	Dir         string `mapstructure:"dir"`
	Prefix      string `mapstructure:"prefix"`
	Permissions uint32 `mapstructure:"permissions"`
}

// ProtocolConfig defines protocol settings
type ProtocolConfig struct {
	MaxFrameSize      int           `mapstructure:"max_frame_size"`
	RequestTimeout    time.Duration `mapstructure:"request_timeout"`
	ConnectionTimeout time.Duration `mapstructure:"connection_timeout"`
}

// LoggingConfig defines logging settings
type LoggingConfig struct {
	Level        string `mapstructure:"level"`
	Format       string `mapstructure:"format"`
	TraceEnabled bool   `mapstructure:"trace_enabled"`
}

// MetricsConfig defines metrics collection settings
type MetricsConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	Endpoint string `mapstructure:"endpoint"`
	Path     string `mapstructure:"path"`
}

// LoadConfig loads configuration from file and environment
func LoadConfig(configPath string) (*Config, error) {
	v := viper.New()

	// Set defaults
	setDefaults(v)

	// Set config file
	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")
		v.AddConfigPath("./config")
		v.AddConfigPath("/etc/pyproc")
	}

	// Read environment variables
	v.SetEnvPrefix("PYPROC")
	v.AutomaticEnv()

	// Read config file
	if err := v.ReadInConfig(); err != nil {
		// It's ok if config file doesn't exist, we have defaults
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config: %w", err)
		}
	}

	// Unmarshal config
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Convert duration fields (viper reads them as seconds)
	cfg.Pool.StartTimeout *= time.Second
	cfg.Pool.HealthInterval *= time.Second
	cfg.Pool.Restart.InitialBackoff *= time.Millisecond
	cfg.Pool.Restart.MaxBackoff *= time.Millisecond
	cfg.Protocol.RequestTimeout *= time.Second
	cfg.Protocol.ConnectionTimeout *= time.Second

	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	// Pool defaults
	v.SetDefault("pool.workers", 4)
	v.SetDefault("pool.max_in_flight", 10)
	v.SetDefault("pool.start_timeout", 30)
	v.SetDefault("pool.health_interval", 30)
	v.SetDefault("pool.restart.max_attempts", 5)
	v.SetDefault("pool.restart.initial_backoff", 1000)
	v.SetDefault("pool.restart.max_backoff", 30000)
	v.SetDefault("pool.restart.multiplier", 2.0)

	// Python defaults
	v.SetDefault("python.executable", "python3")
	v.SetDefault("python.worker_script", "./worker.py")
	v.SetDefault("python.env", map[string]string{
		"PYTHONUNBUFFERED": "1",
	})
	v.SetDefault("python.worker_concurrency", 1) // Default to single-threaded for backward compatibility

	// Socket defaults
	v.SetDefault("socket.dir", "/tmp")
	v.SetDefault("socket.prefix", "pyproc")
	v.SetDefault("socket.permissions", 0600)

	// Protocol defaults
	v.SetDefault("protocol.max_frame_size", 10485760) // 10MB
	v.SetDefault("protocol.request_timeout", 60)
	v.SetDefault("protocol.connection_timeout", 5)

	// Logging defaults
	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.format", "json")
	v.SetDefault("logging.trace_enabled", true)

	// Metrics defaults
	v.SetDefault("metrics.enabled", true)
	v.SetDefault("metrics.endpoint", ":9090")
	v.SetDefault("metrics.path", "/metrics")
}
