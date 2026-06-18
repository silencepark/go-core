package config

// KafkaConfig Kafka 配置，包含生产者与批量消费者配置.
type KafkaConfig struct {
	Brokers   []string                  `mapstructure:"brokers"`
	Version   string                    `mapstructure:"version"`
	GroupID   string                    `mapstructure:"group_id"`
	Consumers map[string]ConsumerConfig `mapstructure:"consumers"`
}

// ConsumerConfig 单个消费者配置，topic / group_id / 并发数全部外置.
type ConsumerConfig struct {
	Topic      string `mapstructure:"topic"`
	GroupID    string `mapstructure:"group_id"`
	WorkerNum  int    `mapstructure:"worker_num"`
	MaxRetries int    `mapstructure:"max_retries"`
	DLQTopic   string `mapstructure:"dlq_topic"`
}
