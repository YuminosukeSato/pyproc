#!/usr/bin/env python3
"""
IO-Bound example worker demonstrating concurrent request handling.

This example simulates IO-bound operations like HTTP requests or database queries
by using time.sleep() to represent waiting for external resources.
"""

import os
import sys
import time
import random
from datetime import datetime
import threading

# Add parent directory to path to import pyproc_worker
sys.path.insert(0, os.path.join(os.path.dirname(__file__), "../../worker/python"))

from pyproc_worker import expose, run_worker


@expose
def fetch_remote_data(request):
    """Simulate fetching data from a remote API."""
    url = request.get("url", "http://example.com/api")
    delay = request.get("delay", 1.0)
    
    # Log thread info to show concurrent execution
    thread_id = threading.get_ident()
    print(f"[Thread {thread_id}] Starting fetch from {url} at {datetime.now().isoformat()}")
    
    # Simulate network delay
    time.sleep(delay)
    
    # Simulate response data
    data = {
        "url": url,
        "status": 200,
        "data": {
            "items": random.randint(10, 100),
            "timestamp": datetime.now().isoformat()
        },
        "thread_id": thread_id,
        "duration": delay
    }
    
    print(f"[Thread {thread_id}] Completed fetch from {url} at {datetime.now().isoformat()}")
    return data


@expose
def batch_fetch(request):
    """Simulate fetching data from multiple endpoints."""
    urls = request.get("urls", ["http://api1.com", "http://api2.com", "http://api3.com"])
    delay_per_url = request.get("delay", 0.5)
    
    thread_id = threading.get_ident()
    print(f"[Thread {thread_id}] Starting batch fetch of {len(urls)} URLs")
    
    results = []
    for url in urls:
        # Simulate fetching each URL
        time.sleep(delay_per_url)
        results.append({
            "url": url,
            "status": 200,
            "data": {"count": random.randint(1, 50)}
        })
    
    return {
        "total_urls": len(urls),
        "results": results,
        "thread_id": thread_id,
        "total_duration": delay_per_url * len(urls)
    }


@expose
def database_query(request):
    """Simulate a database query operation."""
    query = request.get("query", "SELECT * FROM users")
    delay = request.get("delay", 0.3)
    
    thread_id = threading.get_ident()
    print(f"[Thread {thread_id}] Executing query: {query}")
    
    # Simulate query execution
    time.sleep(delay)
    
    # Simulate query results
    rows = random.randint(5, 20)
    return {
        "query": query,
        "rows_affected": rows,
        "execution_time": delay,
        "thread_id": thread_id
    }


@expose
def heavy_io_operation(request):
    """Simulate a heavy IO operation with multiple steps."""
    steps = request.get("steps", 3)
    delay_per_step = request.get("delay", 0.5)
    
    thread_id = threading.get_ident()
    print(f"[Thread {thread_id}] Starting heavy IO operation with {steps} steps")
    
    results = []
    for i in range(steps):
        print(f"[Thread {thread_id}] Processing step {i+1}/{steps}")
        time.sleep(delay_per_step)
        results.append({
            "step": i + 1,
            "status": "completed",
            "data": random.randint(100, 1000)
        })
    
    return {
        "steps_completed": steps,
        "results": results,
        "total_duration": steps * delay_per_step,
        "thread_id": thread_id
    }


if __name__ == "__main__":
    # Get socket path from environment or command line
    socket_path = os.environ.get("PYPROC_SOCKET_PATH")
    if not socket_path and len(sys.argv) > 1:
        socket_path = sys.argv[1]
    
    # Get concurrency from environment (set by Go side)
    concurrency = int(os.environ.get("PYPROC_WORKER_CONCURRENCY", "4"))
    
    print(f"Starting IO-bound worker with concurrency={concurrency}")
    print("This worker demonstrates handling multiple IO-bound requests concurrently")
    print("Each request simulates network/database latency using time.sleep()")
    
    # Run worker with specified concurrency
    run_worker(socket_path, concurrency=concurrency)