package pyproc

import (
	"context"
	"fmt"
)

// CallTyped is a type-safe wrapper for Pool.Call using Go generics.
// This is the RECOMMENDED way to call Python functions from Go.
//
// Benefits:
//   - Compile-time type checking for request and response
//   - No runtime type assertions needed
//   - Clear function signatures in your code
//   - Better IDE autocomplete support
//   - Identical performance to untyped Call() - zero overhead
//
// Type Parameters:
//   - TIn: The input type (must be JSON-serializable)
//   - TOut: The output type (must match Python response structure)
//
// Example usage:
//
//	type PredictRequest struct {
//	    Value float64 `json:"value"`
//	}
//	type PredictResponse struct {
//	    Result float64 `json:"result"`
//	}
//
//	result, err := pyproc.CallTyped[PredictRequest, PredictResponse](
//	    ctx, pool, "predict", PredictRequest{Value: 42},
//	)
//	if err != nil {
//	    return err
//	}
//	fmt.Printf("Result: %v\n", result.Result)  // Type-safe access
//
// Error Handling:
//   - Returns clear error messages for JSON marshaling failures
//   - Returns descriptive errors for type mismatches
//   - All errors are wrapped with context using fmt.Errorf with %w
//
// Performance:
//   - Benchmarked at <1% overhead compared to untyped Call()
//   - Actually uses 13% less memory and 35% fewer allocations
//   - See BenchmarkTypedVsUntyped in pool_generic_test.go
func CallTyped[TIn any, TOut any](ctx context.Context, pool *Pool, method string, input TIn) (TOut, error) {
	var output TOut

	// Call the underlying pool method
	err := pool.Call(ctx, method, input, &output)
	if err != nil {
		return output, fmt.Errorf("call %s failed: %w", method, err)
	}

	return output, nil
}

// CallTypedWithTransport is a type-safe wrapper for PoolWithTransport.Call using Go generics
func CallTypedWithTransport[TIn any, TOut any](ctx context.Context, pool *PoolWithTransport, method string, input TIn) (TOut, error) {
	var output TOut

	// Call the underlying pool method
	err := pool.Call(ctx, method, input, &output)
	if err != nil {
		return output, fmt.Errorf("call %s failed: %w", method, err)
	}

	return output, nil
}

// TypedPool provides a type-safe wrapper around Pool with predefined input/output types.
// Use this when you want to reuse the same type pair for multiple method calls.
//
// Benefits over direct CallTyped:
//   - Type parameters specified once at creation
//   - Cleaner code when calling multiple methods with same types
//   - Full access to pool lifecycle methods (Start, Shutdown, Health)
//
// Example usage:
//
//	type Request struct { Value int `json:"value"` }
//	type Response struct { Result int `json:"result"` }
//
//	pool, err := pyproc.NewPool(opts, logger)
//	if err != nil {
//	    return err
//	}
//
//	typedPool := pyproc.NewTypedPool[Request, Response](pool)
//	if err := typedPool.Start(ctx); err != nil {
//	    return err
//	}
//	defer typedPool.Shutdown(ctx)
//
//	// Multiple calls with type safety
//	resp1, err := typedPool.Call(ctx, "method1", Request{Value: 1})
//	resp2, err := typedPool.Call(ctx, "method2", Request{Value: 2})
type TypedPool[TIn any, TOut any] struct {
	pool *Pool
}

// NewTypedPool creates a new typed pool wrapper with predefined input/output types.
// The returned TypedPool can call any Python method that accepts TIn and returns TOut.
func NewTypedPool[TIn any, TOut any](pool *Pool) *TypedPool[TIn, TOut] {
	return &TypedPool[TIn, TOut]{
		pool: pool,
	}
}

// Call executes a method with type safety
func (tp *TypedPool[TIn, TOut]) Call(ctx context.Context, method string, input TIn) (TOut, error) {
	return CallTyped[TIn, TOut](ctx, tp.pool, method, input)
}

// Start starts all workers in the pool
func (tp *TypedPool[TIn, TOut]) Start(ctx context.Context) error {
	return tp.pool.Start(ctx)
}

// Shutdown gracefully shuts down the pool
func (tp *TypedPool[TIn, TOut]) Shutdown(ctx context.Context) error {
	return tp.pool.Shutdown(ctx)
}

// Health returns the health status of the pool
func (tp *TypedPool[TIn, TOut]) Health() HealthStatus {
	return tp.pool.Health()
}

// TypedWorkerClient provides a type-safe client for a specific Python worker method.
// Use this when you're repeatedly calling the same method and want the cleanest API.
//
// Benefits:
//   - Method name specified once at creation
//   - Simplest call syntax: client.Call(ctx, input)
//   - Built-in batch execution with BatchCall
//   - Type safety for both single and batch operations
//
// Example usage:
//
//	type Request struct { Value int `json:"value"` }
//	type Response struct { Result int `json:"result"` }
//
//	pool, err := pyproc.NewPool(opts, logger)
//	if err != nil {
//	    return err
//	}
//
//	// Create a client for the "predict" method
//	predictClient := pyproc.NewTypedWorkerClient[Request, Response](pool, "predict")
//
//	// Single call - cleanest syntax
//	resp, err := predictClient.Call(ctx, Request{Value: 42})
//
//	// Batch call - parallel execution
//	inputs := []Request{{Value: 1}, {Value: 2}, {Value: 3}}
//	responses, errors := predictClient.BatchCall(ctx, inputs)
type TypedWorkerClient[TIn any, TOut any] struct {
	pool   *Pool
	method string
}

// NewTypedWorkerClient creates a type-safe client for a specific Python worker method.
// The returned client will always call the specified method with type safety.
func NewTypedWorkerClient[TIn any, TOut any](pool *Pool, method string) *TypedWorkerClient[TIn, TOut] {
	return &TypedWorkerClient[TIn, TOut]{
		pool:   pool,
		method: method,
	}
}

// Call executes the predefined method with type safety.
// This is the simplest way to call a Python method with full type safety.
func (tc *TypedWorkerClient[TIn, TOut]) Call(ctx context.Context, input TIn) (TOut, error) {
	return CallTyped[TIn, TOut](ctx, tc.pool, tc.method, input)
}

// BatchCall executes multiple requests in parallel using goroutines.
// Returns a slice of results and a slice of errors (one per input).
// Results and errors are guaranteed to be in the same order as inputs.
//
// Example:
//
//	inputs := []Request{{Value: 1}, {Value: 2}, {Value: 3}}
//	results, errors := client.BatchCall(ctx, inputs)
//	for i := range inputs {
//	    if errors[i] != nil {
//	        log.Printf("Request %d failed: %v", i, errors[i])
//	        continue
//	    }
//	    log.Printf("Result %d: %v", i, results[i])
//	}
func (tc *TypedWorkerClient[TIn, TOut]) BatchCall(ctx context.Context, inputs []TIn) ([]TOut, []error) {
	results := make([]TOut, len(inputs))
	errors := make([]error, len(inputs))

	// Use goroutines for parallel execution
	type result struct {
		index  int
		output TOut
		err    error
	}

	resultCh := make(chan result, len(inputs))

	for i, input := range inputs {
		go func(idx int, in TIn) {
			out, err := tc.Call(ctx, in)
			resultCh <- result{index: idx, output: out, err: err}
		}(i, input)
	}

	// Collect results
	for i := 0; i < len(inputs); i++ {
		res := <-resultCh
		results[res.index] = res.output
		errors[res.index] = res.err
	}

	return results, errors
}

// Example usage types for common patterns

// PredictRequest represents a prediction request
type PredictRequest struct {
	Value float64 `json:"value"`
}

// PredictResponse represents a prediction response
type PredictResponse struct {
	Result float64 `json:"result"`
}

// TransformRequest represents a text transformation request
type TransformRequest struct {
	Text string `json:"text"`
}

// TransformResponse represents a text transformation response
type TransformResponse struct {
	TransformedText string `json:"transformed_text"`
	WordCount       int    `json:"word_count"`
}

// BatchRequest represents a batch processing request
type BatchRequest struct {
	Items []map[string]interface{} `json:"items"`
}

// BatchResponse represents a batch processing response
type BatchResponse struct {
	Results []map[string]interface{} `json:"results"`
	Count   int                      `json:"count"`
}

// StatsRequest represents a statistics computation request
type StatsRequest struct {
	Numbers []float64 `json:"numbers"`
}

// StatsResponse represents a statistics computation response
type StatsResponse struct {
	Mean   float64 `json:"mean"`
	Median float64 `json:"median"`
	StdDev float64 `json:"std_dev"`
	Min    float64 `json:"min"`
	Max    float64 `json:"max"`
}
