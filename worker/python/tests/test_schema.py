"""Tests for schema capture functionality."""

import importlib.util
import sys
from pathlib import Path


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


def test_expose_captures_generic_dict():
    """Test that @expose captures dict[K, V] generic type hints."""
    import pyproc_worker

    # Load the test worker
    fixtures_dir = Path(__file__).parent / "fixtures"
    worker_path = fixtures_dir / "worker_with_types.py"
    load_worker_module(worker_path)

    schemas = pyproc_worker.get_exposed_schemas()
    functions = schemas["functions"]

    # Test func_with_generic_dict: dict[str, int] -> dict[str, int]
    assert "func_with_generic_dict" in functions
    dict_schema = functions["func_with_generic_dict"]
    assert dict_schema["name"] == "func_with_generic_dict"

    # Check parameter type: dict[str, int]
    param = dict_schema["parameters"][0]
    assert param["name"] == "data"
    assert param["type"]["type"] == "object"
    assert "additionalProperties" in param["type"]
    assert param["type"]["additionalProperties"]["type"] == "integer"

    # Check return type: dict[str, int]
    return_type = dict_schema["return_type"]
    assert return_type["type"] == "object"
    assert return_type["additionalProperties"]["type"] == "integer"


def test_expose_captures_generic_list():
    """Test that @expose captures list[T] generic type hints."""
    import pyproc_worker

    # Load the test worker
    fixtures_dir = Path(__file__).parent / "fixtures"
    worker_path = fixtures_dir / "worker_with_types.py"
    load_worker_module(worker_path)

    schemas = pyproc_worker.get_exposed_schemas()
    functions = schemas["functions"]

    # Test func_with_generic_list: list[float] -> list[float]
    assert "func_with_generic_list" in functions
    list_schema = functions["func_with_generic_list"]
    assert list_schema["name"] == "func_with_generic_list"

    # Check parameter type: list[float]
    param = list_schema["parameters"][0]
    assert param["name"] == "numbers"
    assert param["type"]["type"] == "array"
    assert param["type"]["items"]["type"] == "number"

    # Check return type: list[float]
    return_type = list_schema["return_type"]
    assert return_type["type"] == "array"
    assert return_type["items"]["type"] == "number"


def test_expose_captures_nested_generic():
    """Test that @expose captures nested generic types like list[dict[str, int]]."""
    import pyproc_worker

    # Load the test worker
    fixtures_dir = Path(__file__).parent / "fixtures"
    worker_path = fixtures_dir / "worker_with_types.py"
    load_worker_module(worker_path)

    schemas = pyproc_worker.get_exposed_schemas()
    functions = schemas["functions"]

    # Test func_with_nested_generic: list[dict[str, int]] -> list[dict[str, int]]
    assert "func_with_nested_generic" in functions
    nested_schema = functions["func_with_nested_generic"]
    assert nested_schema["name"] == "func_with_nested_generic"

    # Check parameter type: list[dict[str, int]]
    param = nested_schema["parameters"][0]
    assert param["name"] == "data"
    assert param["type"]["type"] == "array"
    # Items should be dict[str, int]
    items = param["type"]["items"]
    assert items["type"] == "object"
    assert items["additionalProperties"]["type"] == "integer"

    # Check return type: list[dict[str, int]]
    return_type = nested_schema["return_type"]
    assert return_type["type"] == "array"
    return_items = return_type["items"]
    assert return_items["type"] == "object"
    assert return_items["additionalProperties"]["type"] == "integer"


def test_expose_captures_frozen_dataclass():
    """Test that @expose captures frozen dataclass types."""
    import pyproc_worker

    # Load the test worker
    fixtures_dir = Path(__file__).parent / "fixtures"
    worker_path = fixtures_dir / "worker_with_types.py"
    load_worker_module(worker_path)

    schemas = pyproc_worker.get_exposed_schemas()
    functions = schemas["functions"]

    # Test func_with_frozen_dataclass: FrozenRequest -> FrozenResponse
    assert "func_with_frozen_dataclass" in functions
    frozen_schema = functions["func_with_frozen_dataclass"]
    assert frozen_schema["name"] == "func_with_frozen_dataclass"

    # Check parameter type: FrozenRequest dataclass
    param = frozen_schema["parameters"][0]
    assert param["name"] == "req"
    assert param["type"]["type"] == "object"
    assert "properties" in param["type"]

    # Verify dataclass fields
    props = param["type"]["properties"]
    assert "value" in props
    assert props["value"]["type"] == "integer"
    assert "name" in props
    assert props["name"]["type"] == "string"

    # Verify frozen=True is captured
    assert param["type"].get("frozen") is True

    # Check return type: FrozenResponse dataclass
    return_type = frozen_schema["return_type"]
    assert return_type["type"] == "object"
    assert "properties" in return_type
    return_props = return_type["properties"]
    assert "result" in return_props
    assert return_props["result"]["type"] == "integer"
    assert "message" in return_props
    assert return_props["message"]["type"] == "string"
    assert return_type.get("frozen") is True


def test_expose_captures_mutable_dataclass():
    """Test that @expose captures mutable (non-frozen) dataclass types."""
    import pyproc_worker

    # Load the test worker
    fixtures_dir = Path(__file__).parent / "fixtures"
    worker_path = fixtures_dir / "worker_with_types.py"
    load_worker_module(worker_path)

    schemas = pyproc_worker.get_exposed_schemas()
    functions = schemas["functions"]

    # Test func_with_mutable_dataclass: MutableRequest -> MutableResponse
    assert "func_with_mutable_dataclass" in functions
    mutable_schema = functions["func_with_mutable_dataclass"]
    assert mutable_schema["name"] == "func_with_mutable_dataclass"

    # Check parameter type: MutableRequest dataclass
    param = mutable_schema["parameters"][0]
    assert param["name"] == "req"
    assert param["type"]["type"] == "object"
    assert "properties" in param["type"]

    # Verify dataclass fields
    props = param["type"]["properties"]
    assert "value" in props
    assert props["value"]["type"] == "integer"
    assert "name" in props
    assert props["name"]["type"] == "string"

    # Verify frozen=False (or not present)
    assert param["type"].get("frozen") is False

    # Check return type: MutableResponse dataclass
    return_type = mutable_schema["return_type"]
    assert return_type["type"] == "object"
    assert "properties" in return_type
    return_props = return_type["properties"]
    assert "result" in return_props
    assert return_props["result"]["type"] == "integer"
    assert "message" in return_props
    assert return_props["message"]["type"] == "string"
    assert return_type.get("frozen") is False


def test_expose_captures_nested_dataclass():
    """Test that @expose captures nested dataclass types."""
    import pyproc_worker

    # Load the test worker
    fixtures_dir = Path(__file__).parent / "fixtures"
    worker_path = fixtures_dir / "worker_with_types.py"
    load_worker_module(worker_path)

    schemas = pyproc_worker.get_exposed_schemas()
    functions = schemas["functions"]

    # Test func_with_nested_dataclass: NestedOuter -> NestedOuter
    assert "func_with_nested_dataclass" in functions
    nested_schema = functions["func_with_nested_dataclass"]
    assert nested_schema["name"] == "func_with_nested_dataclass"

    # Check parameter type: NestedOuter dataclass
    param = nested_schema["parameters"][0]
    assert param["name"] == "req"
    assert param["type"]["type"] == "object"
    assert "properties" in param["type"]

    # Verify outer dataclass fields
    props = param["type"]["properties"]
    assert "inner" in props
    assert "total" in props
    assert props["total"]["type"] == "integer"

    # Verify inner dataclass (NestedInner)
    inner = props["inner"]
    assert inner["type"] == "object"
    assert "properties" in inner
    inner_props = inner["properties"]
    assert "count" in inner_props
    assert inner_props["count"]["type"] == "integer"
    assert "label" in inner_props
    assert inner_props["label"]["type"] == "string"
    assert inner.get("frozen") is True

    # Check return type: NestedOuter dataclass
    return_type = nested_schema["return_type"]
    assert return_type["type"] == "object"
    assert "properties" in return_type
    return_props = return_type["properties"]
    assert "inner" in return_props
    assert "total" in return_props

    # Verify nested inner in return type
    return_inner = return_props["inner"]
    assert return_inner["type"] == "object"
    assert "properties" in return_inner
    assert return_inner.get("frozen") is True
