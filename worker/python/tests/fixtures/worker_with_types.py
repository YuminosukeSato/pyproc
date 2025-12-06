"""Test worker with type hints for schema testing."""

from __future__ import annotations

from dataclasses import dataclass

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


# Generic type examples
@expose
def func_with_generic_dict(data: dict[str, int]) -> dict[str, int]:
    """Function with generic dict type."""
    return {k: v * 2 for k, v in data.items()}


@expose
def func_with_generic_list(numbers: list[float]) -> list[float]:
    """Function with generic list type."""
    return [n * 2.0 for n in numbers]


@expose
def func_with_nested_generic(data: list[dict[str, int]]) -> list[dict[str, int]]:
    """Function with nested generic types."""
    return [{k: v + 1 for k, v in item.items()} for item in data]


# Dataclass examples
@dataclass(frozen=True)
class FrozenRequest:
    """Frozen dataclass for request (recommended pattern)."""

    value: int
    name: str


@dataclass(frozen=True)
class FrozenResponse:
    """Frozen dataclass for response (recommended pattern)."""

    result: int
    message: str


@dataclass
class MutableRequest:
    """Regular (mutable) dataclass for request (backward compatibility)."""

    value: int
    name: str


@dataclass
class MutableResponse:
    """Regular (mutable) dataclass for response (backward compatibility)."""

    result: int
    message: str


@expose
def func_with_frozen_dataclass(req: FrozenRequest) -> FrozenResponse:
    """Function with frozen dataclass (recommended pattern)."""
    return FrozenResponse(result=req.value * 2, message=f"Hello, {req.name}!")


@expose
def func_with_mutable_dataclass(req: MutableRequest) -> MutableResponse:
    """Function with mutable dataclass (backward compatibility)."""
    return MutableResponse(result=req.value * 3, message=f"Hi, {req.name}!")


@dataclass(frozen=True)
class NestedInner:
    """Inner dataclass for nested example."""

    count: int
    label: str


@dataclass(frozen=True)
class NestedOuter:
    """Outer dataclass containing another dataclass."""

    inner: NestedInner
    total: int


@expose
def func_with_nested_dataclass(req: NestedOuter) -> NestedOuter:
    """Function with nested dataclass."""
    new_inner = NestedInner(count=req.inner.count + 1, label=req.inner.label.upper())
    return NestedOuter(inner=new_inner, total=req.total + req.inner.count)
