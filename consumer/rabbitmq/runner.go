// Package rabbitmq 提供 RabbitMQ 消费者 Runner：重试、DLQ、panic 恢复、指标采集。
// 业务只需提供 consumer.MessageHandler，其余由 Runner 统一处理。
package rabbitmq

import (
	"context"
	"fmt"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"

	"github.com/silencepark/go-core/config"
	"github.com/silencepark/go-core/consumer"
	gocore "github.com/silencepark/go-core/infra/rabbitmq"
	"github.com/silencepark/go-core/log"
	"github.com/silencepark/go-core/metrics"
)

// Runner RabbitMQ 消费者，实现 consumer.Runner。
type Runner struct {
	client  *gocore.RabbitMQ
	cfg     config.RMQConsumerConfig
	name    string
	handler consumer.MessageHandler
	logger  *log.Logger

	cancel context.CancelFunc
	wg     sync.WaitGroup
	ch     *amqp.Channel
}

// NewRunner 创建 RMQ Runner。运行需调用 Run()。
func NewRunner(name string, cfg config.RMQConsumerConfig, client *gocore.RabbitMQ, handler consumer.MessageHandler, logger *log.Logger) *Runner {
	if cfg.WorkerNum <= 0 {
		cfg.WorkerNum = 1
	}
	return &Runner{
		client:  client,
		cfg:     cfg,
		name:    name,
		handler: handler,
		logger:  logger,
	}
}

// Name 实现 consumer.Runner。
func (r *Runner) Name() string {
	return fmt.Sprintf("rabbitmq:%s", r.name)
}

// Run 实现 consumer.Runner。阻塞消费直至 ctx 取消。
func (r *Runner) Run(ctx context.Context) error {
	ctx, r.cancel = context.WithCancel(ctx)
	defer r.cancel()

	ch, err := r.client.Channel()
	if err != nil {
		return fmt.Errorf("rmq channel: %w", err)
	}
	r.ch = ch

	if _, err := ch.QueueDeclarePassive(r.cfg.Queue, true, false, false, false, nil); err != nil {
		r.logger.Warn("rmq queue declare passive failed, queue may not exist",
			zap.String("name", r.name),
			zap.String("queue", r.cfg.Queue),
			zap.Error(err),
		)
	}

	deliveries, err := ch.Consume(r.cfg.Queue, r.name, false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("rmq consume %q: %w", r.cfg.Queue, err)
	}

	r.logger.Info("rmq runner starting",
		zap.String("name", r.name),
		zap.String("queue", r.cfg.Queue),
		zap.Int("workers", r.cfg.WorkerNum),
	)

	for i := 0; i < r.cfg.WorkerNum; i++ {
		r.wg.Add(1)
		go func() {
			defer r.wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case delivery, ok := <-deliveries:
					if !ok {
						return
					}
					r.processDelivery(ctx, ch, delivery)
				}
			}
		}()
	}

	<-ctx.Done()
	return nil
}

// Close 实现 infra.Closer。
func (r *Runner) Close(ctx context.Context) error {
	if r.cancel != nil {
		r.cancel()
	}
	done := make(chan struct{})
	go func() {
		r.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
		return ctx.Err()
	}
	if r.ch != nil {
		if err := r.ch.Close(); err != nil {
			r.logger.Error("failed to close rmq channel", zap.Error(err))
		}
	}
	return nil
}

// ── 内部处理 ─────────────────────────────────────────────────

func (r *Runner) processDelivery(ctx context.Context, ch *amqp.Channel, delivery amqp.Delivery) {
	start := time.Now()
	var err error

	defer func() {
		if rec := recover(); rec != nil {
			err = fmt.Errorf("handler panic: %v", rec)
			r.logger.Error("rmq handler panic recovered",
				zap.String("name", r.name),
				zap.String("queue", r.cfg.Queue),
				zap.Any("panic", rec),
			)
		}
		metrics.ObserveRMQConsume(r.cfg.Queue, start, err)
		if ackErr := delivery.Ack(false); ackErr != nil {
			r.logger.Error("failed to ack rmq message", zap.Error(ackErr))
		}
	}()

	if err = r.processWithRetry(ctx, delivery.Body); err != nil {
		r.logger.Error("rmq message failed after retries",
			zap.String("name", r.name),
			zap.String("queue", r.cfg.Queue),
			zap.Int("max_retries", r.cfg.MaxRetries),
			zap.Error(err),
		)
		r.publishDLQ(ch, delivery, err)
	}
}

func (r *Runner) processWithRetry(ctx context.Context, value []byte) error {
	var lastErr error
	for attempt := 0; attempt <= r.cfg.MaxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(attempt) * 200 * time.Millisecond):
			}
		}
		if err := r.handler(ctx, nil, value); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}
	return lastErr
}

func (r *Runner) publishDLQ(ch *amqp.Channel, delivery amqp.Delivery, cause error) {
	if r.cfg.DLQQueue == "" {
		r.logger.Warn("DLQ queue not configured, message dropped",
			zap.String("name", r.name),
			zap.String("queue", r.cfg.Queue),
		)
		return
	}

	pub := amqp.Publishing{
		ContentType: delivery.ContentType,
		Body:        delivery.Body,
		Headers: amqp.Table{
			"x-original-queue": r.cfg.Queue,
			"x-error":          cause.Error(),
		},
	}
	if err := ch.Publish("", r.cfg.DLQQueue, false, false, pub); err != nil {
		r.logger.Error("failed to publish to DLQ",
			zap.String("dlq_queue", r.cfg.DLQQueue),
			zap.Error(err),
		)
	}
}
