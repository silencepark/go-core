package health

import "context"

// HealthRegistry 健康检查注册中心。
// 基础设施组件（DB、Redis 等）在构造函数中自动 Add 自身检查项，
// 业务层只需注入 HealthRegistry，无需手动注册检查。
type HealthRegistry struct {
	checker *Checker
}

// NewHealthRegistry 创建健康检查注册中心。
func NewHealthRegistry() *HealthRegistry {
	return &HealthRegistry{checker: New()}
}

// Add 注册一个检查项，由各组件构造函数自动调用，无需业务层感知。
func (r *HealthRegistry) Add(name string, fn CheckFunc) {
	r.checker.Add(name, fn)
}

// Ready 并发执行所有检查，返回各组件状态。
func (r *HealthRegistry) Ready(ctx context.Context) map[string]string {
	return r.checker.Ready(ctx)
}

// IsHealthy 返回所有检查是否通过。
func (r *HealthRegistry) IsHealthy(ctx context.Context) bool {
	return r.checker.IsHealthy(ctx)
}
