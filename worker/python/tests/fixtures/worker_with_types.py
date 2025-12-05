"""Test worker with type hints for schema testing."""

from pyproc_worker import expose


@expose
def func_with_simple_types(value: int) -> dict[str, float]:
    """Function with simple type hints."""
    return {"result": float(value * 2)}


@expose
def func_with_bool(flag: bool) -> bool:
    """Function with boolean type."""
    return not flag


@expose
def func_with_str(text: str) -> str:
    """Function with string type."""
    return text.upper()


@expose
def func_without_types(data):
    """Function without type hints."""
    return {"status": "ok"}


@expose
def func_with_complex_return(x: int) -> dict[str, int | float]:
    """Function with complex return type."""
    return {"value": x, "doubled": x * 2.0}
