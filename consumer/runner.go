package consumer

import (
	"context"

	"github.com/silencepark/go-core/infra"
)

// Runner 后台消费单元接口。Run 阻塞直至 ctx 取消；同时实现 infra.Closer。
type Runner interface {
	Run(ctx context.Context) error
	infra.Closer
}
