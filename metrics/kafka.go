package metrics

import (
	"time"

	"github.com/IBM/sarama"
	"github.com/prometheus/client_golang/prometheus"
)

const kafkaSubsystem = "kafka"

var (
	kafkaProduceTotal    *prometheus.CounterVec
	kafkaProduceDuration *prometheus.HistogramVec
	kafkaProduceErrors   *prometheus.CounterVec
	kafkaConsumeTotal    *prometheus.CounterVec
	kafkaConsumeDuration *prometheus.HistogramVec
	kafkaConsumeErrors   *prometheus.CounterVec
)

func registerKafka() {
	kafkaProduceTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: Namespace, Subsystem: kafkaSubsystem,
			Name: "produce_messages_total",
			Help: "Total number of produced Kafka messages by topic.",
		},
		[]string{"topic"},
	)
	kafkaProduceDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: Namespace, Subsystem: kafkaSubsystem,
			Name:    "produce_duration_seconds",
			Help:    "Kafka produce duration in seconds.",
			Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5},
		},
		[]string{"topic"},
	)
	kafkaProduceErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: Namespace, Subsystem: kafkaSubsystem,
			Name: "produce_errors_total",
			Help: "Total number of failed Kafka produce calls.",
		},
		[]string{"topic"},
	)
	kafkaConsumeTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: Namespace, Subsystem: kafkaSubsystem,
			Name: "consume_messages_total",
			Help: "Total number of consumed Kafka messages by group and topic.",
		},
		[]string{"group", "topic"},
	)
	kafkaConsumeDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: Namespace, Subsystem: kafkaSubsystem,
			Name:    "consume_duration_seconds",
			Help:    "Kafka message processing duration in seconds.",
			Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10, 30},
		},
		[]string{"group", "topic"},
	)
	kafkaConsumeErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: Namespace, Subsystem: kafkaSubsystem,
			Name: "consume_errors_total",
			Help: "Total number of failed Kafka message processing.",
		},
		[]string{"group", "topic"},
	)
	prometheus.MustRegister(
		kafkaProduceTotal, kafkaProduceDuration, kafkaProduceErrors,
		kafkaConsumeTotal, kafkaConsumeDuration, kafkaConsumeErrors,
	)
}

// ── Producer Wrapper ───────────────────────────────

// WrapSyncProducer 用指标包装 sarama.SyncProducer，自动采集生产端指标。
func WrapSyncProducer(next sarama.SyncProducer) sarama.SyncProducer {
	return &metricsSyncProducer{next: next}
}

type metricsSyncProducer struct {
	next sarama.SyncProducer
}

func (p *metricsSyncProducer) SendMessage(msg *sarama.ProducerMessage) (partition int32, offset int64, err error) {
	start := time.Now()
	topic := msg.Topic
	partition, offset, err = p.next.SendMessage(msg)
	kafkaProduceTotal.WithLabelValues(topic).Inc()
	kafkaProduceDuration.WithLabelValues(topic).Observe(time.Since(start).Seconds())
	if err != nil {
		kafkaProduceErrors.WithLabelValues(topic).Inc()
	}
	return
}

func (p *metricsSyncProducer) SendMessages(msgs []*sarama.ProducerMessage) error {
	start := time.Now()
	err := p.next.SendMessages(msgs)
	for _, msg := range msgs {
		topic := msg.Topic
		kafkaProduceTotal.WithLabelValues(topic).Inc()
		if err != nil {
			kafkaProduceErrors.WithLabelValues(topic).Inc()
		}
	}
	if len(msgs) > 0 {
		kafkaProduceDuration.WithLabelValues(msgs[0].Topic).Observe(time.Since(start).Seconds())
	}
	return err
}

func (p *metricsSyncProducer) Close() error                            { return p.next.Close() }
func (p *metricsSyncProducer) TxnStatus() sarama.ProducerTxnStatusFlag { return p.next.TxnStatus() }
func (p *metricsSyncProducer) IsTransactional() bool                   { return p.next.IsTransactional() }
func (p *metricsSyncProducer) BeginTxn() error                         { return p.next.BeginTxn() }
func (p *metricsSyncProducer) CommitTxn() error                        { return p.next.CommitTxn() }
func (p *metricsSyncProducer) AbortTxn() error                         { return p.next.AbortTxn() }
func (p *metricsSyncProducer) AddOffsetsToTxn(offsets map[string][]*sarama.PartitionOffsetMetadata, groupId string) error {
	return p.next.AddOffsetsToTxn(offsets, groupId)
}
func (p *metricsSyncProducer) AddOffsetsToTxnWithGroupMetadata(offsets map[string][]*sarama.PartitionOffsetMetadata, groupMetadata *sarama.ConsumerGroupMetadata) error {
	return p.next.AddOffsetsToTxnWithGroupMetadata(offsets, groupMetadata)
}
func (p *metricsSyncProducer) AddMessageToTxn(msg *sarama.ConsumerMessage, groupId string, metadata *string) error {
	return p.next.AddMessageToTxn(msg, groupId, metadata)
}
func (p *metricsSyncProducer) AddMessageToTxnWithGroupMetadata(msg *sarama.ConsumerMessage, groupMetadata *sarama.ConsumerGroupMetadata, metadata *string) error {
	return p.next.AddMessageToTxnWithGroupMetadata(msg, groupMetadata, metadata)
}

var _ sarama.SyncProducer = (*metricsSyncProducer)(nil)

// ── Consumer Observers ─────────────────────────────

// ObserveKafkaConsume 记录消费一条 Kafka 消息的耗时和结果。
func ObserveKafkaConsume(group, topic string, start time.Time, err error) {
	kafkaConsumeTotal.WithLabelValues(group, topic).Inc()
	kafkaConsumeDuration.WithLabelValues(group, topic).Observe(time.Since(start).Seconds())
	if err != nil {
		kafkaConsumeErrors.WithLabelValues(group, topic).Inc()
	}
}
