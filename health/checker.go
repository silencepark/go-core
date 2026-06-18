// Package health 提供可复用的健康检查框架。
// 各服务注册 CheckFunc，Checker 聚合执行并返回状态。
package health

import (
	"context"
	"fmt"
	"sync"
)

// CheckFunc 健康检查函数。返回 nil 表示正常，返回 error 表示异常。
type CheckFunc func(ctx context.Context) error

// Checker 聚合多个健康检查，按名称管理。
type Checker struct {
	mu     sync.RWMutex
	checks map[string]CheckFunc
}

// New 创建空的 Checker。
func New() *Checker {
	return &Checker{checks: make(map[string]CheckFunc)}
}

// Add 注册一个检查项。
func (c *Checker) Add(name string, fn CheckFunc) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.checks[name] = fn
}

// Ready 并发执行所有注册的检查，返回各检查项状态。
// 状态值: "ok" 或 "error: <details>"。
func (c *Checker) Ready(ctx context.Context) map[string]string {
	c.mu.RLock()
	names := make([]string, 0, len(c.checks))
	fns := make([]CheckFunc, 0, len(c.checks))
	for n, fn := range c.checks {
		names = append(names, n)
		fns = append(fns, fn)
	}
	c.mu.RUnlock()

	results := make(map[string]string, len(names))
	var wg sync.WaitGroup
	var mu sync.Mutex

	for i, fn := range fns {
		wg.Add(1)
		go func(name string, checkFn CheckFunc) {
			defer wg.Done()
			if err := checkFn(ctx); err != nil {
				mu.Lock()
				results[name] = fmt.Sprintf("error: %s", err.Error())
				mu.Unlock()
			} else {
				mu.Lock()
				results[name] = "ok"
				mu.Unlock()
			}
		}(names[i], fn)
	}
	wg.Wait()
	return results
}

// IsHealthy 返回所有检查是否全部通过。
func (c *Checker) IsHealthy(ctx context.Context) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, fn := range c.checks {
		if fn(ctx) != nil {
			return false
		}
	}
	return true
}
