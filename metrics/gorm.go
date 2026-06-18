package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"gorm.io/gorm"
)

const gormSubsystem = "gorm"

var (
	gormQueryCounter  *prometheus.CounterVec
	gormQueryDuration *prometheus.HistogramVec
	gormQueryErrors   *prometheus.CounterVec
	gormQueryInFlight *prometheus.GaugeVec
)

func registerGORM() {
	gormQueryCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: Namespace,
			Subsystem: gormSubsystem,
			Name:      "queries_total",
			Help:      "Total number of GORM queries by operation and table.",
		},
		[]string{"operation", "table"},
	)
	gormQueryDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: Namespace,
			Subsystem: gormSubsystem,
			Name:      "query_duration_seconds",
			Help:      "GORM query duration in seconds.",
			Buckets:   []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		},
		[]string{"operation", "table"},
	)
	gormQueryErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: Namespace,
			Subsystem: gormSubsystem,
			Name:      "queries_errors_total",
			Help:      "Total number of failed GORM queries.",
		},
		[]string{"operation", "table"},
	)
	gormQueryInFlight = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: Namespace,
			Subsystem: gormSubsystem,
			Name:      "queries_in_flight",
			Help:      "Current number of in-flight GORM queries.",
		},
		[]string{"operation", "table"},
	)
	prometheus.MustRegister(gormQueryCounter, gormQueryDuration, gormQueryErrors, gormQueryInFlight)
}

func tableFromDB(db *gorm.DB) string {
	if db.Statement == nil {
		return "<unknown>"
	}
	if db.Statement.Table != "" {
		return db.Statement.Table
	}
	return "<unknown>"
}

func beforeCallback(op string) func(*gorm.DB) {
	return func(db *gorm.DB) {
		table := tableFromDB(db)
		db.InstanceSet("metrics:start", time.Now())
		db.InstanceSet("metrics:table", table)
		gormQueryInFlight.WithLabelValues(op, table).Inc()
	}
}

func afterCallback(op string) func(*gorm.DB) {
	return func(db *gorm.DB) {
		startVal, ok := db.InstanceGet("metrics:start")
		if !ok {
			return
		}
		elapsed := time.Since(startVal.(time.Time))

		table := "<unknown>"
		if t, ok := db.InstanceGet("metrics:table"); ok {
			table = t.(string)
		}

		gormQueryInFlight.WithLabelValues(op, table).Dec()
		gormQueryCounter.WithLabelValues(op, table).Inc()
		gormQueryDuration.WithLabelValues(op, table).Observe(elapsed.Seconds())
		if db.Error != nil {
			gormQueryErrors.WithLabelValues(op, table).Inc()
		}
	}
}

// GORMPlugin 实现 gorm.Plugin 接口，自动采集 GORM 查询指标。
//
//	db.Use(&metrics.GORMPlugin{})
type GORMPlugin struct{}

func (p *GORMPlugin) Name() string { return "metrics:GORMPlugin" }

func (p *GORMPlugin) Initialize(db *gorm.DB) error {
	cbs := db.Callback()

	cbs.Create().Before("gorm:create").Register("metrics:before_create", beforeCallback("create"))
	cbs.Create().After("gorm:create").Register("metrics:after_create", afterCallback("create"))

	cbs.Query().Before("gorm:query").Register("metrics:before_query", beforeCallback("query"))
	cbs.Query().After("gorm:query").Register("metrics:after_query", afterCallback("query"))

	cbs.Update().Before("gorm:update").Register("metrics:before_update", beforeCallback("update"))
	cbs.Update().After("gorm:update").Register("metrics:after_update", afterCallback("update"))

	cbs.Delete().Before("gorm:delete").Register("metrics:before_delete", beforeCallback("delete"))
	cbs.Delete().After("gorm:delete").Register("metrics:after_delete", afterCallback("delete"))

	cbs.Raw().Before("gorm:row_query").Register("metrics:before_raw", beforeCallback("raw"))
	cbs.Raw().After("gorm:row_query").Register("metrics:after_raw", afterCallback("raw"))

	return nil
}

var _ gorm.Plugin = (*GORMPlugin)(nil)
