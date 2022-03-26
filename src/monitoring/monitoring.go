package monitoring

import (
	"log"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/shirou/gopsutil/cpu"

	ctrl "minitwit/controllers"
)

var (
	cpuGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "cpu_load",
		Help: "The CPU load percentage",
	})

	apiRequestCount = promauto.NewCounter(prometheus.CounterOpts{
		Name: "api_request_count",
		Help: "The total number of processed HTTP requests by the MiniTwit API",
	})

	requestDurationSummary = promauto.NewHistogram(prometheus.HistogramOpts{
		Name: "api_request_duration",
		Help: "Request duration distribution for HTTP requests to the MiniTwit API",
	})
)

func MiddlewareMetrics(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// BEFORE REQUEST
		start := time.Now()
		cpuUsage, err := cpu.Percent(0, false)

		if !ctrl.CheckError(err) {
			cpuGauge.Set(float64(cpuUsage[0]))
		}

		log.Println()

		// REQUEST
		h.ServeHTTP(w, r)

		// AFTER REQUEST
		apiRequestCount.Inc()                                      // api_request_count
		requestDurationSummary.Observe(float64(time.Since(start))) // request_duration
	})
}
