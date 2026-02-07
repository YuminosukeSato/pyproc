"""Tests for worker connection handling."""

from __future__ import annotations

import socket
from pathlib import Path
from typing import Any

from pyproc_worker import Worker


class DummyFramedConnection:
    """Minimal framed connection stub for testing."""

    def __init__(self, messages: list[bytes | None], codec) -> None:
        self._messages = iter(messages)
        self.codec = codec
        self.written: list[bytes] = []

    def read_message(self) -> bytes | None:
        try:
            return next(self._messages)
        except StopIteration:
            return None

    def write_message(self, data: bytes) -> None:
        self.written.append(data)


class DummyConn:
    """Minimal socket-like connection for Worker.start tests."""

    def __init__(self) -> None:
        self.closed = False

    def close(self) -> None:
        self.closed = True


class FakeSocket:
    """Socket stub that yields pre-defined accept results."""

    def __init__(self, accepts: list[object]) -> None:
        self._accepts = iter(accepts)
        self.bound = None
        self.listening = False

    def bind(self, path: str) -> None:
        self.bound = path

    def listen(self, _backlog: int) -> None:
        self.listening = True

    def accept(self):
        result = next(self._accepts)
        if isinstance(result, BaseException):
            raise result
        return result


def test_handle_connection_processes_wrapped_and_legacy(monkeypatch, tmp_path) -> None:
    """_handle_connection should handle wrapped, legacy, and cancellation messages."""
    worker = Worker(str(tmp_path / "worker.sock"), codec_type="json")
    codec = worker.codec

    messages = [
        codec.encode({"type": "cancellation", "payload": {"id": 10, "reason": "stop"}}),
        codec.encode({"type": "unknown", "payload": {}}),
        codec.encode({"type": "request", "payload": {"id": 1, "method": "ping", "body": {"x": 1}}}),
        codec.encode({"id": 2, "method": "ping", "body": {"x": 2}}),
        None,
    ]
    worker.framed_conn = DummyFramedConnection(messages, codec)

    def fake_process(req: dict[str, Any]) -> dict[str, Any]:
        return {"id": req["id"], "ok": True, "body": req["body"]}

    cancels: dict[str, Any] = {}

    def fake_cancel(request_id: int, reason: str) -> bool:
        cancels["id"] = request_id
        cancels["reason"] = reason
        return True

    monkeypatch.setattr(worker, "_process_request", fake_process)
    monkeypatch.setattr(worker.cancellation_manager, "cancel_request", fake_cancel)

    worker._handle_connection()  # noqa: SLF001

    assert cancels == {"id": 10, "reason": "stop"}
    expected_writes = 2
    assert len(worker.framed_conn.written) == expected_writes

    responses = [codec.decode(data) for data in worker.framed_conn.written]
    assert responses[0] == {"id": 1, "ok": True, "body": {"x": 1}}
    assert responses[1] == {"id": 2, "ok": True, "body": {"x": 2}}


def test_handle_connection_sends_error_response_on_exception(monkeypatch, tmp_path) -> None:
    """_handle_connection should send error response when processing fails."""
    worker = Worker(str(tmp_path / "worker.sock"), codec_type="json")
    codec = worker.codec

    messages = [
        codec.encode({"type": "request", "payload": {"id": 5, "method": "boom", "body": {}}}),
        None,
    ]
    worker.framed_conn = DummyFramedConnection(messages, codec)

    def fake_process(_req: dict[str, Any]) -> dict[str, Any]:
        raise RuntimeError("explode")

    monkeypatch.setattr(worker, "_process_request", fake_process)

    worker._handle_connection()  # noqa: SLF001

    assert len(worker.framed_conn.written) == 1
    response = codec.decode(worker.framed_conn.written[0])
    assert response == {"id": 0, "ok": False, "error": "explode"}


def test_handle_connection_suppresses_error_response_failures(monkeypatch, tmp_path) -> None:
    """Failure while sending error response should be suppressed."""
    worker = Worker(str(tmp_path / "worker.sock"), codec_type="json")

    class FailingCodec:
        def decode(self, _data: bytes):
            return {"id": 1, "method": "boom", "body": {}}

        def encode(self, _data: dict[str, Any]) -> bytes:
            raise RuntimeError("encode failed")

    class FailingFramedConnection(DummyFramedConnection):
        def __init__(self, messages):
            super().__init__(messages, FailingCodec())

    worker.framed_conn = FailingFramedConnection([b"msg"])

    def fake_process(_req: dict[str, Any]) -> dict[str, Any]:
        raise RuntimeError("explode")

    monkeypatch.setattr(worker, "_process_request", fake_process)

    worker._handle_connection()  # noqa: SLF001


def test_handle_connection_breaks_on_broken_pipe(monkeypatch, tmp_path) -> None:
    """BrokenPipeError should stop the connection loop cleanly."""
    worker = Worker(str(tmp_path / "worker.sock"), codec_type="json")
    codec = worker.codec

    class BrokenPipeFramedConnection(DummyFramedConnection):
        def write_message(self, _data: bytes) -> None:
            raise BrokenPipeError

    messages = [codec.encode({"id": 1, "method": "ping", "body": {}})]
    worker.framed_conn = BrokenPipeFramedConnection(messages, codec)

    monkeypatch.setattr(worker, "_process_request", lambda _req: {"id": 1, "ok": True, "body": {}})

    worker._handle_connection()  # noqa: SLF001


def test_handle_connection_cancels_active_request_on_close(monkeypatch, tmp_path) -> None:
    """Connection close should cancel any active request ID."""
    worker = Worker(str(tmp_path / "worker.sock"), codec_type="json")
    codec = worker.codec
    worker.framed_conn = DummyFramedConnection([None], codec)

    worker._current_request_id = 99  # noqa: SLF001
    cancelled = {}

    def fake_cancel(request_id: int, reason: str) -> bool:
        cancelled["id"] = request_id
        cancelled["reason"] = reason
        return True

    monkeypatch.setattr(worker.cancellation_manager, "cancel_request", fake_cancel)

    worker._handle_connection()  # noqa: SLF001

    assert cancelled == {"id": 99, "reason": "connection closed"}
    assert worker._current_request_id is None  # noqa: SLF001


def test_worker_start_accepts_and_exits_on_keyboard_interrupt(tmp_path, monkeypatch) -> None:
    """Worker.start should accept, handle once, then exit on KeyboardInterrupt."""
    socket_path = tmp_path / "worker.sock"
    socket_path.write_text("")

    dummy_conn = DummyConn()
    fake_socket = FakeSocket([(dummy_conn, None), KeyboardInterrupt()])

    monkeypatch.setattr(socket, "socket", lambda *_args, **_kwargs: fake_socket)
    worker = Worker(str(socket_path), codec_type="json")

    handled = {"value": False}

    def fake_handle() -> None:
        handled["value"] = True

    monkeypatch.setattr(worker, "_handle_connection", fake_handle)

    worker.start()

    assert handled["value"] is True
    assert worker.conn is dummy_conn
    assert worker.framed_conn is not None
    assert fake_socket.bound == str(socket_path)


def test_worker_start_closes_conn_on_exception(tmp_path, monkeypatch) -> None:
    """Worker.start should close the connection if handling fails."""
    socket_path = tmp_path / "worker.sock"
    Path(socket_path).write_text("")

    dummy_conn = DummyConn()
    fake_socket = FakeSocket([(dummy_conn, None), KeyboardInterrupt()])

    monkeypatch.setattr(socket, "socket", lambda *_args, **_kwargs: fake_socket)
    worker = Worker(str(socket_path), codec_type="json")

    def fake_handle() -> None:
        raise RuntimeError("boom")

    monkeypatch.setattr(worker, "_handle_connection", fake_handle)

    worker.start()

    assert dummy_conn.closed is True
