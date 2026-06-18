package metrics

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	goredis "github.com/redis/go-redis/v9"
)

const redisSubsystem = "redis"

var (
	redisCommandsTotal    *prometheus.CounterVec
	redisCommandDuration  *prometheus.HistogramVec
	redisErrorsTotal      *prometheus.CounterVec
)

func registerRedis() {
	redisCommandsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: Namespace, Subsystem: redisSubsystem,
			Name: "commands_total",
			Help: "Total number of Redis commands by command name.",
		},
		[]string{"cmd"},
	)
	redisCommandDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: Namespace, Subsystem: redisSubsystem,
			Name:    "command_duration_seconds",
			Help:    "Redis command duration in seconds.",
			Buckets: []float64{.0001, .0005, .001, .005, .01, .025, .05, .1, .25, .5, 1},
		},
		[]string{"cmd"},
	)
	redisErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: Namespace, Subsystem: redisSubsystem,
			Name: "errors_total",
			Help: "Total number of failed Redis commands.",
		},
		[]string{"cmd"},
	)
	prometheus.MustRegister(redisCommandsTotal, redisCommandDuration, redisErrorsTotal)
}

// RedisHook 实现 redis.Hook 接口，自动采集 Redis 命令指标。
type RedisHook struct{}

func (h *RedisHook) DialHook(next goredis.DialHook) goredis.DialHook { return next }

func (h *RedisHook) ProcessHook(next goredis.ProcessHook) goredis.ProcessHook {
	return func(ctx context.Context, cmd goredis.Cmder) error {
		start := time.Now()
		name := cmd.Name()
		err := next(ctx, cmd)

		redisCommandsTotal.WithLabelValues(name).Inc()
		redisCommandDuration.WithLabelValues(name).Observe(time.Since(start).Seconds())
		if err != nil && err != goredis.Nil {
			redisErrorsTotal.WithLabelValues(name).Inc()
		}
		return err
	}
}

func (h *RedisHook) ProcessPipelineHook(next goredis.ProcessPipelineHook) goredis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []goredis.Cmder) error {
		start := time.Now()
		err := next(ctx, cmds)
		duration := time.Since(start).Seconds()
		for _, cmd := range cmds {
			name := cmd.Name()
			redisCommandsTotal.WithLabelValues(name).Inc()
			redisCommandDuration.WithLabelValues(name).Observe(duration)
			if err != nil && err != goredis.Nil {
				redisErrorsTotal.WithLabelValues(name).Inc()
			}
		}
		return err
	}
}

var _ goredis.Hook = (*RedisHook)(nil)

// ── 连接池指标采集器 ──────────────────────────────

type redisPoolCollector struct {
	client     *goredis.Client
	totalConns prometheus.Gauge
	idleConns  prometheus.Gauge
	staleConns prometheus.Gauge
	hits       prometheus.Gauge
	misses     prometheus.Gauge
	timeouts   prometheus.Gauge
}

// RegisterRedisPoolStats 注册 Redis 连接池指标采集器。
func RegisterRedisPoolStats(client *goredis.Client, name string) {
	c := &redisPoolCollector{
		client: client,
		totalConns: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: Namespace, Subsystem: redisSubsystem,
			Name: "pool_total_connections",
			Help: "Current number of total connections in the pool.",
			ConstLabels: prometheus.Labels{"instance": name},
		}),
		idleConns: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: Namespace, Subsystem: redisSubsystem,
			Name: "pool_idle_connections",
			Help: "Current number of idle connections in the pool.",
			ConstLabels: prometheus.Labels{"instance": name},
		}),
		staleConns: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: Namespace, Subsystem: redisSubsystem,
			Name: "pool_stale_connections_total",
			Help: "Total number of stale connections removed from the pool.",
			ConstLabels: prometheus.Labels{"instance": name},
		}),
		hits: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: Namespace, Subsystem: redisSubsystem,
			Name: "pool_hits_total",
			Help: "Total number of times a free connection was found.",
			ConstLabels: prometheus.Labels{"instance": name},
		}),
		misses: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: Namespace, Subsystem: redisSubsystem,
			Name: "pool_misses_total",
			Help: "Total number of times a free connection was NOT found.",
			ConstLabels: prometheus.Labels{"instance": name},
		}),
		timeouts: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: Namespace, Subsystem: redisSubsystem,
			Name: "pool_timeouts_total",
			Help: "Total number of wait timeouts.",
			ConstLabels: prometheus.Labels{"instance": name},
		}),
	}
	prometheus.MustRegister(c)
}

func (c *redisPoolCollector) Describe(ch chan<- *prometheus.Desc) {
	c.totalConns.Describe(ch)
	c.idleConns.Describe(ch)
	c.staleConns.Describe(ch)
	c.hits.Describe(ch)
	c.misses.Describe(ch)
	c.timeouts.Describe(ch)
}

func (c *redisPoolCollector) Collect(ch chan<- prometheus.Metric) {
	stats := c.client.PoolStats()
	c.totalConns.Set(float64(stats.TotalConns))
	c.idleConns.Set(float64(stats.IdleConns))
	c.staleConns.Set(float64(stats.StaleConns))
	c.hits.Set(float64(stats.Hits))
	c.misses.Set(float64(stats.Misses))
	c.timeouts.Set(float64(stats.Timeouts))
	c.totalConns.Collect(ch)
	c.idleConns.Collect(ch)
	c.staleConns.Collect(ch)
	c.hits.Collect(ch)
	c.misses.Collect(ch)
	c.timeouts.Collect(ch)
}

var _ prometheus.Collector = (*redisPoolCollector)(nil)
