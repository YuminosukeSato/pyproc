"""Tests for CLI functionality."""

import json
import subprocess
import sys
from pathlib import Path


def test_schema_export_json_format():
    """Test that 'pyproc-worker schema' exports JSON format correctly."""
    # Get path to worker_with_types.py fixture
    fixtures_dir = Path(__file__).parent / "fixtures"
    worker_path = fixtures_dir / "worker_with_types.py"

    # Run pyproc-worker schema command
    result = subprocess.run(
        [sys.executable, "-m", "pyproc_worker.cli", "schema", str(worker_path)],
        capture_output=True,
        text=True,
        check=False,
    )

    # Command should succeed
    assert result.returncode == 0, f"Command failed: {result.stderr}"

    # Parse JSON output
    schema = json.loads(result.stdout)

    # Verify schema structure
    assert "schema_version" in schema
    assert "functions" in schema
    assert schema["schema_version"] == "1.0"

    functions = schema["functions"]

    # Verify that exposed functions are present
    assert "func_with_simple_types" in functions
    assert "func_with_frozen_dataclass" in functions
    assert "health" in functions  # Always available

    # Verify a basic function schema
    simple_func = functions["func_with_simple_types"]
    assert simple_func["name"] == "func_with_simple_types"
    assert "parameters" in simple_func
    assert "return_type" in simple_func
    assert "docstring" in simple_func


def test_schema_export_with_output_file():
    """Test that 'pyproc-worker schema --output' writes to file."""
    import tempfile

    fixtures_dir = Path(__file__).parent / "fixtures"
    worker_path = fixtures_dir / "worker_with_types.py"

    # Create temp file
    with tempfile.NamedTemporaryFile(mode="w", suffix=".json", delete=False) as f:
        output_path = Path(f.name)

    try:
        # Run pyproc-worker schema with output file
        result = subprocess.run(
            [
                sys.executable,
                "-m",
                "pyproc_worker.cli",
                "schema",
                str(worker_path),
                "--output",
                str(output_path),
            ],
            capture_output=True,
            text=True,
            check=False,
        )

        assert result.returncode == 0, f"Command failed: {result.stderr}"

        # Verify file was created and contains valid JSON
        assert output_path.exists()
        schema = json.loads(output_path.read_text())
        assert "schema_version" in schema
        assert "functions" in schema
    finally:
        # Clean up
        if output_path.exists():
            output_path.unlink()


def test_schema_export_go_format():
    """Test that 'pyproc-worker schema --format go' generates Go structs."""
    fixtures_dir = Path(__file__).parent / "fixtures"
    worker_path = fixtures_dir / "worker_with_types.py"

    # Run pyproc-worker schema with Go format
    result = subprocess.run(
        [
            sys.executable,
            "-m",
            "pyproc_worker.cli",
            "schema",
            str(worker_path),
            "--format",
            "go",
        ],
        capture_output=True,
        text=True,
        check=False,
    )

    assert result.returncode == 0, f"Command failed: {result.stderr}"

    # Verify Go code structure
    output = result.stdout
    assert "package main" in output or "type" in output
    assert "struct" in output

    # Should contain structs for dataclass functions
    assert "FuncWithFrozenDataclass" in output or "struct" in output
    assert "Request" in output or "Response" in output

    # Should have json tags
    assert "`json:" in output


def test_run_command_backward_compatibility():
    """Test that default 'run' behavior still works."""
    # This test verifies that the old CLI behavior (just running the worker)
    # still works when no subcommand is specified
    fixtures_dir = Path(__file__).parent / "fixtures"
    worker_path = fixtures_dir / "worker_with_types.py"

    # Try to import the worker (will fail if PYPROC_SOCKET_PATH not set,
    # but that's expected - we're just testing CLI parsing works)
    result = subprocess.run(
        [sys.executable, "-m", "pyproc_worker.cli", str(worker_path), "--help"],
        capture_output=True,
        text=True,
        check=False,
        timeout=2,
    )

    # Should show help without error
    assert "Python worker for pyproc" in result.stdout or result.returncode == 0


def test_detect_env_json_format():
    """Test that 'pyproc-worker detect-env' detects Python environment."""
    result = subprocess.run(
        [sys.executable, "-m", "pyproc_worker.cli", "detect-env", "--format", "json"],
        capture_output=True,
        text=True,
        check=False,
    )

    assert result.returncode == 0, f"Command failed: {result.stderr}"

    # Parse JSON output
    env_info = json.loads(result.stdout)

    # Verify structure
    assert "python_executable" in env_info
    assert "python_version" in env_info
    assert "virtual_env_type" in env_info
    assert "virtual_env_path" in env_info

    # Verify python_executable is valid
    assert env_info["python_executable"] == sys.executable

    # Verify python_version format (e.g., "3.10.0")
    version = env_info["python_version"]
    assert isinstance(version, str)
    parts = version.split(".")
    min_version_parts = 2
    assert len(parts) >= min_version_parts


def test_detect_env_shell_format():
    """Test that 'pyproc-worker detect-env --format shell' outputs shell variables."""
    result = subprocess.run(
        [sys.executable, "-m", "pyproc_worker.cli", "detect-env", "--format", "shell"],
        capture_output=True,
        text=True,
        check=False,
    )

    assert result.returncode == 0, f"Command failed: {result.stderr}"

    # Verify shell export format
    output = result.stdout
    assert "export PYPROC_PYTHON_EXEC=" in output
    assert sys.executable in output


def test_detect_env_detects_venv():
    """Test that detect-env correctly identifies venv environments."""
    result = subprocess.run(
        [sys.executable, "-m", "pyproc_worker.cli", "detect-env", "--format", "json"],
        capture_output=True,
        text=True,
        check=False,
    )

    assert result.returncode == 0

    env_info = json.loads(result.stdout)

    # If running in venv, should detect it
    # sys.prefix != sys.base_prefix indicates venv
    if sys.prefix != sys.base_prefix:
        assert env_info["virtual_env_type"] in ["venv", "virtualenv"]
        assert env_info["virtual_env_path"] is not None


def test_check_worker_valid():
    """Test that 'pyproc-worker check' succeeds for valid worker."""
    fixtures_dir = Path(__file__).parent / "fixtures"
    worker_path = fixtures_dir / "worker_with_types.py"

    result = subprocess.run(
        [sys.executable, "-m", "pyproc_worker.cli", "check", str(worker_path)],
        capture_output=True,
        text=True,
        check=False,
    )

    assert result.returncode == 0, f"Check failed: {result.stderr}"
    assert "✓" in result.stdout or "PASS" in result.stdout


def test_check_worker_nonexistent():
    """Test that 'pyproc-worker check' fails for nonexistent worker."""
    result = subprocess.run(
        [
            sys.executable,
            "-m",
            "pyproc_worker.cli",
            "check",
            "/nonexistent/worker.py",
        ],
        capture_output=True,
        text=True,
        check=False,
    )

    assert result.returncode != 0
    assert "does not exist" in result.stdout or "not found" in result.stdout.lower()


def test_check_worker_syntax_error():
    """Test that 'pyproc-worker check' detects syntax errors."""
    import tempfile

    # Create temp file with syntax error
    with tempfile.NamedTemporaryFile(
        mode="w",
        suffix=".py",
        delete=False,
    ) as f:
        f.write("def invalid syntax here\n")
        temp_path = Path(f.name)

    try:
        result = subprocess.run(
            [sys.executable, "-m", "pyproc_worker.cli", "check", str(temp_path)],
            capture_output=True,
            text=True,
            check=False,
        )

        assert result.returncode != 0
        assert "syntax" in result.stdout.lower() or "invalid" in result.stdout.lower()
    finally:
        if temp_path.exists():
            temp_path.unlink()


def test_check_worker_missing_expose():
    """Test that 'pyproc-worker check' warns about missing @expose functions."""
    import tempfile

    # Create temp file without @expose functions
    with tempfile.NamedTemporaryFile(
        mode="w",
        suffix=".py",
        delete=False,
    ) as f:
        f.write("def some_function():\n    pass\n")
        temp_path = Path(f.name)

    try:
        result = subprocess.run(
            [sys.executable, "-m", "pyproc_worker.cli", "check", str(temp_path)],
            capture_output=True,
            text=True,
            check=False,
        )

        # Should succeed but warn
        assert result.returncode == 0
        assert "@expose" in result.stdout or "no exposed" in result.stdout.lower()
    finally:
        if temp_path.exists():
            temp_path.unlink()
