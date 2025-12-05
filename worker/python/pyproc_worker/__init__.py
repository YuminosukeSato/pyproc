"""pyproc_worker - Python worker for pyproc.

This module implements the Python side of the pyproc protocol,
allowing Python functions to be exposed and called from Go.
"""

from __future__ import annotations

import dataclasses
import inspect
import logging
import os
import socket
import struct
import sys
import traceback
from pathlib import Path
from typing import Callable, TypeVar, get_args, get_origin, get_type_hints

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
_exposed_functions: dict[str, Callable[..., object]] = {}
_ExposedFunc = TypeVar("_ExposedFunc", bound=Callable[..., object])

# Schema registry for exposed functions
_function_schemas: dict[str, dict[str, object]] = {}


def _handle_basic_type(py_type: type) -> dict[str, str] | None:
    """Handle basic Python types (int, float, str, bool, None).

    Args:
        py_type: Python type to check

    Returns:
        JSON schema dict for basic types, or None if not a basic type

    """
    if py_type is type(None):
        return {"type": "null"}
    if py_type is int:
        return {"type": "integer"}
    if py_type is float:
        return {"type": "number"}
    if py_type is str:
        return {"type": "string"}
    if py_type is bool:
        return {"type": "boolean"}
    return None


def _handle_generic_type(origin: type | None, args: tuple[type, ...]) -> dict[str, object] | None:
    """Handle generic types (list, dict, tuple, union).

    Args:
        origin: Origin type from get_origin()
        args: Type arguments from get_args()

    Returns:
        JSON schema dict for generic types, or None if not handled

    """
    if origin is list:
        if args:
            return {"type": "array", "items": _python_type_to_json_schema(args[0])}
        return {"type": "array"}

    if origin is dict:
        if len(args) >= 2:  # noqa: PLR2004
            return {
                "type": "object",
                "additionalProperties": _python_type_to_json_schema(args[1]),
            }
        return {"type": "object"}

    if origin is tuple:
        if args:
            return {
                "type": "array",
                "items": [_python_type_to_json_schema(arg) for arg in args],
            }
        return {"type": "array"}

    # Handle Union types (including Optional)
    if origin is type | type:  # Python 3.10+ union syntax
        types = [_python_type_to_json_schema(arg) for arg in args]
        return {"oneOf": types}

    return None


def _handle_class_type(py_type: type) -> dict[str, object] | None:
    """Handle dataclass types.

    Args:
        py_type: Python type to check

    Returns:
        JSON schema dict for dataclass types, or None if not a dataclass

    """
    if not dataclasses.is_dataclass(py_type):
        return None

    # Get dataclass fields
    fields = dataclasses.fields(py_type)

    # Build properties schema
    properties = {}
    required_fields = []

    for field in fields:
        field_schema = _python_type_to_json_schema(field.type)
        properties[field.name] = field_schema

        # Check if field is required (no default value)
        if field.default == dataclasses.MISSING and field.default_factory == dataclasses.MISSING:
            required_fields.append(field.name)

    # Check if dataclass is frozen (Python 3.10+ has __dataclass_params__)
    is_frozen = False
    if hasattr(py_type, "__dataclass_params__"):
        is_frozen = py_type.__dataclass_params__.frozen

    schema: dict[str, object] = {
        "type": "object",
        "properties": properties,
        "frozen": is_frozen,
    }

    if required_fields:
        schema["required"] = required_fields

    return schema


def _python_type_to_json_schema(py_type: type | object) -> dict[str, object]:
    """Convert Python type hint to JSON schema-like structure.

    Args:
        py_type: Python type hint to convert

    Returns:
        Dictionary representing the type in JSON schema format

    """
    # Handle basic types first
    if isinstance(py_type, type):
        basic_schema = _handle_basic_type(py_type)
        if basic_schema is not None:
            return basic_schema

        # Handle dataclass types (before generic types)
        class_schema = _handle_class_type(py_type)
        if class_schema is not None:
            return class_schema

    # Handle generic types
    origin = get_origin(py_type)
    args = get_args(py_type)
    generic_schema = _handle_generic_type(origin, args)
    if generic_schema is not None:
        return generic_schema

    # Fallback to "any" for unknown types
    return {"type": "any"}


def get_exposed_schemas() -> dict[str, object]:
    """Get all exposed function schemas.

    Returns:
        Dictionary containing schema version and function schemas

    """
    return {"schema_version": "1.0", "functions": _function_schemas}


def expose(func: _ExposedFunc) -> _ExposedFunc:
    """Expose a Python function to Go with schema capture.

    Usage:
        @expose
        def my_function(req):
            return {"result": req["input"] * 2}
    """
    func_name = getattr(func, "__name__", repr(func))
    _exposed_functions[func_name] = func

    # Capture type hints
    try:
        hints = get_type_hints(func)
        sig = inspect.signature(func)

        parameters = []
        for param_name, param in sig.parameters.items():
            # Skip internal parameters
            if param_name == "cancel_event":
                continue

            param_type = hints.get(param_name, object)
            parameters.append(
                {
                    "name": param_name,
                    "type": _python_type_to_json_schema(param_type),
                    "required": param.default == inspect.Parameter.empty,
                },
            )

        _function_schemas[func_name] = {
            "name": func_name,
            "parameters": parameters,
            "return_type": _python_type_to_json_schema(hints.get("return", object)),
            "docstring": inspect.getdoc(func) or "",
        }
    except Exception as e:
        logger.warning("Failed to capture schema for %s: %s", func_name, e)
        _function_schemas[func_name] = {"name": func_name, "schema_error": str(e)}

    logger.info("Exposed function: %s", func_name)
    return func


class FramedConnection:
    """Handles framed message communication over a socket."""

    def __init__(self, conn: socket.socket, codec: Codec | None = None) -> None:
        self.conn = conn
        self.codec = codec or get_codec("auto")

    def read_message(self) -> bytes | None:
        """Read a framed message from the socket."""
        # Read 4-byte length header
        length_bytes = self._read_exact(4)
        if not length_bytes:
            return None

        # Parse length (big-endian)
        length = struct.unpack(">I", length_bytes)[0]

        # Read message body
        message = self._read_exact(length)
        if not message:
            msg = "Failed to read complete message"
            raise RuntimeError(msg)

        return message

    def write_message(self, data: bytes) -> None:
        """Write a framed message to the socket."""
        # Write 4-byte length header (big-endian)
        length = len(data)
        self.conn.sendall(struct.pack(">I", length))

        # Write message body
        self.conn.sendall(data)

    def _read_exact(self, n: int) -> bytes | None:
        """Read exactly n bytes from the socket."""
        data = b""
        while len(data) < n:
            chunk = self.conn.recv(n - len(data))
            if not chunk:
                return None if len(data) == 0 else data
            data += chunk
        return data


class Worker:
    """Main worker class that handles requests from Go."""

    def __init__(self, socket_path: str, codec_type: str = "auto") -> None:
        self.socket_path = socket_path
        self.codec = get_codec(codec_type)
        self.conn = None
        self.framed_conn = None
        self.tracing = get_tracing()
        self.cancellation_manager = CancellationManager()
        logger.info("Using codec: %s", self.codec.name)

    def start(self) -> None:
        """Start the worker and listen for requests."""
        # Remove socket file if it exists
        socket_file = Path(self.socket_path)
        if socket_file.exists():
            socket_file.unlink()

        # Create Unix domain socket
        sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        sock.bind(self.socket_path)
        sock.listen(1)

        logger.info("Worker listening on %s", self.socket_path)

        while True:
            try:
                # Accept connection
                conn, _ = sock.accept()
                self.conn = conn
                self.framed_conn = FramedConnection(conn, self.codec)

                logger.info("Accepted connection")

                # Handle requests on this connection
                self._handle_connection()

            except KeyboardInterrupt:
                logger.info("Worker shutting down")
                break
            except Exception:
                logger.exception("Connection error")
                if self.conn:
                    self.conn.close()

    def _handle_connection(self) -> None:
        """Handle requests on the current connection."""
        current_request_id = None
        while True:
            try:
                # Read message
                message = self.framed_conn.read_message()
                if not message:
                    logger.info("Connection closed by client")
                    # If we have an active request, mark it as cancelled
                    if current_request_id is not None:
                        self.cancellation_manager.cancel_request(
                            current_request_id,
                            "connection closed",
                        )
                        current_request_id = None
                    break

                # Parse message
                msg_data = self.framed_conn.codec.decode(message)

                # Check if it's a wrapped message with type
                if isinstance(msg_data, dict) and "type" in msg_data:
                    msg_type = msg_data.get("type")
                    payload = msg_data.get("payload", {})

                    if msg_type == "cancellation":
                        # Handle cancellation message
                        self._handle_cancellation(payload)
                        continue  # No response needed for cancellation
                    if msg_type == "request":
                        request = payload
                    else:
                        logger.warning(f"Unknown message type: {msg_type}")
                        continue
                else:
                    # Legacy format - treat as request
                    request = msg_data

                logger.debug(f"Received request: {request}")

                # Track current request ID for cancellation
                current_request_id = request.get("id", 0)

                # Process request
                response = self._process_request(request)

                # Clear current request ID after processing
                current_request_id = None

                # Send response in legacy format for now
                # NOTE: Will switch to wrapped format once Go side is updated
                response_bytes = self.framed_conn.codec.encode(response)
                self.framed_conn.write_message(response_bytes)

            except BrokenPipeError:
                # Connection closed due to cancellation - this is expected behavior
                logger.debug(
                    "Connection closed by client during response (likely due to cancellation)",
                )
                break
            except Exception as e:
                logger.exception("Error handling request")
                # Try to send error response
                try:
                    error_response = {"id": 0, "ok": False, "error": str(e)}
                    response_bytes = self.framed_conn.codec.encode(error_response)
                    self.framed_conn.write_message(response_bytes)
                except Exception:  # noqa: S110
                    pass
                break

    def _process_request(self, request: dict[str, object]) -> dict[str, object]:
        """Process a single request and return a response."""
        req_id = request.get("id", 0)
        method = request.get("method", "")
        body = request.get("body", {})

        # Check if method is exposed
        if method not in _exposed_functions:
            return {"id": req_id, "ok": False, "error": f"Method '{method}' not found"}

        # Create tracing context for this request
        with (
            self.tracing.trace_request(request) as span,
            self.cancellation_manager.track_request(req_id) as cancel_event,
        ):
            try:
                # Call the exposed function
                func = _exposed_functions[method]

                # Check if function accepts cancel_event parameter
                sig = inspect.signature(func)
                if "cancel_event" in sig.parameters:
                    # Function supports cancellation
                    result = func(body, cancel_event=cancel_event)
                else:
                    # Function doesn't support cancellation, just call it
                    result = func(body)

                response = {"id": req_id, "ok": True, "body": result}

                # Add trace headers to response
                self.tracing.add_response_headers(response)

                return response

            except CancellationError as e:
                # Request was cancelled
                logger.info(f"Request {req_id} cancelled: {e.reason}")
                return {"id": req_id, "ok": False, "error": f"Cancelled: {e.reason}"}

            except Exception as e:
                # Capture the full traceback for debugging
                tb = traceback.format_exc()
                logger.exception("Error in method '%s': %s", method, tb)

                if span:
                    # Record exception in span
                    span.record_exception(e)

                return {"id": req_id, "ok": False, "error": str(e)}

    def _handle_cancellation(self, cancellation_msg: dict[str, object]) -> None:
        """Handle a cancellation message from Go.

        Args:
            cancellation_msg: Cancellation message with 'id' and 'reason' fields

        """
        req_id = cancellation_msg.get("id", 0)
        reason = cancellation_msg.get("reason", "context cancelled")

        logger.info(f"Received cancellation for request {req_id}: {reason}")

        # Cancel the request
        self.cancellation_manager.cancel_request(req_id, reason)


def run_worker(socket_path: str | None = None, codec_type: str = "auto") -> None:
    """Run the worker with the specified socket path.

    Args:
        socket_path: Path to the Unix domain socket.
                    If not provided, uses environment variable PYPROC_SOCKET_PATH.
        codec_type: Type of codec to use ("auto", "json", "orjson", "msgspec", "msgpack")
                   "auto" will choose the fastest available codec.

    """
    if socket_path is None:
        socket_path = os.environ.get("PYPROC_SOCKET_PATH")
        if not socket_path:
            msg = "Socket path must be provided or set in PYPROC_SOCKET_PATH"
            raise ValueError(msg)

    # Check for codec type from environment variable
    env_codec = os.environ.get("PYPROC_CODEC_TYPE")
    if env_codec:
        codec_type = env_codec

    worker = Worker(socket_path, codec_type)
    worker.start()


# Health check method (always available)
@expose
def health(_req):
    """Health check endpoint."""
    return {"status": "healthy", "pid": os.getpid()}
