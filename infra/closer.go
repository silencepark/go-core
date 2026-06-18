package infra

import (
	"context"
	"fmt"
)

// Closer 统一资源关闭接口，所有基础设施与长生命周期对象均需实现.
type Closer interface {
	Close(ctx context.Context) error
	Name() string
}

// CloserGroup 管理所有资源，按添加顺序逆序关闭.
type CloserGroup struct {
	closers []Closer
}

// NewCloserGroup 创建空的资源关闭组.
func NewCloserGroup() *CloserGroup {
	return &CloserGroup{}
}

// Add 向关闭组追加资源.
func (g *CloserGroup) Add(c Closer) {
	g.closers = append(g.closers, c)
}

// CloseAll 逆序关闭所有资源并收集错误.
func (g *CloserGroup) CloseAll(ctx context.Context) []error {
	var errs []error
	for i := len(g.closers) - 1; i >= 0; i-- {
		c := g.closers[i]
		if err := c.Close(ctx); err != nil {
			errs = append(errs, fmt.Errorf("close %s: %w", c.Name(), err))
		}
	}
	return errs
}
