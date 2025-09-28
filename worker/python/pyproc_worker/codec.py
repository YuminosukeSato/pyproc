"""Codec implementations for pyproc worker."""

from __future__ import annotations

import json
from abc import ABC, abstractmethod
from typing import Any, cast

try:
    import orjson

    HAS_ORJSON = True
except ImportError:
    HAS_ORJSON = False

msgspec_json_module: Any = None
msgspec_msgpack_module: Any = None

try:
    from msgspec import json as _msgspec_json_module
    from msgspec import msgpack as _msgspec_msgpack_module
except ImportError:
    HAS_MSGSPEC = False
    msgspec_json_module = None
    msgspec_msgpack_module = None
else:
    HAS_MSGSPEC = True
    msgspec_json_module = cast("Any", _msgspec_json_module)
    msgspec_msgpack_module = cast("Any", _msgspec_msgpack_module)


class Codec(ABC):
    """Abstract base class for codecs."""

    @abstractmethod
    def encode(self, obj: Any) -> bytes:
        """Encode an object to bytes."""

    @abstractmethod
    def decode(self, data: bytes) -> Any:
        """Decode bytes to an object."""

    @property
    @abstractmethod
    def name(self) -> str:
        """Return the codec name."""


class JSONCodec(Codec):
    """Standard library JSON codec."""

    def encode(self, obj: Any) -> bytes:
        return json.dumps(obj).encode("utf-8")

    def decode(self, data: bytes) -> Any:
        return json.loads(data.decode("utf-8"))

    @property
    def name(self) -> str:
        return "json-stdlib"


class OrjsonCodec(Codec):
    """orjson-based JSON codec (faster)."""

    def __init__(self) -> None:
        if not HAS_ORJSON:
            msg = "orjson is not installed"
            raise ImportError(msg)

    def encode(self, obj: Any) -> bytes:
        return orjson.dumps(obj)

    def decode(self, data: bytes) -> Any:
        return orjson.loads(data)

    @property
    def name(self) -> str:
        return "json-orjson"


class MsgspecCodec(Codec):
    """msgspec-based codec (fastest, with type validation)."""

    def __init__(self) -> None:
        if not HAS_MSGSPEC or msgspec_json_module is None:
            msg = "msgspec is not installed. Install with: uv add msgspec"
            raise ImportError(msg)
        self.encoder = msgspec_json_module.Encoder()
        self.decoder = msgspec_json_module.Decoder()

    def encode(self, obj: Any) -> bytes:
        return self.encoder.encode(obj)

    def decode(self, data: bytes) -> Any:
        return self.decoder.decode(data)

    @property
    def name(self) -> str:
        return "msgspec"


class MsgpackCodec(Codec):
    """MessagePack codec using msgspec."""

    def __init__(self) -> None:
        if not HAS_MSGSPEC or msgspec_msgpack_module is None:
            msg = "msgspec is not installed. Install with: uv add msgspec"
            raise ImportError(msg)
        self.encoder = msgspec_msgpack_module.Encoder()
        self.decoder = msgspec_msgpack_module.Decoder()

    def encode(self, obj: Any) -> bytes:
        return self.encoder.encode(obj)

    def decode(self, data: bytes) -> Any:
        return self.decoder.decode(data)

    @property
    def name(self) -> str:
        return "msgpack"


def get_codec(codec_type: str = "auto") -> Codec:
    """Get a codec instance by type.

    Args:
        codec_type: One of "auto", "json", "orjson", "msgspec", "msgpack"
                   "auto" will choose the fastest available codec

    Returns:
        A Codec instance

    """
    if codec_type == "auto":
        # Try to use the fastest available codec
        if HAS_MSGSPEC:
            return MsgspecCodec()
        if HAS_ORJSON:
            return OrjsonCodec()
        return JSONCodec()
    if codec_type == "json":
        return JSONCodec()
    if codec_type == "orjson":
        return OrjsonCodec()
    if codec_type == "msgspec":
        return MsgspecCodec()
    if codec_type == "msgpack":
        return MsgpackCodec()
    msg = f"Unknown codec type: {codec_type}"
    raise ValueError(msg)


# Default codec - use orjson if available, fallback to stdlib
default_codec = OrjsonCodec() if HAS_ORJSON else JSONCodec()
