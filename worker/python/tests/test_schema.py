"""Tests for schema capture functionality."""

import importlib.util
import sys
from pathlib import Path

import pytest


def load_worker_module(worker_path: Path):
    """Load a worker module from a file path."""
    spec = importlib.util.spec_from_file_location("test_worker", worker_path)
    if spec and spec.loader:
        module = importlib.util.module_from_spec(spec)
        sys.modules["test_worker"] = module
        spec.loader.exec_module(module)
        return module
    msg = f"Failed to load worker module from {worker_path}"
    raise ImportError(msg)


def test_expose_captures_simple_types():
    """Test that @expose captures simple type hints (int, float, str, bool)."""
    # Import pyproc_worker to access the schema registry
    import pyproc_worker

    # Load the test worker with type hints
    fixtures_dir = Path(__file__).parent / "fixtures"
    worker_path = fixtures_dir / "worker_with_types.py"
    load_worker_module(worker_path)

    # Get the captured schemas
    schemas = pyproc_worker.get_exposed_schemas()

    assert "schema_version" in schemas
    assert "functions" in schemas
    functions = schemas["functions"]

    # Test func_with_simple_types: int -> dict[str, float]
    assert "func_with_simple_types" in functions
    simple_schema = functions["func_with_simple_types"]
    assert simple_schema["name"] == "func_with_simple_types"
    assert len(simple_schema["parameters"]) == 1
    assert simple_schema["parameters"][0]["name"] == "value"
    assert simple_schema["parameters"][0]["type"]["type"] == "integer"
    assert simple_schema["return_type"]["type"] == "object"

    # Test func_with_bool: bool -> bool
    assert "func_with_bool" in functions
    bool_schema = functions["func_with_bool"]
    assert bool_schema["parameters"][0]["type"]["type"] == "boolean"
    assert bool_schema["return_type"]["type"] == "boolean"

    # Test func_with_str: str -> str
    assert "func_with_str" in functions
    str_schema = functions["func_with_str"]
    assert str_schema["parameters"][0]["type"]["type"] == "string"
    assert str_schema["return_type"]["type"] == "string"


def test_expose_handles_missing_type_hints():
    """Test that @expose handles functions without type hints gracefully."""
    import pyproc_worker

    # Load the test worker
    fixtures_dir = Path(__file__).parent / "fixtures"
    worker_path = fixtures_dir / "worker_with_types.py"
    load_worker_module(worker_path)

    schemas = pyproc_worker.get_exposed_schemas()
    functions = schemas["functions"]

    # func_without_types should be registered with "any" type
    assert "func_without_types" in functions
    no_type_schema = functions["func_without_types"]
    assert no_type_schema["name"] == "func_without_types"
    # Parameters without type hints should default to "any"
    assert no_type_schema["parameters"][0]["type"]["type"] == "any"
    assert no_type_schema["return_type"]["type"] == "any"
