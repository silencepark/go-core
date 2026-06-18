// Package kafka 提供 Kafka 生产者与消费组客户端封装。
package kafka

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/IBM/sarama"

	"github.com/silencepark/go-core/config"
	"github.com/silencepark/go-core/infra"
	"github.com/silencepark/go-core/metrics"
)

// Kafka 聚合 Kafka 生产者与多个消费组客户端。
type Kafka struct {
	cfg            config.KafkaConfig
	producer       sarama.SyncProducer
	version        sarama.KafkaVersion
	consumerGroups map[string]sarama.ConsumerGroup
	mu             sync.Mutex
}

// NewKafka 初始化 Kafka 客户端，Producer 在启动时立即创建避免首请求延迟。
func NewKafka(cfg *config.Config, cg *infra.CloserGroup) (*Kafka, error) {
	version, err := sarama.ParseKafkaVersion(cfg.Kafka.Version)
	if err != nil {
		log.Printf("warn: invalid kafka version %q, fallback to v3.5.0.0: %v", cfg.Kafka.Version, err)
		version = sarama.V3_5_0_0
	}

	producer, err := createSyncProducer(cfg.Kafka.Brokers, version)
	if err != nil {
		return nil, fmt.Errorf("kafka producer: %w", err)
	}

	k := &Kafka{
		cfg:            cfg.Kafka,
		version:        version,
		producer:       metrics.WrapSyncProducer(producer),
		consumerGroups: make(map[string]sarama.ConsumerGroup),
	}
	cg.Add(k)
	return k, nil
}

func createSyncProducer(brokers []string, version sarama.KafkaVersion) (sarama.SyncProducer, error) {
	pcfg := sarama.NewConfig()
	pcfg.Producer.Return.Successes = true
	pcfg.Producer.RequiredAcks = sarama.WaitForLocal
	pcfg.Version = version
	return sarama.NewSyncProducer(brokers, pcfg)
}

// Producer 返回已初始化的同步生产者，内置指标采集。
func (k *Kafka) Producer() (sarama.SyncProducer, error) {
	k.mu.Lock()
	defer k.mu.Unlock()

	if k.producer == nil {
		return nil, fmt.Errorf("kafka producer not available")
	}
	return k.producer, nil
}

// ConsumerGroup 按 group_id 返回消费组客户端，相同 group_id 复用同一实例。
func (k *Kafka) ConsumerGroup(groupID string) (sarama.ConsumerGroup, error) {
	if groupID == "" {
		groupID = k.cfg.GroupID
	}

	k.mu.Lock()
	defer k.mu.Unlock()

	if cg, ok := k.consumerGroups[groupID]; ok {
		return cg, nil
	}

	ccfg := sarama.NewConfig()
	ccfg.Version = k.version
	ccfg.Consumer.Group.Rebalance.Strategy = sarama.BalanceStrategyRoundRobin
	ccfg.Consumer.Offsets.Initial = sarama.OffsetOldest

	cg, err := sarama.NewConsumerGroup(k.cfg.Brokers, groupID, ccfg)
	if err != nil {
		return nil, fmt.Errorf("kafka consumer group %s: %w", groupID, err)
	}
	k.consumerGroups[groupID] = cg
	return cg, nil
}

// Close 关闭生产者与所有消费组。
func (k *Kafka) Close(ctx context.Context) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	for id, cg := range k.consumerGroups {
		_ = cg.Close()
		delete(k.consumerGroups, id)
	}
	if k.producer != nil {
		err := k.producer.Close()
		k.producer = nil
		return err
	}
	return nil
}

// Name 返回资源名称。
func (k *Kafka) Name() string { return "kafka" }
