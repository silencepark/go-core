package config

// RabbitMQConfig RabbitMQ 配置.
type RabbitMQConfig struct {
	URL       string                       `mapstructure:"url"`
	Consumers map[string]RMQConsumerConfig `mapstructure:"consumers"`
}

// RMQConsumerConfig 单个 RabbitMQ 消费者配置.
type RMQConsumerConfig struct {
	Queue      string `mapstructure:"queue"`
	WorkerNum  int    `mapstructure:"worker_num"`
	MaxRetries int    `mapstructure:"max_retries"`
	DLQQueue   string `mapstructure:"dlq_queue"`
}
