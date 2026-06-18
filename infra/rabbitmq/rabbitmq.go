// Package rabbitmq 提供 RabbitMQ 连接封装。
package rabbitmq

import (
	"context"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/silencepark/go-core/config"
	"github.com/silencepark/go-core/infra"
)

// RabbitMQ RabbitMQ 连接聚合。
type RabbitMQ struct {
	conn *amqp.Connection
}

// NewRabbitMQ 初始化 RabbitMQ 连接；url 为空时跳过（无 RMQ Runner 场景）。
func NewRabbitMQ(cfg *config.Config, cg *infra.CloserGroup) (*RabbitMQ, error) {
	if cfg.RabbitMQ.URL == "" {
		return &RabbitMQ{}, nil
	}

	conn, err := amqp.Dial(cfg.RabbitMQ.URL)
	if err != nil {
		return nil, fmt.Errorf("rabbitmq dial: %w", err)
	}

	r := &RabbitMQ{conn: conn}
	cg.Add(r)
	return r, nil
}

// Channel 打开 Channel。
func (r *RabbitMQ) Channel() (*amqp.Channel, error) {
	if r.conn == nil {
		return nil, fmt.Errorf("rabbitmq not configured")
	}
	return r.conn.Channel()
}

// Close 关闭连接。
func (r *RabbitMQ) Close(ctx context.Context) error {
	if r.conn == nil {
		return nil
	}
	return r.conn.Close()
}

// Name 返回资源名称。
func (r *RabbitMQ) Name() string { return "rabbitmq" }
