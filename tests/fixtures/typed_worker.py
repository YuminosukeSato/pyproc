"""Typed worker for schema export E2E testing."""

from __future__ import annotations

from typing import Any

from pyproc_worker import expose, run_worker


@expose
def predict(req: dict[str, Any]) -> dict[str, Any]:
    """
    Prediction function with type hints.

    Args:
        req: Request with "features" field containing list of floats

    Returns:
        Response with "prediction" and "confidence" fields
    """
    features: list[float] = req["features"]
    # Simple mock prediction: sum of features
    prediction = sum(features)
    confidence = min(1.0, abs(prediction) / 10.0)

    return {
        "prediction": prediction,
        "confidence": confidence,
    }


@expose
def transform(req: dict[str, Any]) -> dict[str, Any]:
    """
    Transform function with type hints.

    Args:
        req: Request with "data" field containing list of values

    Returns:
        Response with "transformed" field
    """
    data: list[Any] = req["data"]
    transformed = [x * 2 if isinstance(x, (int, float)) else x for x in data]

    return {"transformed": transformed}


@expose
def preprocess(req: dict[str, Any]) -> dict[str, Any]:
    """
    Preprocess function with type hints.

    Args:
        req: Request with "text" field

    Returns:
        Response with "tokens" and "count" fields
    """
    text: str = req["text"]
    tokens = text.lower().split()

    return {
        "tokens": tokens,
        "count": len(tokens),
    }


if __name__ == "__main__":
    run_worker()
