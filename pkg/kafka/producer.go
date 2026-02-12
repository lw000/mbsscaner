package kafka

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/IBM/sarama"
	"go.uber.org/zap"
	"mbsscaner/config"
	"mbsscaner/pkg/modbus"
)

// Producer Kafka生产者
type Producer struct {
	producer sarama.SyncProducer
	topic    string
	logger   *zap.Logger
	wg       sync.WaitGroup
	stopCh   chan struct{}
}

// SimpleMessage 简化的消息结构
type SimpleMessage struct {
	Device string                 `json:"device"`
	Values map[string]interface{} `json:"values"`
}

// NewProducer 创建新的Kafka生产者
func NewProducer(kafkaConfig config.KafkaConfig, logger *zap.Logger) (*Producer, error) {
	// 创建Sarama配置
	saramaConfig := sarama.NewConfig()

	// 设置生产者配置
	saramaConfig.Producer.Return.Successes = true
	saramaConfig.Producer.Return.Errors = true
	saramaConfig.Producer.RequiredAcks = sarama.WaitForAll
	saramaConfig.Producer.Retry.Max = 5
	saramaConfig.Producer.Retry.Backoff = time.Second

	// 设置压缩类型
	switch kafkaConfig.Compression {
	case "gzip":
		saramaConfig.Producer.Compression = sarama.CompressionGZIP
	case "snappy":
		saramaConfig.Producer.Compression = sarama.CompressionSnappy
	case "lz4":
		saramaConfig.Producer.Compression = sarama.CompressionLZ4
	case "zstd":
		saramaConfig.Producer.Compression = sarama.CompressionZSTD
	default:
		saramaConfig.Producer.Compression = sarama.CompressionNone
	}

	// 设置批处理配置
	if kafkaConfig.BatchSize > 0 {
		saramaConfig.Producer.Flush.Messages = kafkaConfig.BatchSize
	}
	if kafkaConfig.FlushFrequency > 0 {
		saramaConfig.Producer.Flush.Frequency = time.Duration(kafkaConfig.FlushFrequency) * time.Millisecond
	}

	// 创建同步生产者
	producer, err := sarama.NewSyncProducer(kafkaConfig.Brokers, saramaConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create kafka producer: %w", err)
	}

	return &Producer{
		producer: producer,
		topic:    kafkaConfig.Topic,
		logger:   logger,
		stopCh:   make(chan struct{}),
	}, nil
}

// SendPointValues 发送点位数据（对象格式）
func (p *Producer) SendPointValues(deviceName string, values []modbus.PointValue) error {
	// 创建对象格式的数据
	valuesMap := make(map[string]interface{})
	for _, value := range values {
		valuesMap[value.Name] = value.Value
	}

	// 创建简化的消息
	message := SimpleMessage{
		Device: deviceName,
		Values: valuesMap,
	}

	// 序列化消息
	messageBytes, err := json.Marshal(message)
	if err != nil {
		p.logger.Error("failed to marshal optimized message",
			zap.Error(err),
			zap.String("device", deviceName))
		return fmt.Errorf("failed to marshal optimized message: %w", err)
	}

	// 创建Kafka消息
	kafkaMessage := &sarama.ProducerMessage{
		Topic: p.topic,
		Key:   sarama.StringEncoder(deviceName),
		Value: sarama.ByteEncoder(messageBytes),
		Headers: []sarama.RecordHeader{
			{
				Key:   []byte("device"),
				Value: []byte(deviceName),
			},
			{
				Key:   []byte("timestamp"),
				Value: []byte(time.Now().Format(time.RFC3339Nano)),
			},
			{
				Key:   []byte("source"),
				Value: []byte("mbsscaner"),
			},
			{
				Key:   []byte("optimized"),
				Value: []byte("true"),
			},
		},
	}

	// 发送消息
	partition, offset, err := p.producer.SendMessage(kafkaMessage)
	if err != nil {
		p.logger.Error("failed to send optimized message to kafka",
			zap.Error(err),
			zap.String("topic", p.topic),
			zap.String("device", deviceName))
		return fmt.Errorf("failed to send optimized message: %w", err)
	}

	p.logger.Debug("optimized message sent to kafka",
		zap.String("topic", p.topic),
		zap.Int32("partition", partition),
		zap.Int64("offset", offset),
		zap.String("device", deviceName),
		zap.Int("point_count", len(valuesMap)))

	return nil
}

// Stop 停止生产者
func (p *Producer) Stop() error {
	// 发送停止信号
	close(p.stopCh)

	// 等待异步发送器完成
	p.wg.Wait()

	// 关闭生产者
	if p.producer != nil {
		if err := p.producer.Close(); err != nil {
			p.logger.Error("failed to close kafka producer",
				zap.Error(err))
			return fmt.Errorf("failed to close producer: %w", err)
		}
	}

	p.logger.Info("kafka producer stopped")
	return nil
}

// GetTopic 获取当前主题
func (p *Producer) GetTopic() string {
	return p.topic
}
