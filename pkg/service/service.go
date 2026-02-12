package service

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/judwhite/go-svc"
	"go.uber.org/zap"
	"mbsscaner/config"
	"mbsscaner/pkg/kafka"
	"mbsscaner/pkg/logger"
	"mbsscaner/pkg/modbus"
)

// ModbusService Modbus数据采集服务
type ModbusService struct {
	config     *config.Config
	logger     *zap.Logger
	kafkaProd  *kafka.Producer
	collectors map[string]*modbus.Collector
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	shutdown   chan struct{}
}

// NewService 创建新的服务实例
func NewService(configFile string) (*ModbusService, error) {
	// 加载配置
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// 创建日志记录器
	log, err := logger.NewLogger(cfg.Logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create logger: %w", err)
	}

	service := &ModbusService{
		config:     cfg,
		logger:     log,
		collectors: make(map[string]*modbus.Collector),
		shutdown:   make(chan struct{}),
	}

	return service, nil
}

// Init 实现svc.Service接口
func (s *ModbusService) Init(env svc.Environment) error {
	s.logger.Info("Initializing Modbus Scanner Service")

	// 创建Kafka生产者（可选，失败不影响Modbus功能）
	prod, err := kafka.NewProducer(s.config.Kafka, s.logger)
	if err != nil {
		s.logger.Warn("failed to create kafka producer, continuing without kafka",
			zap.Error(err))
		// 不返回错误，允许继续使用Modbus功能
	} else {
		s.kafkaProd = prod
	}

	// 创建Modbus采集器
	for _, device := range s.config.Modbus.Devices {
		collector, err := modbus.NewCollector(device, s.logger)
		if err != nil {
			s.logger.Error("failed to create collector for device",
				zap.String("device", device.Name),
				zap.Error(err))
			continue
		}
		s.collectors[device.Name] = collector

		// 加载点位配置
		points, err := config.LoadPoints(device.PointsFile)
		if err != nil {
			s.logger.Error("failed to load points for device",
				zap.String("device", device.Name),
				zap.String("file", device.PointsFile),
				zap.Error(err))
			continue
		}

		// 验证点位配置
		if err := config.ValidatePoints(points); err != nil {
			s.logger.Error("invalid points configuration for device",
				zap.String("device", device.Name),
				zap.Error(err))
			continue
		}

		s.logger.Info("loaded configuration for device",
			zap.String("device", device.Name),
			zap.String("host", device.Host),
			zap.Int("port", device.Port),
			zap.Int("points_count", len(points)))
	}

	if len(s.collectors) == 0 {
		return fmt.Errorf("no valid collectors initialized")
	}

	s.logger.Info("service initialization completed",
		zap.Int("collectors_count", len(s.collectors)))

	return nil
}

// Start 实现svc.Service接口
func (s *ModbusService) Start() error {
	s.logger.Info("Starting Modbus Scanner Service")

	// 创建上下文
	s.ctx, s.cancel = context.WithCancel(context.Background())

	// 启动数据采集协程
	for _, device := range s.config.Modbus.Devices {
		collector, exists := s.collectors[device.Name]
		if !exists {
			continue
		}

		// 加载点位配置
		points, err := config.LoadPoints(device.PointsFile)
		if err != nil {
			s.logger.Error("failed to load points for device",
				zap.String("device", device.Name),
				zap.Error(err))
			continue
		}

		// 启动采集协程
		s.wg.Add(1)
		go s.startCollector(device.Name, collector, points, device.Interval)
	}

	s.logger.Info("Modbus Scanner Service started successfully")
	return nil
}

// Stop 实现svc.Service接口
func (s *ModbusService) Stop() error {
	s.logger.Info("Stopping Modbus Scanner Service")

	// 取消上下文
	if s.cancel != nil {
		s.cancel()
	}

	// 等待所有协程完成
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	// 等待协程完成或超时
	select {
	case <-done:
		s.logger.Info("All goroutines stopped gracefully")
	case <-time.After(30 * time.Second):
		s.logger.Warn("Timeout waiting for goroutines to stop")
	}

	// 关闭采集器
	for name, collector := range s.collectors {
		if err := collector.Close(); err != nil {
			s.logger.Error("failed to close collector",
				zap.String("device", name),
				zap.Error(err))
		}
	}

	// 关闭Kafka生产者
	if s.kafkaProd != nil {
		if err := s.kafkaProd.Stop(); err != nil {
			s.logger.Error("failed to stop kafka producer", zap.Error(err))
		}
	}

	// 关闭日志记录器
	if s.logger != nil {
		s.logger.Sync()
	}

	s.logger.Info("Modbus Scanner Service stopped")
	return nil
}

// startCollector 启动单个采集器
func (s *ModbusService) startCollector(deviceName string, collector *modbus.Collector, points []config.Point, interval int) {
	defer s.wg.Done()

	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	s.logger.Info("collector started",
		zap.String("device", deviceName),
		zap.Int("interval", interval),
		zap.Int("points_count", len(points)))

	for {
		select {
		case <-s.ctx.Done():
			s.logger.Info("collector stopping due to context cancellation",
				zap.String("device", deviceName))
			return

		case <-ticker.C:
			s.collectDeviceData(deviceName, collector, points)
		}
	}
}

// collectDeviceData 采集设备数据
func (s *ModbusService) collectDeviceData(deviceName string, collector *modbus.Collector, points []config.Point) {
	startTime := time.Now()

	// 采集数据
	values, err := collector.CollectPoints(points)
	if err != nil {
		s.logger.Error("failed to collect data from device",
			zap.String("device", deviceName),
			zap.Error(err))
		return
	}

	// 定期输出连接状态（每100次采集输出一次）
	static := collector.GetConnectionStats()
	if connected, ok := static["connected"].(bool); ok && !connected {
		s.logger.Warn("device is disconnected",
			zap.String("device", deviceName))
	}

	// 发送到Kafka
	if s.kafkaProd != nil {
		if err := s.kafkaProd.SendPointValues(deviceName, values); err != nil {
			s.logger.Error("failed to send data to kafka",
				zap.String("device", deviceName),
				zap.Error(err))
		}
	}

	// 统计信息
	successCount := 0
	for _, value := range values {
		if value.Quality == "GOOD" {
			successCount++
		}
	}

	duration := time.Since(startTime)
	s.logger.Debug("data collection completed",
		zap.String("device", deviceName),
		zap.Int("total_points", len(values)),
		zap.Int("success_points", successCount),
		zap.Duration("duration", duration))
}

// RunAsService 以服务模式运行
func RunAsService(configFile string) error {
	svc := &ModbusService{}
	service, err := NewService(configFile)
	if err != nil {
		return fmt.Errorf("failed to create service: %w", err)
	}
	*svc = *service

	// 运行服务
	if err := svc.Run(); err != nil {
		return fmt.Errorf("service failed: %w", err)
	}

	return nil
}

// Run 运行服务
func (s *ModbusService) Run() error {
	// 设置信号处理
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 初始化服务（创建Environment）
	var env svc.Environment
	if err := s.Init(env); err != nil {
		return err
	}

	// 启动服务
	if err := s.Start(); err != nil {
		return err
	}

	// 等待停止信号
	select {
	case sig := <-sigChan:
		s.logger.Info("received signal, stopping service", zap.String("signal", sig.String()))
	case <-s.shutdown:
		s.logger.Info("shutdown signal received")
	}

	// 停止服务
	return s.Stop()
}
