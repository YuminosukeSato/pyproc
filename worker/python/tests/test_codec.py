"""Test codec functionality."""

import importlib
import importlib.abc
import sys

import pytest

from pyproc_worker.codec import (
    HAS_MSGSPEC,
    HAS_ORJSON,
    JSONCodec,
    OrjsonCodec,
    get_codec,
)


def test_json_codec() -> None:
    """Test standard JSON codec."""
    codec = JSONCodec()
    assert codec.name == "json-stdlib"

    # Test encoding and decoding
    data = {"key": "value", "number": 42, "list": [1, 2, 3]}
    encoded = codec.encode(data)
    assert isinstance(encoded, bytes)

    decoded = codec.decode(encoded)
    assert decoded == data


def test_orjson_codec() -> None:
    """Test orjson codec if available."""
    if not HAS_ORJSON:
        pytest.skip("orjson not installed")

    codec = OrjsonCodec()
    assert codec.name == "json-orjson"

    # Test encoding and decoding
    data = {"key": "value", "number": 42, "list": [1, 2, 3]}
    encoded = codec.encode(data)
    assert isinstance(encoded, bytes)

    decoded = codec.decode(encoded)
    assert decoded == data


def test_get_codec_auto() -> None:
    """Test automatic codec selection."""
    codec = get_codec("auto")

    # Should prefer orjson or msgspec if available
    if HAS_MSGSPEC:
        assert codec.name == "msgspec"
    elif HAS_ORJSON:
        assert codec.name == "json-orjson"
    else:
        assert codec.name == "json-stdlib"


def test_get_codec_json() -> None:
    """Test explicit JSON codec selection."""
    codec = get_codec("json")
    assert codec.name == "json-stdlib"


def test_get_codec_orjson() -> None:
    """Test explicit orjson codec selection."""
    if not HAS_ORJSON:
        pytest.skip("orjson not installed")

    codec = get_codec("orjson")
    assert codec.name == "json-orjson"


def test_codec_roundtrip() -> None:
    """Test that all codecs can roundtrip various data types."""
    test_data = [
        {"simple": "string"},
        {"number": 123.456},
        {"list": [1, 2, 3, 4, 5]},
        {"nested": {"a": 1, "b": {"c": 2}}},
        {"unicode": "こんにちは世界"},
        {"special": "!@#$%^&*()"},
    ]

    codecs_to_test = ["json"]
    if HAS_ORJSON:
        codecs_to_test.append("orjson")

    for codec_type in codecs_to_test:
        codec = get_codec(codec_type)
        for data in test_data:
            encoded = codec.encode(data)
            decoded = codec.decode(encoded)
            assert decoded == data, f"Failed roundtrip for {codec_type} with data: {data}"


def test_invalid_codec_type() -> None:
    """Test that invalid codec type raises error."""
    with pytest.raises(ValueError, match="Unknown codec type"):
        get_codec("invalid")


class _ImportBlocker(importlib.abc.MetaPathFinder):
    """Block imports for optional codec dependencies."""

    def find_spec(self, fullname, _path, _target=None):
        if fullname in {"orjson", "msgspec"}:
            raise ImportError("blocked")


def test_codec_importerror_paths() -> None:
    """Optional codec imports should fail gracefully when blocked."""
    from pyproc_worker import codec

    blocker = _ImportBlocker()
    sys.meta_path.insert(0, blocker)
    sys.modules.pop("orjson", None)
    sys.modules.pop("msgspec", None)

    try:
        reloaded = importlib.reload(codec)
        assert reloaded.HAS_ORJSON is False
        assert reloaded.HAS_MSGSPEC is False
        assert isinstance(reloaded.get_codec("auto"), reloaded.JSONCodec)

        with pytest.raises(ImportError, match="orjson is not installed"):
            reloaded.OrjsonCodec()

        with pytest.raises(ImportError, match="msgspec is not installed"):
            reloaded.MsgspecCodec()

        with pytest.raises(ImportError, match="msgspec is not installed"):
            reloaded.MsgpackCodec()
    finally:
        sys.meta_path.remove(blocker)
        importlib.reload(codec)


def test_get_codec_auto_prefers_orjson_when_msgspec_disabled(monkeypatch) -> None:
    """Auto should select orjson when msgspec is disabled but orjson is available."""
    from pyproc_worker import codec

    if not codec.HAS_ORJSON:
        pytest.skip("orjson not installed")

    monkeypatch.setattr(codec, "HAS_MSGSPEC", False)
    monkeypatch.setattr(codec, "msgspec_json_module", None)

    selected = codec.get_codec("auto")
    assert selected.name == "json-orjson"
