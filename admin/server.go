// Package admin 提供运维管理 HTTP 服务。
// 一行注入即可获得独立的运维端口，包含健康检查、Prometheus 指标和 pprof。
//
// 使用方式：
//
//	// Wire 注入 — New 内部已完成 metrics 初始化和端口监听
//	adminSrv := admin.New(cfg, healthRegistry)
//	closer.Add(adminSrv) // 随 CloserGroup 自动清理
//
// 端点：
//
//	GET  /health         — 存活检查（始终 200）
//	GET  /ready          — 就绪检查（聚合所有组件状态）
//	GET  /metrics        — Prometheus 指标
//	GET  /debug/pprof/*  — Go pprof 性能分析
//
// cfg.AdminPort 为 0 时 New 返回 nil，可禁用运维端口。
package admin

import (
	"context"
	"errors"
	"fmt"
	stdlog "log"
	"net/http"
	"net/http/pprof"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/silencepark/go-core/config"
	"github.com/silencepark/go-core/health"
	"github.com/silencepark/go-core/metrics"
)

// Server 运维 HTTP 服务，独立端口运行。
// 实现 infra.Closer，可加入 CloserGroup 随主流程自动清理。
type Server struct {
	srv *http.Server
}

// New 创建并启动运维 Server。
// 内部自动完成 metrics 命名空间设置和注册，调用方无需关心。
// cfg.AdminPort 为 0 时返回 nil。
func New(cfg *config.AppConfig, hr *health.HealthRegistry) *Server {
	port := cfg.AdminPort
	if port <= 0 {
		return nil
	}

	// 一行搞定：命名空间 + 注册所有 Prometheus 指标
	metrics.Init(cfg.Name)

	engine := gin.New()
	engine.Use(gin.Recovery())

	// 存活检查 — 进程还活着就返回 200
	engine.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// 就绪检查 — 聚合所有注册组件的健康状态
	engine.GET("/ready", func(c *gin.Context) {
		results := hr.Ready(c.Request.Context())
		healthy := true
		for _, status := range results {
			if status != "ok" {
				healthy = false
				break
			}
		}
		code := http.StatusOK
		if !healthy {
			code = http.StatusServiceUnavailable
		}
		c.JSON(code, gin.H{"status": results})
	})

	// Prometheus 指标
	engine.GET("/metrics", metrics.Handler())

	// pprof 性能分析
	pprofGroup := engine.Group("/debug/pprof")
	{
		pprofGroup.GET("/", gin.WrapF(pprof.Index))
		pprofGroup.GET("/cmdline", gin.WrapF(pprof.Cmdline))
		pprofGroup.GET("/profile", gin.WrapF(pprof.Profile))
		pprofGroup.POST("/symbol", gin.WrapF(pprof.Symbol))
		pprofGroup.GET("/symbol", gin.WrapF(pprof.Symbol))
		pprofGroup.GET("/trace", gin.WrapF(pprof.Trace))
		pprofGroup.GET("/allocs", gin.WrapH(pprof.Handler("allocs")))
		pprofGroup.GET("/block", gin.WrapH(pprof.Handler("block")))
		pprofGroup.GET("/goroutine", gin.WrapH(pprof.Handler("goroutine")))
		pprofGroup.GET("/heap", gin.WrapH(pprof.Handler("heap")))
		pprofGroup.GET("/mutex", gin.WrapH(pprof.Handler("mutex")))
		pprofGroup.GET("/threadcreate", gin.WrapH(pprof.Handler("threadcreate")))
	}

	s := &Server{
		srv: &http.Server{
			Addr:         fmt.Sprintf(":%d", port),
			Handler:      engine,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
	}

	// fire-and-forget：运维端口后台静默监听，不阻塞主流程
	go func() {
		if err := s.srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			stdlog.Printf("[admin] listen on :%d failed: %v", port, err)
		}
	}()

	return s
}

// Close 优雅关闭运维 HTTP 服务，实现 infra.Closer。
func (s *Server) Close(ctx context.Context) error {
	if s == nil || s.srv == nil {
		return nil
	}
	return s.srv.Shutdown(ctx)
}

// Name 返回资源名称，实现 infra.Closer。
func (s *Server) Name() string { return "admin" }
