# pyproc-worker

Python worker implementation for [pyproc](https://github.com/YuminosukeSato/pyproc) - Call Python from Go without CGO or microservices.

## Installation

```bash
pip install pyproc-worker
```

## Quick Start

Create a Python worker with your functions:

```python
from pyproc_worker import expose, run_worker

@expose
def predict(req):
    """Your ML model or Python logic here"""
    return {"result": req["value"] * 2}

@expose
def process_data(req):
    """Process data with Python libraries"""
    import pandas as pd
    df = pd.DataFrame(req["data"])
    return df.describe().to_dict()

if __name__ == "__main__":
    run_worker()
```

Then call it from Go using pyproc:

```go
pool, _ := pyproc.NewPool(pyproc.PoolOptions{
    Config: pyproc.PoolConfig{
        Workers:              4,
        MaxInFlight:          10,
        MaxInFlightPerWorker: 1,
    },
    WorkerConfig: pyproc.WorkerConfig{
        SocketPath:   "/tmp/pyproc.sock",
        PythonExec:   "python3",
        WorkerScript: "worker.py",
    },
}, nil)

pool.Start(ctx)
defer pool.Shutdown(ctx)

// Call Python function
input := map[string]interface{}{"value": 42}
var output map[string]interface{}
pool.Call(ctx, "predict", input, &output)
```

## Features

- **Simple decorator-based API** - Just use `@expose` to make functions callable
- **Automatic serialization** - Handles JSON serialization/deserialization
- **Built-in health checks** - Health endpoint automatically exposed
- **Graceful shutdown** - Proper cleanup on exit
- **Logging support** - Structured logging with configurable levels

## API Reference

### `@expose` Decorator

Makes a Python function callable from Go:

```python
@expose
def my_function(req):
    # req is a dict containing the request data
    # Return a dict that will be sent back to Go
    return {"result": "success"}
```

### `run_worker(socket_path=None)`

Starts the worker and listens for requests:

```python
if __name__ == "__main__":
    # Socket path from environment or command line
    run_worker()
    
    # Or specify explicitly
    run_worker("/tmp/my-worker.sock")
```

## Environment Variables

- `PYPROC_SOCKET_PATH` - Unix domain socket path
- `PYPROC_LOG_LEVEL` - Logging level (debug, info, warning, error)

## Examples

### Machine Learning Model

```python
import pickle
from pyproc_worker import expose, run_worker

# Load model at startup
with open("model.pkl", "rb") as f:
    model = pickle.load(f)

@expose
def predict(req):
    features = req["features"]
    prediction = model.predict([features])[0]
    
    return {
        "prediction": int(prediction),
        "confidence": float(model.predict_proba([features])[0].max())
    }

if __name__ == "__main__":
    run_worker()
```

### Data Processing

```python
import pandas as pd
from pyproc_worker import expose, run_worker

@expose
def analyze_csv(req):
    df = pd.DataFrame(req["data"])
    
    return {
        "mean": df.mean().to_dict(),
        "std": df.std().to_dict(),
        "correlation": df.corr().to_dict()
    }

if __name__ == "__main__":
    run_worker()
```

### Async Operations

```python
import asyncio
from pyproc_worker import expose, run_worker

@expose
async def fetch_data(req):
    url = req["url"]
    # Async operations work automatically
    async with aiohttp.ClientSession() as session:
        async with session.get(url) as response:
            data = await response.json()
    
    return {"data": data}

if __name__ == "__main__":
    run_worker()
```

## Development

### Running Tests

```bash
# Install dev dependencies
pip install -e .[dev]

# Run tests
pytest
```

### Building from Source

```bash
git clone https://github.com/YuminosukeSato/pyproc
cd pyproc/worker/python
pip install -e .
```

## License

Apache 2.0 - See [LICENSE](https://github.com/YuminosukeSato/pyproc/blob/main/LICENSE) for details.

## Links

- [pyproc Go Library](https://github.com/YuminosukeSato/pyproc)
- [Documentation](https://github.com/YuminosukeSato/pyproc#readme)
- [Examples](https://github.com/YuminosukeSato/pyproc/tree/main/examples)
- [Issue Tracker](https://github.com/YuminosukeSato/pyproc/issues)
