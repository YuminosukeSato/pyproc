# JSON Codec Performance Guide

## Overview

PyProc supports multiple JSON codec implementations for optimal performance:

- **Standard Library** (`json-stdlib`): Default, most compatible
- **goccy/go-json** (`json-goccy`): High performance, drop-in replacement
- **segmentio/encoding** (`json-segmentio`): Fastest performance

## Build Tags

Select the JSON codec at compile time using build tags:

```bash
# Default (standard library)
go build ./...

# Use goccy/go-json
go build -tags=json_goccy ./...

# Use segmentio/encoding (recommended for production)
go build -tags=json_segmentio ./...
```

## Benchmark Results

On Apple M2 Max (arm64):

| Implementation | Marshal (ns/op) | Unmarshal (ns/op) | Improvement |
|---|---|---|---|
| Standard Library | 1493 | 2170 | Baseline |
| goccy/go-json | 1645 | 1962 | ~10% faster unmarshal |
| **segmentio/encoding** | **571** | **935** | **62% faster marshal, 57% faster unmarshal** |

## Runtime Detection

Check which codec is being used:

```go
import "github.com/YuminosukeSato/pyproc/pkg/pyproc"

func main() {
    codecType := pyproc.GetJSONCodecType()
    fmt.Printf("Using JSON codec: %s\n", codecType)
}
```

## Recommendations

- **Development**: Use default (standard library) for maximum compatibility
- **Production**: Use `segmentio/encoding` for best performance
- **Compatibility Issues**: Fall back to `goccy/go-json` or standard library

## Building for Production

```bash
# Build with fastest JSON codec
go build -tags=json_segmentio -o myapp ./cmd/myapp

# Verify codec in use
./myapp --version
```

## Performance Impact

With segmentio/encoding, the JSON codec overhead is reduced by ~60%, contributing to achieving our target latencies:
- p50 < 100µs (JSON codec ~0.5-1µs)
- p99 < 500µs (JSON codec ~1-2µs)