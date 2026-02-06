package pyproc

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// LivenessHandler returns an http.Handler for Kubernetes liveness probes.
// It always returns HTTP 200 with {"status":"ok"} as long as the process is alive.
func LivenessHandler(_ *Pool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"ok"}`) //nolint:errcheck
	})
}

// ReadinessHandler returns an http.Handler for Kubernetes readiness probes.
// It returns HTTP 200 when at least one worker is healthy and the pool is not
// shutting down; otherwise it returns HTTP 503.
func ReadinessHandler(pool *Pool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		health := pool.Health()
		if health.HealthyWorkers > 0 && !pool.shutdown.Load() {
			w.WriteHeader(http.StatusOK)
			resp, _ := json.Marshal(map[string]interface{}{
				"status":          "ready",
				"healthy_workers": health.HealthyWorkers,
				"total_workers":   health.TotalWorkers,
			})
			w.Write(resp) //nolint:errcheck
			return
		}

		w.WriteHeader(http.StatusServiceUnavailable)
		resp, _ := json.Marshal(map[string]interface{}{
			"status":          "not_ready",
			"healthy_workers": health.HealthyWorkers,
			"total_workers":   health.TotalWorkers,
		})
		w.Write(resp) //nolint:errcheck
	})
}

// StartupHandler returns an http.Handler for Kubernetes startup probes.
// It returns HTTP 200 once the pool has completed its Start() sequence;
// otherwise it returns HTTP 503.
func StartupHandler(pool *Pool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if pool.started.Load() {
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"status":"started"}`) //nolint:errcheck
			return
		}

		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprintf(w, `{"status":"not_started"}`) //nolint:errcheck
	})
}
