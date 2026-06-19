// Package kafka 提供 Kafka 消费者 Runner：重试、DLQ、panic 恢复、指标采集。
// 业务只需提供 consumer.MessageHandler，其余由 Runner 统一处理。
package kafka

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/IBM/sarama"
	"go.uber.org/zap"

	"github.com/silencepark/go-core/config"
	"github.com/silencepark/go-core/consumer"
	gocore "github.com/silencepark/go-core/infra/kafka"
	"github.com/silencepark/go-core/log"
	"github.com/silencepark/go-core/metrics"
)

// Runner Kafka 消费者，实现 consumer.Runner。
type Runner struct {
	client  *gocore.Kafka
	cfg     config.ConsumerConfig
	name    string
	handler consumer.MessageHandler
	logger  *log.Logger

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewRunner 创建 Kafka Runner。运行需调用 Run()。
func NewRunner(name string, cfg config.ConsumerConfig, client *gocore.Kafka, handler consumer.MessageHandler, logger *log.Logger) *Runner {
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
	return fmt.Sprintf("kafka:%s", r.name)
}

// Run 实现 consumer.Runner。阻塞消费直至 ctx 取消。
func (r *Runner) Run(ctx context.Context) error {
	ctx, r.cancel = context.WithCancel(ctx)
	defer r.cancel()

	cg, err := r.client.ConsumerGroup(r.cfg.GroupID)
	if err != nil {
		return fmt.Errorf("kafka consumer group %q: %w", r.cfg.GroupID, err)
	}

	r.logger.Info("kafka runner starting",
		zap.String("name", r.name),
		zap.String("topic", r.cfg.Topic),
		zap.String("group_id", r.cfg.GroupID),
		zap.Int("workers", r.cfg.WorkerNum),
	)

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		if err := cg.Consume(ctx, []string{r.cfg.Topic}, &consumerGroupHandler{runner: r}); err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			r.logger.Error("kafka consume error, will reconnect",
				zap.String("name", r.name),
				zap.Error(err),
			)
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(time.Second):
			}
		}
	}
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
	return nil
}

// ── sarama.ConsumerGroupHandler（内部实现，不对外暴露）─────────

type consumerGroupHandler struct {
	runner *Runner
}

func (h *consumerGroupHandler) Setup(s sarama.ConsumerGroupSession) error {
	h.runner.logger.Debug("kafka session setup", zap.String("name", h.runner.name))
	return nil
}

func (h *consumerGroupHandler) Cleanup(s sarama.ConsumerGroupSession) error {
	h.runner.logger.Debug("kafka session cleanup", zap.String("name", h.runner.name))
	return nil
}

func (h *consumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	r := h.runner
	sem := make(chan struct{}, r.cfg.WorkerNum)

	for {
		select {
		case msg, ok := <-claim.Messages():
			if !ok {
				for i := 0; i < r.cfg.WorkerNum; i++ {
					sem <- struct{}{}
				}
				return nil
			}

			select {
			case sem <- struct{}{}:
			case <-session.Context().Done():
				return nil
			}

			r.wg.Add(1)
			go func(m *sarama.ConsumerMessage) {
				defer r.wg.Done()
				defer func() { <-sem }()
				r.processMessage(session, m)
			}(msg)

		case <-session.Context().Done():
			return nil
		}
	}
}

// ── 内部处理 ─────────────────────────────────────────────────

func (r *Runner) processMessage(session sarama.ConsumerGroupSession, msg *sarama.ConsumerMessage) {
	start := time.Now()
	var err error

	defer func() {
		if rec := recover(); rec != nil {
			err = fmt.Errorf("handler panic: %v", rec)
			r.logger.Error("kafka handler panic recovered",
				zap.String("name", r.name),
				zap.String("topic", r.cfg.Topic),
				zap.Any("panic", rec),
			)
		}
		metrics.ObserveKafkaConsume(r.cfg.GroupID, r.cfg.Topic, start, err)
		session.MarkMessage(msg, "")
	}()

	if err = r.processWithRetry(session.Context(), msg.Key, msg.Value); err != nil {
		r.logger.Error("kafka message failed after retries",
			zap.String("name", r.name),
			zap.String("topic", r.cfg.Topic),
			zap.Int("max_retries", r.cfg.MaxRetries),
			zap.Error(err),
		)
		r.publishDLQ(msg, err)
	}
}

func (r *Runner) processWithRetry(ctx context.Context, key, value []byte) error {
	var lastErr error
	for attempt := 0; attempt <= r.cfg.MaxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(attempt) * 200 * time.Millisecond
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
		}
		if err := r.handler(ctx, key, value); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}
	return lastErr
}

func (r *Runner) publishDLQ(msg *sarama.ConsumerMessage, cause error) {
	if r.cfg.DLQTopic == "" {
		r.logger.Warn("DLQ topic not configured, message dropped",
			zap.String("name", r.name),
			zap.String("topic", r.cfg.Topic),
		)
		return
	}

	producer, err := r.client.Producer()
	if err != nil {
		r.logger.Error("failed to get kafka producer for DLQ", zap.Error(err))
		return
	}

	pm := &sarama.ProducerMessage{
		Topic: r.cfg.DLQTopic,
		Key:   sarama.ByteEncoder(msg.Key),
		Value: sarama.ByteEncoder(msg.Value),
		Headers: []sarama.RecordHeader{
			{Key: []byte("x-original-topic"), Value: []byte(r.cfg.Topic)},
			{Key: []byte("x-error"), Value: []byte(cause.Error())},
		},
	}
	if _, _, err := producer.SendMessage(pm); err != nil {
		r.logger.Error("failed to publish to DLQ",
			zap.String("dlq_topic", r.cfg.DLQTopic),
			zap.Error(err),
		)
	}
}
