package pyproc

import (
	"fmt"
	"io"
	"net/http"
)

// MetricsHandler returns an http.Handler that serves Prometheus text exposition
// format metrics from the given PoolWithMetrics.
// No external dependencies; hand-written text format.
func MetricsHandler(pool *PoolWithMetrics) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		snap := pool.GetMetrics()
		health := pool.Health()

		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

		writeMetrics(w, snap, health)
	})
}

func writeMetrics(w io.Writer, snap MetricsSnapshot, health HealthStatus) {
	// Request counters
	fmt.Fprintf(w, "# HELP pyproc_requests_total Total number of requests.\n")               //nolint:errcheck
	fmt.Fprintf(w, "# TYPE pyproc_requests_total counter\n")                                 //nolint:errcheck
	fmt.Fprintf(w, "pyproc_requests_total{status=\"success\"} %d\n", snap.RequestsSucceeded) //nolint:errcheck
	fmt.Fprintf(w, "pyproc_requests_total{status=\"failed\"} %d\n", snap.RequestsFailed)     //nolint:errcheck
	fmt.Fprintf(w, "pyproc_requests_total{status=\"timeout\"} %d\n", snap.RequestsTimeout)   //nolint:errcheck

	// Latency percentiles
	fmt.Fprintf(w, "# HELP pyproc_request_duration_seconds Request latency percentiles in seconds.\n")   //nolint:errcheck
	fmt.Fprintf(w, "# TYPE pyproc_request_duration_seconds gauge\n")                                     //nolint:errcheck
	fmt.Fprintf(w, "pyproc_request_duration_seconds{quantile=\"0.5\"} %f\n", snap.LatencyP50.Seconds())  //nolint:errcheck
	fmt.Fprintf(w, "pyproc_request_duration_seconds{quantile=\"0.95\"} %f\n", snap.LatencyP95.Seconds()) //nolint:errcheck
	fmt.Fprintf(w, "pyproc_request_duration_seconds{quantile=\"0.99\"} %f\n", snap.LatencyP99.Seconds()) //nolint:errcheck

	// Worker gauges
	fmt.Fprintf(w, "# HELP pyproc_workers_total Total number of workers.\n") //nolint:errcheck
	fmt.Fprintf(w, "# TYPE pyproc_workers_total gauge\n")                    //nolint:errcheck
	fmt.Fprintf(w, "pyproc_workers_total %d\n", health.TotalWorkers)         //nolint:errcheck

	fmt.Fprintf(w, "# HELP pyproc_workers_healthy Number of healthy workers.\n") //nolint:errcheck
	fmt.Fprintf(w, "# TYPE pyproc_workers_healthy gauge\n")                      //nolint:errcheck
	fmt.Fprintf(w, "pyproc_workers_healthy %d\n", health.HealthyWorkers)         //nolint:errcheck

	// Inflight
	fmt.Fprintf(w, "# HELP pyproc_inflight_requests Number of in-flight requests.\n") //nolint:errcheck
	fmt.Fprintf(w, "# TYPE pyproc_inflight_requests gauge\n")                         //nolint:errcheck
	fmt.Fprintf(w, "pyproc_inflight_requests %d\n", snap.QueueDepth)                  //nolint:errcheck

	// Worker restarts
	fmt.Fprintf(w, "# HELP pyproc_worker_restarts_total Total worker restarts.\n") //nolint:errcheck
	fmt.Fprintf(w, "# TYPE pyproc_worker_restarts_total counter\n")                //nolint:errcheck
	fmt.Fprintf(w, "pyproc_worker_restarts_total %d\n", snap.WorkerRestarts)       //nolint:errcheck
}
