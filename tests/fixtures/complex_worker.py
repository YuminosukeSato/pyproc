"""Complex worker with advanced types for E2E testing."""

from __future__ import annotations

from dataclasses import dataclass
from typing import Any

from pyproc_worker import expose, run_worker


@dataclass
class Point:
    """Simple dataclass for testing."""

    x: float
    y: float


@expose
def process_nested(req: dict[str, Any]) -> dict[str, Any]:
    """Process nested structures."""
    data = req["data"]
    metadata = req.get("metadata", {})

    result = {
        "processed": {
            "items": data.get("items", []),
            "count": len(data.get("items", [])),
        },
        "metadata": {
            "original_keys": list(metadata.keys()),
            "timestamp": metadata.get("timestamp", "unknown"),
        },
    }

    return result


@expose
def calculate_distance(req: dict[str, Any]) -> dict[str, Any]:
    """Calculate distance using dataclass-like structure."""
    p1 = req["point1"]
    p2 = req["point2"]

    dx = p2["x"] - p1["x"]
    dy = p2["y"] - p1["y"]
    distance = (dx**2 + dy**2) ** 0.5

    return {
        "distance": distance,
        "points": {
            "start": p1,
            "end": p2,
        },
    }


@expose
def aggregate_data(req: dict[str, Any]) -> dict[str, Any]:
    """Aggregate data with multiple operations."""
    numbers: list[float] = req["numbers"]
    operation: str = req.get("operation", "sum")

    if operation == "sum":
        result = sum(numbers)
    elif operation == "avg":
        result = sum(numbers) / len(numbers) if numbers else 0
    elif operation == "max":
        result = max(numbers) if numbers else 0
    elif operation == "min":
        result = min(numbers) if numbers else 0
    else:
        result = 0

    return {
        "result": result,
        "operation": operation,
        "count": len(numbers),
    }


if __name__ == "__main__":
    run_worker()
