package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const rmqSubsystem = "rabbitmq"

var (
	rmqPublishTotal     *prometheus.CounterVec
	rmqPublishDuration  *prometheus.HistogramVec
	rmqPublishErrors    *prometheus.CounterVec
	rmqConsumeTotal     *prometheus.CounterVec
	rmqConsumeDuration  *prometheus.HistogramVec
	rmqConsumeErrors    *prometheus.CounterVec
)

func registerRMQ() {
	rmqPublishTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: Namespace, Subsystem: rmqSubsystem,
			Name: "publish_messages_total",
			Help: "Total number of published RMQ messages by queue.",
		},
		[]string{"queue"},
	)
	rmqPublishDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: Namespace, Subsystem: rmqSubsystem,
			Name:    "publish_duration_seconds",
			Help:    "RMQ publish duration in seconds.",
			Buckets: []float64{.0005, .001, .005, .01, .025, .05, .1, .25, .5, 1},
		},
		[]string{"queue"},
	)
	rmqPublishErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: Namespace, Subsystem: rmqSubsystem,
			Name: "publish_errors_total",
			Help: "Total number of failed RMQ publish calls.",
		},
		[]string{"queue"},
	)
	rmqConsumeTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: Namespace, Subsystem: rmqSubsystem,
			Name: "consume_messages_total",
			Help: "Total number of consumed RMQ messages by queue.",
		},
		[]string{"queue"},
	)
	rmqConsumeDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: Namespace, Subsystem: rmqSubsystem,
			Name:    "consume_duration_seconds",
			Help:    "RMQ message processing duration in seconds.",
			Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10, 30},
		},
		[]string{"queue"},
	)
	rmqConsumeErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: Namespace, Subsystem: rmqSubsystem,
			Name: "consume_errors_total",
			Help: "Total number of failed RMQ message processing.",
		},
		[]string{"queue"},
	)
	prometheus.MustRegister(
		rmqPublishTotal, rmqPublishDuration, rmqPublishErrors,
		rmqConsumeTotal, rmqConsumeDuration, rmqConsumeErrors,
	)
}

// ── Producer Observers ─────────────────────────────

// ObserveRMQPublish 记录 RMQ 消息发布的耗时和结果。
func ObserveRMQPublish(queue string, start time.Time, err error) {
	rmqPublishTotal.WithLabelValues(queue).Inc()
	rmqPublishDuration.WithLabelValues(queue).Observe(time.Since(start).Seconds())
	if err != nil {
		rmqPublishErrors.WithLabelValues(queue).Inc()
	}
}

// ── Consumer Observers ─────────────────────────────

// ObserveRMQConsume 记录消费一条 RMQ 消息的耗时和结果。
func ObserveRMQConsume(queue string, start time.Time, err error) {
	rmqConsumeTotal.WithLabelValues(queue).Inc()
	rmqConsumeDuration.WithLabelValues(queue).Observe(time.Since(start).Seconds())
	if err != nil {
		rmqConsumeErrors.WithLabelValues(queue).Inc()
	}
}
