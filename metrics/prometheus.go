package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	totalRequestsCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "goreplay_total_requests",
			Help: "total income requests",
		},
		[]string{"location", "code"},
	)
	subRequestsCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "test_sub_requests",
			Help: "sub requests",
		},
		[]string{"test"},
	)

	buckets                    = []float64{0, 10, 30, 50, 100, 200}
	totalRequestsTimeHistogram = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "goreplay_total_requests_time",
			Help:    "incoming requests time",
			Buckets: buckets,
		},
		[]string{"location"},
	)
)

func init() {
	prometheus.MustRegister(totalRequestsCounter)
	prometheus.MustRegister(subRequestsCounter)
	prometheus.MustRegister(totalRequestsTimeHistogram)
}

func IncreaseTotalRequests(location, code string) {
	totalRequestsCounter.With(prometheus.Labels{"location": location, "code": code}).Add(1)
}

func IncreaseSubRequests() {
	subRequestsCounter.With(prometheus.Labels{"test": "test"}).Add(1)
}

func ObserveTotalRequestsTimeHistogram(location string, d float64) {
	totalRequestsTimeHistogram.With(prometheus.Labels{"location": location}).Observe(d)
}
