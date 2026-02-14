package config

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

// ByteOrder 字节序枚举
type ByteOrder string

const (
	ByteOrderBigEndian    ByteOrder = "big"    // 大端序
	ByteOrderLittleEndian ByteOrder = "little" // 小端序
	ByteOrderMiddleEndian ByteOrder = "middle" // 中端序（PDP-11）
	ByteOrderSwap         ByteOrder = "swap"   // 交换字节序（ABCD -> CDAB）
)

// Config 主配置结构
type Config struct {
	Modbus  ModbusConfig  `toml:"modbus"`
	Kafka   KafkaConfig   `toml:"kafka"`
	Logger  LoggerConfig  `toml:"logger"`
	Service ServiceConfig `toml:"service"`
	HTTP    HTTPConfig    `toml:"http"`
}

// ModbusConfig Modbus配置
type ModbusConfig struct {
	Devices []ModbusDevice `toml:"devices"`
}

// ModbusDevice Modbus设备配置
type ModbusDevice struct {
	Name       string    `toml:"name"`
	Host       string    `toml:"host"`
	Port       int       `toml:"port"`
	SlaveID    byte      `toml:"slave_id"`
	Timeout    int       `toml:"timeout"`  // 超时时间（秒）
	Interval   int       `toml:"interval"` // 采集间隔（秒）
	PointsFile string    `toml:"points_file"`
	ByteOrder  ByteOrder `toml:"byte_order"` // 字节序（默认大端序）
}

// KafkaConfig Kafka配置
type KafkaConfig struct {
	Enable         bool     `toml:"enable"` // 是否启用Kafka
	Brokers        []string `toml:"brokers"`
	Topic          string   `toml:"topic"`
	Compression    string   `toml:"compression"` // none, gzip, snappy, lz4, zstd
	BatchSize      int      `toml:"batch_size"`
	FlushFrequency int      `toml:"flush_frequency"` // 毫秒
}

// LoggerConfig 日志配置
type LoggerConfig struct {
	Level      string `toml:"level"`       // debug, info, warn, error
	Output     string `toml:"output"`      // stdout, file
	Filename   string `toml:"filename"`    // 日志文件名
	MaxSize    int    `toml:"max_size"`    // 日志文件最大大小（MB）
	MaxBackups int    `toml:"max_backups"` // 保留的旧日志文件数量
	MaxAge     int    `toml:"max_age"`     // 保留日志文件的最大天数
	Compress   bool   `toml:"compress"`    // 是否压缩旧日志文件
}

// ServiceConfig 服务配置
type ServiceConfig struct {
	Name        string `toml:"name"`
	DisplayName string `toml:"display_name"`
	Description string `toml:"description"`
}

// HTTPConfig HTTP服务器配置
type HTTPConfig struct {
	Enable bool   `toml:"enable"` // 是否启用HTTP服务器
	Addr   string `toml:"addr"`   // HTTP服务器地址
}

// LoadConfig 加载配置文件
func LoadConfig(configFile string) (*Config, error) {
	config := &Config{}

	if _, err := toml.DecodeFile(configFile, config); err != nil {
		return nil, fmt.Errorf("failed to decode config file: %w", err)
	}

	// 验证配置
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return config, nil
}

// Validate 验证配置
func (c *Config) Validate() error {
	// 验证Modbus配置
	if len(c.Modbus.Devices) == 0 {
		return fmt.Errorf("at least one modbus device must be configured")
	}

	for i, device := range c.Modbus.Devices {
		if device.Name == "" {
			return fmt.Errorf("modbus device %d: name cannot be empty", i)
		}
		if device.Host == "" {
			return fmt.Errorf("modbus device %s: host cannot be empty", device.Name)
		}
		if device.Port <= 0 || device.Port > 65535 {
			return fmt.Errorf("modbus device %s: invalid port %d", device.Name, device.Port)
		}
		if device.PointsFile == "" {
			return fmt.Errorf("modbus device %s: points_file cannot be empty", device.Name)
		}
		if device.Timeout <= 0 {
			device.Timeout = 5 // 默认5秒
		}
		if device.Interval <= 0 {
			device.Interval = 10 // 默认10秒
		}
	}

	// 验证Kafka配置（仅当启用时）
	if c.Kafka.Enable {
		if len(c.Kafka.Brokers) == 0 {
			return fmt.Errorf("at least one kafka broker must be configured")
		}
		if c.Kafka.Topic == "" {
			return fmt.Errorf("kafka topic cannot be empty")
		}
		if c.Kafka.BatchSize <= 0 {
			c.Kafka.BatchSize = 100 // 默认100
		}
		if c.Kafka.FlushFrequency <= 0 {
			c.Kafka.FlushFrequency = 100 // 默认100ms
		}
	}

	// 验证日志配置
	if c.Logger.Level == "" {
		c.Logger.Level = "info" // 默认info
	}
	if c.Logger.Output == "" {
		c.Logger.Output = "stdout" // 默认stdout
	}
	if c.Logger.MaxSize <= 0 {
		c.Logger.MaxSize = 100 // 默认100MB
	}
	if c.Logger.MaxBackups <= 0 {
		c.Logger.MaxBackups = 3 // 默认保留3个
	}
	if c.Logger.MaxAge <= 0 {
		c.Logger.MaxAge = 28 // 默认28天
	}

	// 验证服务配置
	if c.Service.Name == "" {
		c.Service.Name = "mbsscaner"
	}
	if c.Service.DisplayName == "" {
		c.Service.DisplayName = "MBS Scanner"
	}
	if c.Service.Description == "" {
		c.Service.Description = "Modbus Data Scanner Service"
	}

	return nil
}

// Exists 检查文件是否存在
func Exists(filename string) bool {
	_, err := os.Stat(filename)
	return !os.IsNotExist(err)
}
