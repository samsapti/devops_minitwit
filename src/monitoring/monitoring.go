package monitoring

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/shirou/gopsutil/cpu"
)

var (
	apiCPUGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "api_cpu_load",
		Help: "The CPU load percentage for the MiniTwit API",
	})

	appCPUGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "app_cpu_load",
		Help: "The CPU load percentage for the MiniTwit app",
	})

	apiRequestCount = promauto.NewCounter(prometheus.CounterOpts{
		Name: "api_request_count",
		Help: "The total number of processed HTTP requests by the MiniTwit API",
	})

	appRequestCount = promauto.NewCounter(prometheus.CounterOpts{
		Name: "app_request_count",
		Help: "The total number of processed HTTP requests by the MiniTwit app",
	})

	apiRequestDurationSummary = promauto.NewHistogram(prometheus.HistogramOpts{
		Name: "api_request_duration",
		Help: "Request duration distribution for HTTP requests to the MiniTwit API",
	})

	appRequestDurationSummary = promauto.NewHistogram(prometheus.HistogramOpts{
		Name: "app_request_duration",
		Help: "Request duration distribution for HTTP requests to the MiniTwit app",
	})
)

func MiddlewareMetrics(h http.Handler, isApi bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// BEFORE REQUEST
		start := time.Now()
		cpuUsage, err := cpu.Percent(0, false)

		if err == nil {
			if isApi {
				apiCPUGauge.Set(cpuUsage[0])
			} else {
				appCPUGauge.Set(cpuUsage[0])
			}
		}

		// REQUEST
		h.ServeHTTP(w, r)

		// AFTER REQUEST
		if isApi {
			apiRequestCount.Inc()
			apiRequestDurationSummary.Observe(float64(time.Since(start)))
		} else {
			appRequestCount.Inc()
			appRequestDurationSummary.Observe(float64(time.Since(start)))
		}
	})
}
