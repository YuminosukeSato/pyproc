"""OpenTelemetry tracing support for pyproc worker."""

from __future__ import annotations

import logging
import os
from contextlib import contextmanager
from typing import Any

logger = logging.getLogger(__name__)

# Try to import OpenTelemetry
try:
    from opentelemetry import trace
    from opentelemetry.propagate import extract, inject
    from opentelemetry.sdk.resources import Resource
    from opentelemetry.sdk.trace import TracerProvider
    from opentelemetry.sdk.trace.export import BatchSpanProcessor, ConsoleSpanExporter
    from opentelemetry.trace import Status, StatusCode

    HAS_OTEL = True
except ImportError:
    HAS_OTEL = False
    logger.debug("OpenTelemetry not installed, tracing disabled")


class TracingManager:
    """Manages OpenTelemetry tracing for pyproc worker."""

    def __init__(self, enabled: bool = True, service_name: str = "pyproc-worker") -> None:
        """Initialize the tracing manager.

        Args:
            enabled: Whether tracing is enabled
            service_name: Name of the service for tracing

        """
        self.enabled = enabled and HAS_OTEL
        self.service_name = service_name
        self.tracer = None

        if self.enabled:
            self._setup_tracing()

    def _setup_tracing(self) -> None:
        """Set up OpenTelemetry tracing."""
        # Create a resource with service name
        resource = Resource.create(
            {
                "service.name": self.service_name,
                "service.version": "0.1.0",
                "process.pid": os.getpid(),
            },
        )

        # Create and set the tracer provider
        provider = TracerProvider(resource=resource)

        # Add console exporter for development
        # In production, you'd add OTLP exporter or other backends
        if os.environ.get("PYPROC_TRACE_CONSOLE") == "true":
            processor = BatchSpanProcessor(ConsoleSpanExporter())
            provider.add_span_processor(processor)

        trace.set_tracer_provider(provider)
        self.tracer = trace.get_tracer(__name__)
        logger.info("OpenTelemetry tracing initialized for %s", self.service_name)

    @contextmanager
    def span(
        self,
        name: str,
        attributes: dict[str, Any] | None = None,
        context: dict[str, Any] | None = None,
    ):
        """Create a tracing span.

        Args:
            name: Name of the span
            attributes: Span attributes
            context: Context to extract trace parent from

        Yields:
            The span object or None if tracing is disabled

        """
        if not self.enabled or not self.tracer:
            yield None
            return

        # Extract trace context from incoming request if provided
        ctx = None
        if context and "traceparent" in context:
            # Extract W3C trace context
            carrier = {"traceparent": context["traceparent"]}
            if "tracestate" in context:
                carrier["tracestate"] = context["tracestate"]
            ctx = extract(carrier)

        # Start span with context
        with self.tracer.start_as_current_span(
            name,
            context=ctx,
            attributes=attributes,
        ) as span:
            try:
                yield span
            except Exception as e:
                if span:
                    span.set_status(Status(StatusCode.ERROR, str(e)))
                    span.record_exception(e)
                raise

    def inject_context(self, carrier: dict[str, Any]) -> None:
        """Inject current trace context into a carrier.

        Args:
            carrier: Dictionary to inject trace context into

        """
        if self.enabled:
            inject(carrier)

    def extract_context(self, carrier: dict[str, Any]) -> Any | None:
        """Extract trace context from a carrier.

        Args:
            carrier: Dictionary containing trace context

        Returns:
            Extracted context or None

        """
        if self.enabled:
            return extract(carrier)
        return None


class WorkerTracing:
    """Tracing integration for pyproc worker."""

    def __init__(self, worker_id: str | None = None) -> None:
        """Initialize worker tracing.

        Args:
            worker_id: Optional worker ID for identification

        """
        self.worker_id = worker_id or f"worker-{os.getpid()}"
        self.manager = TracingManager(
            enabled=os.environ.get("PYPROC_TRACING_ENABLED", "false").lower() == "true",
            service_name=os.environ.get("PYPROC_SERVICE_NAME", "pyproc-worker"),
        )

    def trace_request(self, request: dict[str, Any]):
        """Create a tracing context for a request.

        Args:
            request: The incoming request

        Returns:
            A context manager for the request span

        """
        method = request.get("method", "unknown")
        req_id = request.get("id", 0)

        # Extract trace context from request headers
        headers = request.get("headers", {})

        attributes = {
            "rpc.method": method,
            "rpc.request_id": req_id,
            "worker.id": self.worker_id,
        }

        return self.manager.span(
            f"pyproc.{method}",
            attributes=attributes,
            context=headers,
        )

    def add_response_headers(self, response: dict[str, Any]) -> None:
        """Add trace context to response headers.

        Args:
            response: The response dictionary to add headers to

        """
        if self.manager.enabled:
            headers = response.setdefault("headers", {})
            self.manager.inject_context(headers)


# Global tracing instance (initialized lazily)
_global_tracing: WorkerTracing | None = None


def get_tracing() -> WorkerTracing:
    """Get the global tracing instance."""
    global _global_tracing
    if _global_tracing is None:
        _global_tracing = WorkerTracing()
    return _global_tracing


def extract_trace_context(request: dict[str, Any]) -> Any | None:
    """Extract W3C Trace Context from Go request.

    Args:
        request: Dictionary containing traceparent and tracestate keys

    Returns:
        OpenTelemetry Context object for span creation, or None if tracing disabled

    """
    if not HAS_OTEL:
        return None

    carrier = {
        "traceparent": request.get("traceparent", ""),
        "tracestate": request.get("tracestate", ""),
    }
    return extract(carrier)


def trace_method(func):
    """Trace a method execution.

    Args:
        func: The function to trace

    Returns:
        Wrapped function with tracing

    """

    def wrapper(request: dict[str, Any]) -> dict[str, Any]:
        tracing = get_tracing()
        with tracing.trace_request({"method": func.__name__, **request}):
            return func(request)

    wrapper.__name__ = func.__name__
    wrapper.__doc__ = func.__doc__
    return wrapper
