"""Tests for FramedConnection behavior."""

from __future__ import annotations

import socket

import pytest

from pyproc_worker import FramedConnection
from pyproc_worker.codec import JSONCodec


def test_write_and_read_message_roundtrip() -> None:
    """FramedConnection should roundtrip framed messages."""
    left, right = socket.socketpair()
    try:
        left_conn = FramedConnection(left, JSONCodec())
        right_conn = FramedConnection(right, JSONCodec())

        payload = b"hello world"
        left_conn.write_message(payload)

        received = right_conn.read_message()
        assert received == payload
    finally:
        left.close()
        right.close()


def test_read_message_returns_none_on_eof() -> None:
    """read_message should return None when the socket is closed."""
    left, right = socket.socketpair()
    try:
        conn = FramedConnection(left, JSONCodec())
        right.close()
        assert conn.read_message() is None
    finally:
        left.close()


def test_read_message_raises_on_missing_body(monkeypatch) -> None:
    """read_message should raise when body cannot be read."""
    left, right = socket.socketpair()
    try:
        conn = FramedConnection(left, JSONCodec())

        calls = {"count": 0}

        def fake_read_exact(n: int):
            calls["count"] += 1
            if calls["count"] == 1:
                return b"\x00\x00\x00\x04"
            return b""

        monkeypatch.setattr(conn, "_read_exact", fake_read_exact)

        with pytest.raises(RuntimeError, match="Failed to read complete message"):
            conn.read_message()
    finally:
        left.close()
        right.close()
