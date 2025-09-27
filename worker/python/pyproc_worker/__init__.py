"""pyproc_worker - Python worker for pyproc.

This module implements the Python side of the pyproc protocol,
allowing Python functions to be exposed and called from Go.
"""

from __future__ import annotations

import inspect
import logging
import os
import socket
import struct
import sys
import threading
import traceback
from concurrent.futures import ThreadPoolExecutor
from contextlib import suppress
from pathlib import Path
from typing import Any, Callable

from .cancellation import CancellationError, CancellationManager
from .codec import Codec, get_codec
from .tracing import get_tracing

# Setup logging
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s - %(name)s - %(levelname)s - %(message)s",
    stream=sys.stderr,
)
logger = logging.getLogger(__name__)

# Registry for exposed functions
_exposed_functions: dict[str, Callable] = {}


def expose(func: Callable) -> Callable:
    """Expose a Python function to Go.

    Usage:
        @expose
        def my_function(req):
            return {"result": req["input"] * 2}
    """
    _exposed_functions[func.__name__] = func
    logger.info("Exposed function: %s", func.__name__)
    return func


class FramedConnection:
    """Handles framed message communication over a socket."""

    def __init__(self, conn: socket.socket, codec: Codec | None = None) -> None:
        self.conn = conn
        self.codec = codec or get_codec("auto")

    def read_message(self) -> bytes | None:
        """Read a framed message from the socket."""
        length_bytes = self._read_exact(4)
        if not length_bytes:
            return None

        length = struct.unpack(">I", length_bytes)[0]
        message = self._read_exact(length)
        if not message:
            msg = "Failed to read complete message"
            raise RuntimeError(msg)

        return message

    def write_message(self, data: bytes) -> None:
        """Write a framed message to the socket."""
        length = len(data)
        self.conn.sendall(struct.pack(">I", length))
        self.conn.sendall(data)

    def _read_exact(self, num_bytes: int) -> bytes | None:
        """Read exactly num_bytes from the socket."""
        data = b""
        while len(data) < num_bytes:
            chunk = self.conn.recv(num_bytes - len(data))
            if not chunk:
                return data if data else None
            data += chunk
        return data


class Worker:
    """Main worker class that handles requests from Go."""

    def __init__(self, socket_path: str, codec_type: str = "auto", concurrency: int = 1) -> None:
        self.socket_path = socket_path
        self.codec_type = codec_type
        codec_preview = get_codec(codec_type)
        self.codec_name = codec_preview.name
        self.tracing = get_tracing()
        self.cancellation_manager = CancellationManager()
        self.concurrency = max(1, concurrency)
        self._shutdown = threading.Event()
        self._connection_executor: ThreadPoolExecutor | None = None

        if self.concurrency > 1:
            self._connection_executor = ThreadPoolExecutor(
                max_workers=self.concurrency,
                thread_name_prefix="pyproc-worker",
            )
            logger.info("Using threaded mode with %d workers", self.concurrency)

        logger.info("Using codec: %s", self.codec_name)

    def start(self) -> None:
        """Start the worker and listen for requests."""
        socket_file = Path(self.socket_path)
        if socket_file.exists():
            socket_file.unlink()

        sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        sock.bind(self.socket_path)
        sock.listen(self.concurrency)
        sock.settimeout(1.0)

        logger.info("Worker listening on %s", self.socket_path)

        try:
            while not self._shutdown.is_set():
                try:
                    conn, _ = sock.accept()
                except socket.timeout:
                    continue
                except OSError as exc:
                    if self._shutdown.is_set():
                        break
                    logger.error("Socket accept error: %s", exc)
                    continue

                if self._connection_executor:
                    try:
                        self._connection_executor.submit(self._handle_connection, conn)
                    except RuntimeError:
                        with suppress(Exception):
                            conn.close()
                else:
                    self._handle_connection(conn)

        except KeyboardInterrupt:
            logger.info("Worker shutting down")
        finally:
            self._shutdown.set()
            with suppress(Exception):
                sock.close()
            if self._connection_executor:
                self._connection_executor.shutdown(wait=True)

    def _create_codec(self) -> Codec:
        """Create a new codec instance for the connection."""
        return get_codec(self.codec_type)

    def _handle_connection(self, conn: socket.socket) -> None:
        """Serve a single connection until it is closed."""
        codec = self._create_codec()
        framed_conn = FramedConnection(conn, codec)
        current_request_id: int | None = None

        try:
            while not self._shutdown.is_set():
                message = framed_conn.read_message()
                if not message:
                    logger.info("Connection closed by client")
                    if current_request_id is not None:
                        self.cancellation_manager.cancel_request(
                            current_request_id,
                            "connection closed",
                        )
                        current_request_id = None
                    break

                msg_data = framed_conn.codec.decode(message)

                request = self._extract_request(msg_data)
                if request is None:
                    continue

                logger.debug("Received request: %s", request)

                current_request_id = request.get("id", 0)
                response = self._process_request(request)
                current_request_id = None

                response_bytes = framed_conn.codec.encode(response)
                framed_conn.write_message(response_bytes)

        except BrokenPipeError:
            logger.debug(
                "Connection closed by client during response (likely due to cancellation)",
            )
        except Exception as exc:
            logger.exception("Error handling request")
            try:
                error_response = {"id": current_request_id or 0, "ok": False, "error": str(exc)}
                response_bytes = framed_conn.codec.encode(error_response)
                framed_conn.write_message(response_bytes)
            except Exception:
                logger.debug("Error sending error response to client")
        finally:
            with suppress(Exception):
                conn.close()

    def _extract_request(self, message: Any) -> dict[str, Any] | None:
        """Normalize incoming message payloads into request dictionaries."""
        if isinstance(message, dict) and "type" in message:
            msg_type = message.get("type")
            payload = message.get("payload", {})

            if msg_type == "cancellation":
                self._handle_cancellation(payload)
                return None

            if msg_type != "request":
                logger.warning("Unknown message type: %s", msg_type)
                return None

            if not isinstance(payload, dict):
                logger.warning("Invalid payload type: %s", type(payload))
                return None

            return payload

        if not isinstance(message, dict):
            logger.warning("Invalid request message: %s", type(message))
            return None

        return message

    def _process_request(self, request: dict[str, Any]) -> dict[str, Any]:
        """Process a single request and return a response."""
        req_id = request.get("id", 0)
        method = request.get("method", "")
        body = request.get("body", {})

        if method not in _exposed_functions:
            return {"id": req_id, "ok": False, "error": f"Method '{method}' not found"}

        with (
            self.tracing.trace_request(request) as span,
            self.cancellation_manager.track_request(req_id) as cancel_event,
        ):
            try:
                func = _exposed_functions[method]

                sig = inspect.signature(func)
                if "cancel_event" in sig.parameters:
                    result = func(body, cancel_event=cancel_event)
                else:
                    result = func(body)

                response = {"id": req_id, "ok": True, "body": result}
                self.tracing.add_response_headers(response)
                return response

            except CancellationError as exc:
                logger.info("Request %s cancelled: %s", req_id, exc.reason)
                return {"id": req_id, "ok": False, "error": f"Cancelled: {exc.reason}"}

            except Exception as exc:
                traceback_text = traceback.format_exc()
                logger.exception("Error in method '%s': %s", method, traceback_text)

                if span:
                    span.record_exception(exc)

                return {"id": req_id, "ok": False, "error": str(exc)}

    def _handle_cancellation(self, cancellation_msg: dict[str, Any]) -> None:
        """Handle a cancellation message from Go."""
        req_id = cancellation_msg.get("id", 0)
        reason = cancellation_msg.get("reason", "context cancelled")
        logger.info("Received cancellation for request %s: %s", req_id, reason)
        self.cancellation_manager.cancel_request(req_id, reason)


def run_worker(
    socket_path: str | None = None,
    codec_type: str = "auto",
    concurrency: int | None = None,
) -> None:
    """Run the worker with the specified socket path."""
    if socket_path is None:
        socket_path = os.environ.get("PYPROC_SOCKET_PATH")
        if not socket_path:
            msg = "Socket path must be provided or set in PYPROC_SOCKET_PATH"
            raise ValueError(msg)

    env_codec = os.environ.get("PYPROC_CODEC_TYPE")
    if env_codec:
        codec_type = env_codec

    if concurrency is None:
        env_concurrency = os.environ.get("PYPROC_WORKER_CONCURRENCY")
        if env_concurrency:
            try:
                concurrency = int(env_concurrency)
            except ValueError:
                logger.warning(
                    "Invalid PYPROC_WORKER_CONCURRENCY value: %s, using default 1",
                    env_concurrency,
                )
                concurrency = 1
        else:
            concurrency = 1

    if concurrency <= 0:
        logger.warning("Invalid concurrency value %s, using default 1", concurrency)
        concurrency = 1

    worker = Worker(socket_path, codec_type, concurrency)
    worker.start()


@expose
def health(_req: dict[str, Any]) -> dict[str, Any]:
    """Health check endpoint."""
    return {"status": "healthy", "pid": os.getpid()}
