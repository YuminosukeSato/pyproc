// Package pyproc provides a Go library for calling Python functions
// without CGO, using Unix Domain Sockets for high-performance IPC.
package pyproc

import (
	"context"
	"encoding/json"
	"errors"
	"os/exec"
)

// PythonEnv represents detected Python environment information.
type PythonEnv struct {
	Executable     string
	Version        string
	VirtualEnvType string // "venv", "poetry", "uv", "virtualenv", or ""
	VirtualEnvPath string
}

// DetectPythonEnv attempts to auto-detect Python environment using pyproc-worker CLI.
// It first tries to call 'pyproc-worker detect-env --format json' to get environment
// information from the Python side. If that fails (e.g., CLI not in PATH), it falls
// back to basic detection using exec.LookPath().
//
// Returns PythonEnv with detected executable path, version, and virtual environment info.
func DetectPythonEnv(ctx context.Context) (*PythonEnv, error) {
	// Try primary detection via pyproc-worker CLI
	cmd := exec.CommandContext(ctx, "pyproc-worker", "detect-env", "--format", "json")
	output, err := cmd.Output()
	if err != nil {
		// CLI unavailable or failed - use fallback
		return detectPythonFallback()
	}

	// Parse JSON response
	var detection struct {
		PythonExecutable string `json:"python_executable"`
		PythonVersion    string `json:"python_version"`
		VirtualEnvType   string `json:"virtual_env_type"`
		VirtualEnvPath   string `json:"virtual_env_path"`
	}

	if err := json.Unmarshal(output, &detection); err != nil {
		// JSON parse error - use fallback
		return detectPythonFallback()
	}

	return &PythonEnv{
		Executable:     detection.PythonExecutable,
		Version:        detection.PythonVersion,
		VirtualEnvType: detection.VirtualEnvType,
		VirtualEnvPath: detection.VirtualEnvPath,
	}, nil
}

// detectPythonFallback provides basic Python detection when pyproc-worker CLI is unavailable.
// It searches for common Python executable names in PATH.
func detectPythonFallback() (*PythonEnv, error) {
	candidates := []string{
		"python3",
		"python",
		"python3.12",
		"python3.11",
		"python3.10",
		"python3.9",
	}

	for _, candidate := range candidates {
		if path, err := exec.LookPath(candidate); err == nil {
			return &PythonEnv{
				Executable: path,
				Version:    "", // Version detection would require running --version
			}, nil
		}
	}

	return nil, errors.New("no Python executable found in PATH")
}
