"""Tests for cancellation management."""

from __future__ import annotations

import logging
import threading

import pytest

from pyproc_worker.cancellation import (
    CancellableOperation,
    CancellationError,
    CancellationManager,
    make_cancellable,
)


def test_register_unregister_runs_cleanup_callbacks() -> None:
    """Ensure cleanup callbacks run on unregister."""
    manager = CancellationManager()
    cancel_event = manager.register_request(1)
    assert isinstance(cancel_event, threading.Event)
    assert manager.is_cancelled(1) is False

    called = {"value": False}

    def cleanup() -> None:
        called["value"] = True

    manager.add_cleanup_callback(1, cleanup)
    manager.unregister_request(1)

    assert called["value"] is True
    assert manager.is_cancelled(1) is False


def test_register_twice_logs_warning(caplog) -> None:
    """Registering twice should log a warning."""
    manager = CancellationManager()
    manager.register_request(10)

    with caplog.at_level(logging.WARNING):
        manager.register_request(10)

    assert "already registered" in caplog.text


def test_cleanup_callback_failure_is_logged(caplog) -> None:
    """Failing cleanup callbacks should be logged as errors."""
    manager = CancellationManager()
    manager.register_request(20)

    def bad_cleanup() -> None:
        raise RuntimeError("cleanup failed")

    manager.add_cleanup_callback(20, bad_cleanup)
    with caplog.at_level(logging.ERROR):
        manager.unregister_request(20)

    assert "Cleanup callback failed" in caplog.text


def test_cancel_request_sets_event_and_is_idempotent() -> None:
    """Cancel should set event once and be idempotent."""
    manager = CancellationManager()
    manager.register_request(2)

    assert manager.cancel_request(2, "stop") is True
    assert manager.is_cancelled(2) is True
    assert manager.cancel_request(2, "stop-again") is False
    assert manager.cancel_request(999, "unknown") is False


def test_track_request_raises_if_cancelled_before_exit() -> None:
    """track_request should raise if cancelled during context."""
    manager = CancellationManager()

    with (
        pytest.raises(CancellationError, match="Request cancelled during execution"),
        manager.track_request(3) as cancel_event,
    ):
        cancel_event.set()


def test_check_cancellation_raises_for_cancelled_request() -> None:
    """check_cancellation should raise for cancelled requests."""
    manager = CancellationManager()
    manager.register_request(4)

    manager.cancel_request(4, "reason")
    with pytest.raises(CancellationError):
        manager.check_cancellation(4)

    manager.unregister_request(4)
    manager.check_cancellation(4)


def test_make_cancellable_passes_cancel_event_when_supported() -> None:
    """make_cancellable should forward cancel_event when accepted."""
    called = {"event": None}

    def func(_req, cancel_event: threading.Event) -> str:
        called["event"] = cancel_event
        return "ok"

    wrapper = make_cancellable(func)
    event = threading.Event()

    assert wrapper({"k": "v"}, event) == "ok"
    assert called["event"] is event


def test_make_cancellable_ignores_cancel_event_when_not_supported() -> None:
    """make_cancellable should ignore cancel_event if func doesn't accept it."""
    called = {"value": None}

    def func(req) -> str:
        called["value"] = req
        return "done"

    wrapper = make_cancellable(func)
    event = threading.Event()

    assert wrapper({"x": 1}, event) == "done"
    assert called["value"] == {"x": 1}


def test_cancellable_operation_check_and_exit() -> None:
    """CancellableOperation should raise on cancellation."""
    event = threading.Event()
    operation = CancellableOperation(event, check_interval=1)

    operation.check()
    event.set()

    with pytest.raises(CancellationError, match="Operation cancelled"):
        operation.check()

    event.clear()
    with pytest.raises(CancellationError, match="Operation cancelled at exit"), operation:
        event.set()

    with (
        pytest.raises(
            CancellationError,
            match="Operation cancelled at exit",
        ),
        CancellableOperation(event, check_interval=2),
    ):
        event.set()

    event.clear()
    with CancellableOperation(event, check_interval=2):
        pass
