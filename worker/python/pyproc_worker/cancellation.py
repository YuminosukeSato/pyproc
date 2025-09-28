"""Cancellation management for pyproc worker.

This module provides cancellation support for long-running Python operations,
allowing them to be interrupted when the Go side cancels the context.
"""

from __future__ import annotations

import inspect
import logging
import threading
from contextlib import contextmanager
from typing import Any, Callable

logger = logging.getLogger(__name__)


class CancellationError(Exception):
    """Exception raised when an operation is cancelled."""

    def __init__(self, request_id: int, reason: str = "Operation cancelled") -> None:
        self.request_id = request_id
        self.reason = reason
        super().__init__(f"Request {request_id} cancelled: {reason}")


class CancellationManager:
    """Manages cancellation events for active requests."""

    def __init__(self) -> None:
        """Initialize the cancellation manager."""
        self._active_requests: dict[int, threading.Event] = {}
        self._lock = threading.RLock()
        self._cleanup_callbacks: dict[int, list[Callable]] = {}

    def register_request(self, request_id: int) -> threading.Event:
        """Register a new request and return its cancellation event.

        Args:
            request_id: Unique identifier for the request

        Returns:
            Threading event that will be set when the request is cancelled

        """
        with self._lock:
            if request_id in self._active_requests:
                logger.warning(f"Request {request_id} already registered, replacing")

            cancel_event = threading.Event()
            self._active_requests[request_id] = cancel_event
            self._cleanup_callbacks[request_id] = []

            logger.debug(f"Registered request {request_id}")
            return cancel_event

    def unregister_request(self, request_id: int) -> None:
        """Unregister a completed or cancelled request.

        Args:
            request_id: Unique identifier for the request

        """
        with self._lock:
            if request_id in self._active_requests:
                del self._active_requests[request_id]

                # Run and clear cleanup callbacks
                if request_id in self._cleanup_callbacks:
                    callbacks = self._cleanup_callbacks.pop(request_id)
                    for callback in callbacks:
                        try:
                            callback()
                        except Exception as e:
                            logger.error(f"Cleanup callback failed for request {request_id}: {e}")

                logger.debug(f"Unregistered request {request_id}")

    def cancel_request(self, request_id: int, reason: str = "context cancelled") -> bool:
        """Cancel a specific request.

        Args:
            request_id: Unique identifier for the request
            reason: Reason for cancellation

        Returns:
            True if the request was found and cancelled, False otherwise

        """
        with self._lock:
            if request_id in self._active_requests:
                cancel_event = self._active_requests[request_id]
                if not cancel_event.is_set():
                    cancel_event.set()
                    logger.info(f"Cancelled request {request_id}: {reason}")
                    return True
                logger.debug(f"Request {request_id} already cancelled")
                return False
            logger.warning(f"Cannot cancel unknown request {request_id}")
            return False

    def is_cancelled(self, request_id: int) -> bool:
        """Check if a request has been cancelled.

        Args:
            request_id: Unique identifier for the request

        Returns:
            True if the request is cancelled, False otherwise

        """
        with self._lock:
            if request_id in self._active_requests:
                return self._active_requests[request_id].is_set()
            return False

    def add_cleanup_callback(self, request_id: int, callback: Callable) -> None:
        """Add a cleanup callback to be called when the request is unregistered.

        Args:
            request_id: Unique identifier for the request
            callback: Function to call during cleanup

        """
        with self._lock:
            if request_id in self._cleanup_callbacks:
                self._cleanup_callbacks[request_id].append(callback)

    @contextmanager
    def track_request(self, request_id: int):
        """Context manager for tracking a cancellable request.

        Args:
            request_id: Unique identifier for the request

        Yields:
            Threading event that will be set when the request is cancelled

        Raises:
            CancellationError: If the request is cancelled during execution

        """
        cancel_event = self.register_request(request_id)

        try:
            yield cancel_event

            # Check if cancelled at the end
            if cancel_event.is_set():
                raise CancellationError(request_id, "Request cancelled during execution")

        finally:
            self.unregister_request(request_id)

    def check_cancellation(self, request_id: int) -> None:
        """Check if a request has been cancelled and raise if so.

        Args:
            request_id: Unique identifier for the request

        Raises:
            CancellationError: If the request has been cancelled

        """
        if self.is_cancelled(request_id):
            raise CancellationError(request_id)


def make_cancellable(func: Callable[..., Any]) -> Callable[[dict[str, Any], threading.Event], Any]:
    """Decorator to make a function cancellable.

    The decorated function should accept a 'cancel_event' parameter
    and periodically check if it's set.

    Args:
        func: Function to make cancellable

    Returns:
        Wrapped function that handles cancellation

    """
    accepts_cancel_event = "cancel_event" in inspect.signature(func).parameters

    def wrapper(request: dict[str, Any], cancel_event: threading.Event) -> Any:
        """Wrapper that adds cancellation checking."""
        # Pass the cancel_event to the function
        if accepts_cancel_event:
            return func(request, cancel_event=cancel_event)
        # Function doesn't support cancellation, just call it
        return func(request)

    func_name = getattr(func, "__name__", wrapper.__name__)
    wrapper.__name__ = func_name
    wrapper.__doc__ = getattr(func, "__doc__", wrapper.__doc__)
    return wrapper


class CancellableOperation:
    """Helper class for long-running cancellable operations."""

    def __init__(self, cancel_event: threading.Event, check_interval: int = 100) -> None:
        """Initialize a cancellable operation.

        Args:
            cancel_event: Event that signals cancellation
            check_interval: How often to check for cancellation (iterations)

        """
        self.cancel_event = cancel_event
        self.check_interval = check_interval
        self.iteration = 0

    def check(self) -> None:
        """Check if the operation should be cancelled.

        This should be called periodically during long-running operations.

        Raises:
            CancellationError: If the operation has been cancelled

        """
        self.iteration += 1
        if self.iteration % self.check_interval == 0 and self.cancel_event.is_set():
            raise CancellationError(0, "Operation cancelled")

    def __enter__(self):
        """Enter the cancellable context."""
        return self

    def __exit__(self, exc_type, exc_val, exc_tb):
        """Exit the cancellable context."""
        # Check one final time on exit
        if self.cancel_event.is_set() and exc_type is None:
            raise CancellationError(0, "Operation cancelled at exit")
        return False
