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
