"""Simple worker for basic E2E testing."""

from __future__ import annotations

from pyproc_worker import expose, run_worker


@expose
def echo(req):
    """Echo back the message."""
    return {"echo": req.get("message", "")}


@expose
def add(req):
    """Add two numbers."""
    a = req["a"]
    b = req["b"]
    return {"result": a + b}


@expose
def uppercase(req):
    """Convert text to uppercase."""
    text = req["text"]
    return {"result": text.upper()}


if __name__ == "__main__":
    run_worker()
