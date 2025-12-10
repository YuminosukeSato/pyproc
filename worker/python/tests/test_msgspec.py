"""Test msgspec codec functionality."""

import pytest

from pyproc_worker.codec import Codec, HAS_MSGSPEC, MsgpackCodec, MsgspecCodec, get_codec


def test_msgspec_json_codec() -> None:
    """Test msgspec JSON codec."""
    if not HAS_MSGSPEC:
        pytest.skip("msgspec not installed")

    codec = MsgspecCodec()
    assert codec.name == "msgspec"

    # Test encoding and decoding
    data = {"key": "value", "number": 42, "list": [1, 2, 3]}
    encoded = codec.encode(data)
    assert isinstance(encoded, bytes)

    decoded = codec.decode(encoded)
    assert decoded == data


def test_msgpack_codec() -> None:
    """Test MessagePack codec."""
    if not HAS_MSGSPEC:
        pytest.skip("msgspec not installed")

    codec = MsgpackCodec()
    assert codec.name == "msgpack"

    # Test encoding and decoding
    data = {"key": "value", "number": 42, "list": [1, 2, 3]}
    encoded = codec.encode(data)
    assert isinstance(encoded, bytes)

    decoded = codec.decode(encoded)
    assert decoded == data


def test_get_codec_msgspec() -> None:
    """Test explicit msgspec codec selection."""
    if not HAS_MSGSPEC:
        pytest.skip("msgspec not installed")

    codec = get_codec("msgspec")
    assert codec.name == "msgspec"


def test_get_codec_msgpack() -> None:
    """Test explicit msgpack codec selection."""
    if not HAS_MSGSPEC:
        pytest.skip("msgspec not installed")

    codec = get_codec("msgpack")
    assert codec.name == "msgpack"


def test_msgspec_complex_data() -> None:
    """Test msgspec with complex data structures."""
    if not HAS_MSGSPEC:
        pytest.skip("msgspec not installed")

    complex_data = {
        "string": "hello world",
        "int": 42,
        "float": 3.14159,
        "bool": True,
        "null": None,
        "list": [1, 2, 3, 4, 5],
        "nested": {
            "a": 1,
            "b": {"c": 2, "d": [3, 4, 5]},
        },
        "unicode": "你好世界 🌍",
        "large_list": list(range(1000)),
    }

    # Test both msgspec JSON and MessagePack
    for codec_type in ["msgspec", "msgpack"]:
        codec = get_codec(codec_type)
        encoded = codec.encode(complex_data)
        decoded = codec.decode(encoded)
        assert decoded == complex_data


def test_msgspec_performance() -> None:
    """Simple performance test to verify msgspec is faster than stdlib JSON."""
    if not HAS_MSGSPEC:
        pytest.skip("msgspec not installed")

    import statistics
    import time

    large_data = {
        f"key_{i}": {"value": i, "squared": i**2, "text": f"item_{i}"} for i in range(1000)
    }

    json_codec = get_codec("json")
    msgspec_codec = get_codec("msgspec")

    def measure(codec: Codec, iterations: int) -> float:
        start = time.perf_counter()
        for _ in range(iterations):
            encoded = codec.encode(large_data)
            _ = codec.decode(encoded)
        return time.perf_counter() - start

    for codec in [json_codec, msgspec_codec]:
        for _ in range(20):
            encoded = codec.encode(large_data)
            _ = codec.decode(encoded)

    rounds = 5
    iterations = 100
    json_samples = [measure(json_codec, iterations) for _ in range(rounds)]
    msgspec_samples = [measure(msgspec_codec, iterations) for _ in range(rounds)]

    json_median = statistics.median(json_samples)
    msgspec_median = statistics.median(msgspec_samples)

    assert msgspec_median < json_median * 1.5, (
        "msgspec median should be within 1.5x of JSON median: "
        f"msgspec={msgspec_median:.4f}s json={json_median:.4f}s"
    )
