"""Tests for worker request processing and public API."""

from __future__ import annotations

import os

import pytest

import pyproc_worker
from pyproc_worker import Worker, expose, health
from pyproc_worker.cancellation import CancellationError


@pytest.fixture
def exposed_registry(monkeypatch):
    """Provide an isolated exposed function registry."""
    registry = {}
    monkeypatch.setattr(pyproc_worker, "_exposed_functions", registry)
    return registry


@pytest.fixture
def socket_path(tmp_path):
    """Provide a temporary socket path for worker instantiation."""
    return tmp_path / "pyproc.sock"


def test_expose_registers_function(exposed_registry) -> None:
    """Expose should register the function by name."""

    @expose
    def sample(req):
        return req

    assert exposed_registry["sample"] is sample


def test_process_request_returns_not_found(exposed_registry, socket_path) -> None:
    """_process_request should return not found for unknown methods."""
    worker = Worker(str(socket_path), codec_type="json")

    response = worker._process_request(  # noqa: SLF001
        {"id": 1, "method": "missing", "body": {}},
    )

    assert response == {
        "id": 1,
        "ok": False,
        "error": "Method 'missing' not found",
    }


def test_process_request_success_with_cancel_event(exposed_registry, socket_path) -> None:
    """_process_request should pass cancel_event when supported."""
    event_seen = {"value": None}

    def func(body, cancel_event):
        event_seen["value"] = cancel_event
        return {"echo": body}

    exposed_registry["do"] = func
    worker = Worker(str(socket_path), codec_type="json")
    req_id = 2

    response = worker._process_request(  # noqa: SLF001
        {"id": req_id, "method": "do", "body": {"x": 1}},
    )

    assert response["id"] == req_id
    assert response["ok"] is True
    assert response["body"] == {"echo": {"x": 1}}
    assert event_seen["value"] is not None


def test_process_request_handles_cancellation_error(exposed_registry, socket_path) -> None:
    """_process_request should return cancellation error details."""

    def func(_body):
        raise CancellationError(3, "stop")

    exposed_registry["cancel"] = func
    worker = Worker(str(socket_path), codec_type="json")
    req_id = 3

    response = worker._process_request(  # noqa: SLF001
        {"id": req_id, "method": "cancel", "body": {}},
    )

    assert response == {"id": req_id, "ok": False, "error": "Cancelled: stop"}


def test_process_request_handles_generic_exception(exposed_registry, socket_path) -> None:
    """_process_request should return error string for exceptions."""

    def func(_body):
        raise ValueError("boom")

    exposed_registry["explode"] = func
    worker = Worker(str(socket_path), codec_type="json")
    req_id = 4

    response = worker._process_request(  # noqa: SLF001
        {"id": req_id, "method": "explode", "body": {}},
    )

    assert response["id"] == req_id
    assert response["ok"] is False
    assert response["error"] == "boom"


def test_process_request_records_exception_with_tracing_enabled(
    exposed_registry,
    monkeypatch,
    socket_path,
) -> None:
    """_process_request should record exceptions when tracing is enabled."""
    monkeypatch.setenv("PYPROC_TRACING_ENABLED", "true")
    monkeypatch.setenv("PYPROC_TRACE_CONSOLE", "false")

    from pyproc_worker import tracing

    monkeypatch.setattr(tracing, "_global_tracing", None)

    def func(_body):
        raise ValueError("trace-error")

    exposed_registry["trace"] = func
    worker = Worker(str(socket_path), codec_type="json")

    response = worker._process_request(  # noqa: SLF001
        {"id": 5, "method": "trace", "body": {}},
    )

    assert response["ok"] is False
    assert response["error"] == "trace-error"


def test_handle_cancellation_invokes_manager(monkeypatch, socket_path) -> None:
    """_handle_cancellation should forward to cancellation manager."""
    worker = Worker(str(socket_path), codec_type="json")
    captured = {}

    def fake_cancel(request_id, reason):
        captured["id"] = request_id
        captured["reason"] = reason
        return True

    monkeypatch.setattr(worker.cancellation_manager, "cancel_request", fake_cancel)
    worker._handle_cancellation({"id": 9, "reason": "bye"})  # noqa: SLF001

    assert captured == {"id": 9, "reason": "bye"}


def test_run_worker_uses_env_and_codec(monkeypatch, socket_path) -> None:
    """run_worker should read environment and call Worker.start."""
    created = {}

    class DummyWorker:
        def __init__(self, socket_path: str, codec_type: str = "auto") -> None:
            created["socket_path"] = socket_path
            created["codec_type"] = codec_type

        def start(self) -> None:
            created["started"] = True

    monkeypatch.setenv("PYPROC_SOCKET_PATH", str(socket_path))
    monkeypatch.setenv("PYPROC_CODEC_TYPE", "json")
    monkeypatch.setattr(pyproc_worker, "Worker", DummyWorker)

    pyproc_worker.run_worker()

    assert created == {
        "socket_path": str(socket_path),
        "codec_type": "json",
        "started": True,
    }


def test_run_worker_requires_socket_path(monkeypatch) -> None:
    """run_worker should raise when no socket path is provided."""
    monkeypatch.delenv("PYPROC_SOCKET_PATH", raising=False)
    monkeypatch.delenv("PYPROC_CODEC_TYPE", raising=False)

    with pytest.raises(ValueError, match="Socket path must be provided"):
        pyproc_worker.run_worker()


def test_health_returns_status_and_pid() -> None:
    """Health should return healthy status and pid."""
    response = health({})

    assert response["status"] == "healthy"
    assert response["pid"] == os.getpid()
