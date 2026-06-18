// Package cache 提供 Redis 缓存与分布式锁的通用访问层。
package cache

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"

	"github.com/silencepark/go-core/infra/redis"
)

const lockKeyPrefix = "lock:"

// Cache Redis 缓存与分布式锁访问层。
type Cache struct {
	rdb goredis.UniversalClient
}

// New 创建 Cache 实例。
func New(rds *redis.Redis) *Cache {
	return &Cache{rdb: rds.Client()}
}

// Set 写入缓存。
func (c *Cache) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	return c.rdb.Set(ctx, key, value, ttl).Err()
}

// Get 读取缓存。key 不存在时返回空字符串和 nil error。
func (c *Cache) Get(ctx context.Context, key string) (string, error) {
	val, err := c.rdb.Get(ctx, key).Result()
	if errors.Is(err, goredis.Nil) {
		return "", nil
	}
	return val, err
}

// Delete 删除缓存。
func (c *Cache) Delete(ctx context.Context, key string) error {
	return c.rdb.Del(ctx, key).Err()
}

// WithLock 在分布式锁保护下执行 fn。
// 使用 SetNX + Lua 脚本安全释放，避免误删其他持有者的锁。
func (c *Cache) WithLock(ctx context.Context, key string, ttl time.Duration, fn func() error) error {
	lockKey := lockKeyPrefix + key
	token := uuid.NewString()

	acquired, err := c.rdb.SetNX(ctx, lockKey, token, ttl).Result()
	if err != nil {
		return fmt.Errorf("acquire lock %s: %w", key, err)
	}
	if !acquired {
		return fmt.Errorf("lock %s not acquired", key)
	}

	defer func() {
		script := goredis.NewScript(`if redis.call("get", KEYS[1]) == ARGV[1] then return redis.call("del", KEYS[1]) else return 0 end`)
		_ = script.Run(ctx, c.rdb, []string{lockKey}, token).Err()
	}()

	return fn()
}
