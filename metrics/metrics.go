// Package metrics 提供基于 Prometheus 的指标采集，包括：
//   - HTTP 请求量 / 延迟分布 (Gin 中间件)
//   - /metrics 端点 (promhttp)
//   - DB / Redis 连接池状态采集器
//   - GORM / Redis / Kafka / RabbitMQ 操作指标
//
// 使用方式：在 main.go 启动早期依次调用：
//
//	metrics.SetNamespace("my_service")
//	metrics.MustRegister()         // 必须在所有指标使用之前调用
//
// 所有指标采用懒初始化——调用 MustRegister 前访问指标变量会 panic（nil pointer），
// 这是有意为之，确保注册发生在使用之前。
package metrics

import (
	"database/sql"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Namespace Prometheus 指标命名空间前缀。各服务应在 MustRegister 之前调用 SetNamespace。
var Namespace = "default"

// SetNamespace 设置 Prometheus 指标命名空间。必须在 MustRegister 之前调用。
func SetNamespace(ns string) { Namespace = ns }

// Init 设置命名空间并注册所有指标。可安全重复调用（sync.Once 保证单次注册）。
// admin.New 内部已调用，业务入口也可显式调用（如 Worker 无 admin server 场景）。
func Init(ns string) {
	SetNamespace(ns)
	MustRegister()
}

// ── HTTP 指标定义（懒初始化）──────────────────────

var (
	HTTPRequestCounter  *prometheus.CounterVec
	HTTPRequestDuration *prometheus.HistogramVec
)

var registerOnce sync.Once

func registerHTTP() {
	HTTPRequestCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: Namespace,
			Subsystem: "http",
			Name:      "requests_total",
			Help:      "Total number of HTTP requests.",
		},
		[]string{"method", "path", "status"},
	)
	HTTPRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: Namespace,
			Subsystem: "http",
			Name:      "request_duration_seconds",
			Help:      "HTTP request duration in seconds.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)
	prometheus.MustRegister(HTTPRequestCounter)
	prometheus.MustRegister(HTTPRequestDuration)
}

// ── MustRegister ───────────────────────────────────

// MustRegister 显式注册所有指标到默认 Prometheus Registry。
// 必须在 main.go 启动早期调用（所有指标使用之前），调用前可先 SetNamespace。
// 多次调用安全（sync.Once 保证只执行一次）。
func MustRegister() {
	registerOnce.Do(func() {
		registerHTTP()
		registerGORM()
		registerRedis()
		registerRMQ()
		registerKafka()
	})
}

// ── Gin 中间件 ─────────────────────────────────────

// GinMiddleware 返回记录 HTTP 请求指标的 Gin 中间件。
// 使用前必须先调用 MustRegister，否则访问 HTTPRequestCounter 会 panic。
func GinMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path

		c.Next()

		status := strconv.Itoa(c.Writer.Status())
		duration := time.Since(start).Seconds()

		HTTPRequestCounter.WithLabelValues(c.Request.Method, path, status).Inc()
		HTTPRequestDuration.WithLabelValues(c.Request.Method, path).Observe(duration)
	}
}

// Handler 返回 /metrics 端点处理函数，暴露 Prometheus 格式指标。
func Handler() gin.HandlerFunc {
	h := promhttp.Handler()
	return func(c *gin.Context) {
		h.ServeHTTP(c.Writer, c.Request)
	}
}

// ── DB 连接池指标采集器 ────────────────────────────

type dbStatsCollector struct {
	db               *sql.DB
	openConns        prometheus.Gauge
	inUseConns       prometheus.Gauge
	idleConns        prometheus.Gauge
	waitCount        prometheus.Gauge
	waitDuration     prometheus.Gauge
	maxIdleClosed    prometheus.Gauge
	maxLifetimeClosed prometheus.Gauge
}

// RegisterDBStats 注册 DB 连接池指标采集器，按 name 区分数据源。
func RegisterDBStats(db *sql.DB, name string) {
	c := &dbStatsCollector{
		db: db,
		openConns: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace:   Namespace,
			Subsystem:   "database",
			Name:        "open_connections",
			Help:        "Current number of open connections.",
			ConstLabels: prometheus.Labels{"source": name},
		}),
		inUseConns: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace:   Namespace,
			Subsystem:   "database",
			Name:        "in_use_connections",
			Help:        "Current number of connections in use.",
			ConstLabels: prometheus.Labels{"source": name},
		}),
		idleConns: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace:   Namespace,
			Subsystem:   "database",
			Name:        "idle_connections",
			Help:        "Current number of idle connections.",
			ConstLabels: prometheus.Labels{"source": name},
		}),
		waitCount: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace:   Namespace,
			Subsystem:   "database",
			Name:        "wait_count_total",
			Help:        "Total number of connections waited for.",
			ConstLabels: prometheus.Labels{"source": name},
		}),
		waitDuration: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace:   Namespace,
			Subsystem:   "database",
			Name:        "wait_duration_seconds_total",
			Help:        "Total time blocked waiting for a new connection (seconds).",
			ConstLabels: prometheus.Labels{"source": name},
		}),
		maxIdleClosed: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace:   Namespace,
			Subsystem:   "database",
			Name:        "max_idle_closed_total",
			Help:        "Total number of connections closed due to SetMaxIdleConns.",
			ConstLabels: prometheus.Labels{"source": name},
		}),
		maxLifetimeClosed: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace:   Namespace,
			Subsystem:   "database",
			Name:        "max_lifetime_closed_total",
			Help:        "Total number of connections closed due to SetConnMaxLifetime.",
			ConstLabels: prometheus.Labels{"source": name},
		}),
	}
	prometheus.MustRegister(c)
}

func (c *dbStatsCollector) Describe(ch chan<- *prometheus.Desc) {
	c.openConns.Describe(ch)
	c.inUseConns.Describe(ch)
	c.idleConns.Describe(ch)
	c.waitCount.Describe(ch)
	c.waitDuration.Describe(ch)
	c.maxIdleClosed.Describe(ch)
	c.maxLifetimeClosed.Describe(ch)
}

func (c *dbStatsCollector) Collect(ch chan<- prometheus.Metric) {
	stats := c.db.Stats()
	c.openConns.Set(float64(stats.OpenConnections))
	c.inUseConns.Set(float64(stats.InUse))
	c.idleConns.Set(float64(stats.Idle))
	c.waitCount.Set(float64(stats.WaitCount))
	c.waitDuration.Set(stats.WaitDuration.Seconds())
	c.maxIdleClosed.Set(float64(stats.MaxIdleClosed))
	c.maxLifetimeClosed.Set(float64(stats.MaxLifetimeClosed))

	c.openConns.Collect(ch)
	c.inUseConns.Collect(ch)
	c.idleConns.Collect(ch)
	c.waitCount.Collect(ch)
	c.waitDuration.Collect(ch)
	c.maxIdleClosed.Collect(ch)
	c.maxLifetimeClosed.Collect(ch)
}
