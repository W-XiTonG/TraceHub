package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	GatewayRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "tracehub_gateway_requests_total",
		Help: "网关请求总数",
	}, []string{"model", "channel_type", "status"})

	GatewayLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "tracehub_gateway_latency_seconds",
		Help:    "网关上游耗时",
		Buckets: prometheus.ExponentialBuckets(0.05, 2, 12),
	}, []string{"channel_type"})
)
