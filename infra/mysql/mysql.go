// Package mysql 提供 MySQL 业务库连接封装。
// *gorm.DB 为私有字段，外部只能通过显式方法访问，避免 ORM 实现细节泄露。
package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/silencepark/go-core/config"
	"github.com/silencepark/go-core/health"
	"github.com/silencepark/go-core/infra"
	"github.com/silencepark/go-core/metrics"
)

// DB 业务库连接。*gorm.DB 为私有字段，避免 ORM 实现细节泄露。
type DB struct {
	db *gorm.DB
}

// New 根据配置初始化业务库连接，自动注册健康检查和指标。
// key 对应 YAML 中 mysql.<key> 的数据源名称（如 "platform"、"user"、"order"）。
func New(cfg *config.Config, cg *infra.CloserGroup, hr *health.HealthRegistry, key string) (*DB, error) {
	sc, ok := cfg.MySQL[key]
	if !ok {
		return nil, fmt.Errorf("mysql.%s not configured", key)
	}

	db, err := openSource(sc, cfg.App.Mode)
	if err != nil {
		return nil, fmt.Errorf("open platform db: %w", err)
	}

	// 注册 Prometheus 指标插件
	db.Use(&metrics.GORMPlugin{})

	pdb := &DB{db: db}
	cg.Add(pdb)

	// 自动注册 MySQL 连通性健康检查
	hr.Add("mysql", func(ctx context.Context) error {
		sqlDB, err := pdb.SQLDB()
		if err != nil {
			return fmt.Errorf("get sql.DB: %w", err)
		}
		return sqlDB.PingContext(ctx)
	})

	// 注册 DB 连接池 Prometheus 指标
	if sqlDB, err := db.DB(); err == nil {
		metrics.RegisterDBStats(sqlDB, "platform")
	}

	return pdb, nil
}

func openSource(cfg config.SourceConfig, appMode string) (*gorm.DB, error) {
	level := logger.Info
	if appMode == "release" {
		level = logger.Silent
	}

	db, err := gorm.Open(mysql.Open(cfg.DSN()), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
		Logger:                                   logger.Default.LogMode(level),
	})
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxIdleConns(cfg.MaxIdle)
	sqlDB.SetMaxOpenConns(cfg.MaxOpen)
	sqlDB.SetConnMaxLifetime(time.Duration(cfg.MaxLifetime) * time.Second)
	return db, nil
}

// WithContext 返回绑定 context 的 *gorm.DB，Repository 层唯一访问 GORM 的入口。
func (d *DB) WithContext(ctx context.Context) *gorm.DB {
	return d.db.WithContext(ctx)
}

// SQLDB 返回底层 *sql.DB，供健康检查、指标采集等基础设施使用。
func (d *DB) SQLDB() (*sql.DB, error) {
	return d.db.DB()
}

// Close 关闭连接池。
func (d *DB) Close(ctx context.Context) error {
	sqlDB, err := d.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// Name 返回资源名称。
func (d *DB) Name() string { return "platform_db" }
