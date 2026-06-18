// Package redis 提供 Redis 客户端封装（单节点或集群）。
package redis

import (
	"context"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/silencepark/go-core/config"
	"github.com/silencepark/go-core/health"
	"github.com/silencepark/go-core/infra"
	"github.com/silencepark/go-core/metrics"
)

// Redis Redis 客户端聚合。
type Redis struct {
	client goredis.UniversalClient
}

// NewRedis 初始化 Redis 客户端（单节点或集群），同时注册 Prometheus 指标和健康检查。
func NewRedis(cfg *config.Config, cg *infra.CloserGroup, hr *health.HealthRegistry) (*Redis, error) {
	rcfg := cfg.Redis
	if len(rcfg.Addrs) == 0 {
		return nil, fmt.Errorf("redis addrs is empty")
	}

	var client goredis.UniversalClient
	if len(rcfg.Addrs) > 1 {
		cc := goredis.NewClusterClient(&goredis.ClusterOptions{
			Addrs:    rcfg.Addrs,
			Password: rcfg.Password,
			PoolSize: rcfg.PoolSize,
		})
		cc.AddHook(&metrics.RedisHook{})
		client = cc
	} else {
		sc := goredis.NewClient(&goredis.Options{
			Addr:         rcfg.Addrs[0],
			Password:     rcfg.Password,
			DB:           rcfg.DB,
			PoolSize:     rcfg.PoolSize,
			MinIdleConns: rcfg.MinIdle,
		})
		sc.AddHook(&metrics.RedisHook{})
		metrics.RegisterRedisPoolStats(sc, "default")
		client = sc
	}

	pingCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping: %w", err)
	}

	r := &Redis{client: client}
	cg.Add(r)

	// 自动注册 Redis 连通性健康检查
	hr.Add("redis", func(ctx context.Context) error {
		return r.Client().Ping(ctx).Err()
	})

	return r, nil
}

// Client 返回底层 Redis 客户端。
func (r *Redis) Client() goredis.UniversalClient { return r.client }

// Close 关闭 Redis 连接。
func (r *Redis) Close(ctx context.Context) error {
	return r.client.Close()
}

// Name 返回资源名称。
func (r *Redis) Name() string { return "redis" }
