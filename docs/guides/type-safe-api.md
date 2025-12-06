---
title: Type-Safe API Guide - pyproc
description: Learn how to use CallTyped with Go generics for compile-time type safety
keywords: go, python, type safety, generics, calltyped
---

# Type-Safe API Guide

pyproc provides a **type-safe API** using Go generics, giving you compile-time type checking when calling Python functions.

## Why Type Safety Matters

### Without Type Safety (Legacy `Call` API)

```go
var output map[string]interface{}
err := pool.Call(ctx, "predict", input, &output)

// Runtime type assertions required
result := output["result"].(float64)  // Panic if wrong type!
model := output["model"].(string)     // Runtime error risk
```

**Problems**:
- ❌ No compile-time validation
- ❌ Runtime panics if types mismatch
- ❌ No IDE autocomplete
- ❌ Difficult to refactor

### With Type Safety (`CallTyped` API)

```go
result, err := pyproc.CallTyped[PredictRequest, PredictResponse](
    ctx, pool, "predict", PredictRequest{Value: 42},
)

// Type-safe field access
fmt.Println(result.Result)  // Compile-time type checking!
fmt.Println(result.Model)   // IDE autocomplete works!
```

**Benefits**:
- ✅ Compile-time type checking
- ✅ IDE autocomplete and navigation
- ✅ Safe refactoring
- ✅ Self-documenting code
- ✅ **Zero runtime overhead** (pure generics, no reflection)

## Basic Usage

### 1. Define Request and Response Types

```go
type PredictRequest struct {
    Value     float64   `json:"value"`
    Features  []float64 `json:"features,omitempty"`
    ModelName string    `json:"model_name,omitempty"`
}

type PredictResponse struct {
    Result     float64 `json:"result"`
    Confidence float64 `json:"confidence"`
    Model      string  `json:"model"`
}
```

### 2. Call with Type Safety

```go
result, err := pyproc.CallTyped[PredictRequest, PredictResponse](
    ctx,
    pool,
    "predict",  // Python function name
    PredictRequest{
        Value:     42.0,
        Features:  []float64{1.0, 2.0, 3.0},
        ModelName: "xgboost",
    },
)
if err != nil {
    return err
}

// Type-safe access
fmt.Printf("Prediction: %.2f (confidence: %.2f%%)\n",
    result.Result, result.Confidence * 100)
```

### 3. Python Worker Implementation

```python
from pyproc_worker import expose

@expose
def predict(req):
    """
    Args:
        req: Dict matching Go PredictRequest
            {
                "value": float,
                "features": List[float],
                "model_name": str
            }

    Returns:
        Dict matching Go PredictResponse
            {
                "result": float,
                "confidence": float,
                "model": str
            }
    """
    value = req["value"]
    features = req.get("features", [])
    model_name = req.get("model_name", "default")

    # Your ML logic here
    result = value * 2.0
    confidence = 0.95

    return {
        "result": result,
        "confidence": confidence,
        "model": model_name
    }
```

## Type Mapping Reference

### Go to Python Type Mapping

| Go Type | Python Type | JSON | Notes |
|---------|-------------|------|-------|
| `int`, `int64` | `int` | Number | - |
| `float32`, `float64` | `float` | Number | - |
| `string` | `str` | String | - |
| `bool` | `bool` | Boolean | - |
| `[]T` | `List[T]` | Array | - |
| `map[string]T` | `Dict[str, T]` | Object | - |
| `*T` (pointer) | `T \| None` | Null if nil | Use `omitempty` tag |
| `time.Time` | `str` | ISO 8601 string | Custom serialization |
| `struct{}` | `dict` | Object | Based on JSON tags |

### Struct Tags

```go
type Request struct {
    // Required field (no omitempty)
    UserID string `json:"user_id"`

    // Optional field (omitted if zero value)
    Email string `json:"email,omitempty"`

    // Field with custom JSON name
    FullName string `json:"full_name"`

    // Field ignored by JSON
    Internal string `json:"-"`
}
```

## Advanced Patterns

### Nested Structures

```go
type BatchPredictRequest struct {
    Samples []Sample `json:"samples"`
    Options Options  `json:"options,omitempty"`
}

type Sample struct {
    ID       string    `json:"id"`
    Features []float64 `json:"features"`
}

type Options struct {
    BatchSize int  `json:"batch_size,omitempty"`
    Async     bool `json:"async,omitempty"`
}

type BatchPredictResponse struct {
    Results []Result `json:"results"`
    Stats   Stats    `json:"stats"`
}

type Result struct {
    ID         string  `json:"id"`
    Prediction float64 `json:"prediction"`
    Confidence float64 `json:"confidence"`
}

type Stats struct {
    Total     int     `json:"total"`
    Succeeded int     `json:"succeeded"`
    Failed    int     `json:"failed"`
    AvgTime   float64 `json:"avg_time_ms"`
}
```

```go
result, err := pyproc.CallTyped[BatchPredictRequest, BatchPredictResponse](
    ctx, pool, "batch_predict",
    BatchPredictRequest{
        Samples: []Sample{
            {ID: "1", Features: []float64{1.0, 2.0}},
            {ID: "2", Features: []float64{3.0, 4.0}},
        },
        Options: Options{BatchSize: 32},
    },
)
```

```python
@expose
def batch_predict(req):
    samples = req["samples"]
    options = req.get("options", {})
    batch_size = options.get("batch_size", 10)

    results = []
    for sample in samples:
        # Process each sample
        prediction = sum(sample["features"]) * 2.0
        results.append({
            "id": sample["id"],
            "prediction": prediction,
            "confidence": 0.9
        })

    return {
        "results": results,
        "stats": {
            "total": len(samples),
            "succeeded": len(results),
            "failed": 0,
            "avg_time_ms": 1.5
        }
    }
```

### Handling Optional Fields

```go
type QueryRequest struct {
    Query     string   `json:"query"`
    Filters   *Filters `json:"filters,omitempty"`  // Pointer = optional
    Limit     *int     `json:"limit,omitempty"`    // Pointer = optional
}

type Filters struct {
    Category string   `json:"category,omitempty"`
    Tags     []string `json:"tags,omitempty"`
}
```

```python
@expose
def search(req):
    query = req["query"]

    # Optional fields with defaults
    filters = req.get("filters")
    limit = req.get("limit", 10)

    if filters:
        category = filters.get("category")
        tags = filters.get("tags", [])
        # Apply filters

    # ... search logic ...
    return {"results": []}
```

### Error Handling with Type Safety

```go
type Response struct {
    Data  *ResultData `json:"data,omitempty"`
    Error *ErrorInfo  `json:"error,omitempty"`
}

type ResultData struct {
    Value float64 `json:"value"`
}

type ErrorInfo struct {
    Code    string `json:"code"`
    Message string `json:"message"`
}
```

```go
result, err := pyproc.CallTyped[Request, Response](ctx, pool, "process", req)
if err != nil {
    // Transport/communication error
    return fmt.Errorf("call failed: %w", err)
}

// Check application-level error
if result.Error != nil {
    return fmt.Errorf("Python error [%s]: %s",
        result.Error.Code, result.Error.Message)
}

// Success - process data
fmt.Println(result.Data.Value)
```

## Migration from Legacy API

### Before (Non-Type-Safe)

```go
input := map[string]interface{}{
    "value": 42.0,
}

var output map[string]interface{}
err := pool.Call(ctx, "predict", input, &output)
if err != nil {
    return err
}

result := output["result"].(float64)  // Runtime type assertion
model := output["model"].(string)
```

### After (Type-Safe)

```go
result, err := pyproc.CallTyped[PredictRequest, PredictResponse](
    ctx, pool, "predict", PredictRequest{Value: 42.0},
)
if err != nil {
    return err
}

// Type-safe access, no assertions needed
fmt.Println(result.Result)
fmt.Println(result.Model)
```

## Best Practices

### 1. Use Descriptive Type Names

```go
// ❌ Bad: Generic names
type Input struct { ... }
type Output struct { ... }

// ✅ Good: Descriptive names
type PredictRequest struct { ... }
type PredictResponse struct { ... }
```

### 2. Document Expected Types

```go
// PredictRequest contains input data for ML model prediction.
type PredictRequest struct {
    // Value is the primary feature value (required)
    Value float64 `json:"value"`

    // Features contains additional feature vector (optional)
    Features []float64 `json:"features,omitempty"`
}
```

### 3. Keep Types in Sync

Create a shared package for request/response types:

```
myapp/
  pkg/
    pytypes/
      predict.go    # PredictRequest, PredictResponse
      batch.go      # BatchRequest, BatchResponse
  cmd/
    server/
      main.go       # Import pytypes
```

### 4. Validate Input Before Calling Python

```go
func callPredict(ctx context.Context, pool *pyproc.Pool, value float64) (*PredictResponse, error) {
    if value < 0 {
        return nil, errors.New("value must be non-negative")
    }

    return pyproc.CallTyped[PredictRequest, PredictResponse](
        ctx, pool, "predict", PredictRequest{Value: value},
    )
}
```

### 5. Use Pointers for Optional Complex Types

```go
type Request struct {
    Required   string   `json:"required"`           // Always present
    Optional   *Options `json:"options,omitempty"`  // May be nil
}
```

## Performance Notes

### Zero Runtime Overhead

`CallTyped` uses **pure Go generics** with no reflection:

- ✅ Same performance as `Call` API
- ✅ No reflection overhead
- ✅ Inline-able by compiler
- ✅ Type checking at compile time only

### Serialization Cost

Both APIs use the same JSON serialization:

```go
// Same performance
Call(ctx, pool, "predict", input, &output)           // ~5μs
CallTyped[Req, Resp](ctx, pool, "predict", req)      // ~5μs
```

The type safety is **free** in terms of runtime performance.

## Troubleshooting

### Type Mismatch Errors

**Symptom**: JSON unmarshal error

```
json: cannot unmarshal string into Go struct field .result of type float64
```

**Cause**: Python returns wrong type

**Solution**: Check Python worker output

```python
# ❌ Bad: Returns string instead of float
return {"result": "84.0"}

# ✅ Good: Returns float
return {"result": 84.0}
```

### Missing Field Errors

**Symptom**: Zero values in Go struct

```go
result.Model == ""  // Expected "xgboost"
```

**Cause**: Python doesn't return the field

**Solution**: Ensure Python returns all fields

```python
# ❌ Bad: Missing "model" field
return {"result": 84.0}

# ✅ Good: All fields present
return {"result": 84.0, "model": "xgboost", "confidence": 0.95}
```

### Pointer vs Value Confusion

**Symptom**: Optional field always nil

```go
type Request struct {
    Filters Filters `json:"filters,omitempty"`  // Not optional!
}
```

**Solution**: Use pointer for truly optional fields

```go
type Request struct {
    Filters *Filters `json:"filters,omitempty"`  // Optional
}
```

## Next Steps

- **[Error Handling](error-handling.md)**: Learn robust error handling patterns
- **[Performance Tuning](performance-tuning.md)**: Optimize for low latency
- **[Testing Guide](testing.md)**: Write tests for type-safe calls

---

## See Also

- [Go Generics Documentation](https://go.dev/doc/tutorial/generics)
- [JSON and Go](https://go.dev/blog/json)
- [pyproc API Reference](https://pkg.go.dev/github.com/YuminosukeSato/pyproc)
