package modbus

import (
	"encoding/binary"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/simonvetter/modbus"
	"go.uber.org/zap"
	"mbsscaner/config"
)

// Collector Modbus数据采集器
type Collector struct {
	client      *modbus.ModbusClient
	logger      *zap.Logger
	deviceName  string
	device      config.ModbusDevice
	connected   bool
	stateMutex  sync.RWMutex
	opMutex     sync.Mutex
	lastConnect time.Time
	retryCount  int
	maxRetries  int
	retryDelay  time.Duration
}

// PointValue 点位数据值
type PointValue struct {
	Name        string      `json:"name"`
	Value       interface{} `json:"value"`
	DataType    string      `json:"data_type"`
	Unit        string      `json:"unit"`
	Description string      `json:"description"`
	Timestamp   time.Time   `json:"timestamp"`
	Quality     string      `json:"quality"`
	Device      string      `json:"device"`
}

// NewCollector 创建新的Modbus采集器
func NewCollector(device config.ModbusDevice, logger *zap.Logger) (*Collector, error) {
	client, err := modbus.NewClient(&modbus.ClientConfiguration{
		URL:     fmt.Sprintf("tcp://%s:%d", device.Host, device.Port),
		Timeout: time.Duration(device.Timeout) * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create modbus client: %w", err)
	}

	collector := &Collector{
		client:     client,
		logger:     logger,
		deviceName: device.Name,
		device:     device,
		connected:  false,
		maxRetries: 3,
		retryDelay: 5 * time.Second,
	}

	// 尝试建立连接
	if err := collector.Connect(); err != nil {
		logger.Warn("failed to connect during initialization, will retry later",
			zap.String("device", device.Name),
			zap.Error(err))
		// 不返回错误，允许在后台重连
	}

	return collector, nil
}

// Connect 建立连接
func (c *Collector) Connect() error {
	c.stateMutex.Lock()
	defer c.stateMutex.Unlock()

	if c.connected {
		return nil
	}

	c.logger.Info("connecting to modbus device",
		zap.String("device", c.deviceName),
		zap.String("host", c.device.Host),
		zap.Int("port", c.device.Port))

	if err := c.client.Open(); err != nil {
		c.retryCount++
		c.logger.Error("failed to open modbus connection",
			zap.String("device", c.deviceName),
			zap.Int("retry", c.retryCount),
			zap.Error(err))
		return fmt.Errorf("failed to open connection: %w", err)
	}

	// 测试连接
	if err := c.testConnection(); err != nil {
		c.client.Close()
		c.retryCount++
		c.logger.Error("connection test failed",
			zap.String("device", c.deviceName),
			zap.Int("retry", c.retryCount),
			zap.Error(err))
		return fmt.Errorf("connection test failed: %w", err)
	}

	c.connected = true
	c.retryCount = 0
	c.lastConnect = time.Now()

	c.logger.Info("successfully connected to modbus device",
		zap.String("device", c.deviceName))

	return nil
}

// Disconnect 断开连接
func (c *Collector) Disconnect() {
	c.stateMutex.Lock()
	defer c.stateMutex.Unlock()

	if !c.connected {
		return
	}

	c.logger.Info("disconnecting from modbus device",
		zap.String("device", c.deviceName))

	if err := c.client.Close(); err != nil {
		c.logger.Error("error closing connection",
			zap.String("device", c.deviceName),
			zap.Error(err))
	}

	c.connected = false
}

// IsConnected 检查连接状态
func (c *Collector) IsConnected() bool {
	c.stateMutex.RLock()
	defer c.stateMutex.RUnlock()
	return c.connected
}

// Reconnect 重连
func (c *Collector) Reconnect() error {
	c.Disconnect()
	return c.Connect()
}

// testConnection 测试连接（内部方法，不加锁）
func (c *Collector) testConnection() error {
	_, err := c.client.ReadRegister(0, modbus.HOLDING_REGISTER)
	return err
}

// ensureConnected 确保连接可用，支持断线重连
func (c *Collector) ensureConnected() error {
	if c.IsConnected() {
		// 连接看起来正常，但实际测试一下
		if err := c.testConnection(); err == nil {
			return nil
		}
		// 连接已断开，标记为未连接
		c.stateMutex.Lock()
		c.connected = false
		c.stateMutex.Unlock()
		c.logger.Warn("connection lost, attempting to reconnect",
			zap.String("device", c.deviceName))
	}

	// 尝试重连
	var lastErr error
	for i := 0; i < c.maxRetries; i++ {
		err := c.Connect()
		if err == nil {
			return nil
		}
		lastErr = err

		if i < c.maxRetries-1 {
			c.logger.Info("retrying connection",
				zap.String("device", c.deviceName),
				zap.Int("attempt", i+1),
				zap.Int("max_retries", c.maxRetries),
				zap.Duration("delay", c.retryDelay))
			time.Sleep(c.retryDelay)
		}
	}

	return fmt.Errorf("failed to reconnect after %d attempts: %w", c.maxRetries, lastErr)
}

// CollectPoints 采集多个点位数据（使用长连接和批量读取）
func (c *Collector) CollectPoints(points []config.Point) ([]PointValue, error) {
	var values []PointValue

	// 确保连接可用
	if err := c.ensureConnected(); err != nil {
		c.logger.Error("failed to establish connection",
			zap.String("device", c.deviceName),
			zap.Error(err))

		// 返回所有点位为BAD状态
		for _, point := range points {
			values = append(values, PointValue{
				Name:        point.Name,
				Value:       nil,
				DataType:    string(point.DataType),
				Unit:        point.Unit,
				Description: point.Description,
				Timestamp:   time.Now(),
				Quality:     "BAD",
				Device:      c.deviceName,
			})
		}
		return values, nil
	}

	// 加锁保护客户端操作
	c.opMutex.Lock()
	defer c.opMutex.Unlock()

	// 按寄存器类型和连续地址对点位进行分组
	batchGroups := c.groupPointsByRegisterType(points)

	c.logger.Debug("optimized batch collection",
		zap.String("device", c.deviceName),
		zap.Int("total_points", len(points)),
		zap.Int("batch_groups", len(batchGroups)))

	// 批量采集每组数据
	for _, group := range batchGroups {
		groupValues, err := c.collectBatchGroup(group)
		if err != nil {
			c.logger.Error("failed to collect batch group",
				zap.String("device", c.deviceName),
				zap.String("register_type", string(group.RegisterType)),
				zap.Uint16("start_address", group.StartAddress),
				zap.Error(err))

			// 为组内所有点位添加错误状态
			for _, point := range group.Points {
				values = append(values, PointValue{
					Name:        point.Name,
					Value:       nil,
					DataType:    string(point.DataType),
					Unit:        point.Unit,
					Description: point.Description,
					Timestamp:   time.Now(),
					Quality:     "BAD",
					Device:      c.deviceName,
				})
			}
			continue
		}

		values = append(values, groupValues...)
	}

	return values, nil
}

// BatchGroup 批量读取组
type BatchGroup struct {
	RegisterType config.RegisterType
	StartAddress uint16
	Points       []config.Point
	TotalLength  uint16
}

// groupPointsByRegisterType 按寄存器类型和连续地址对点位进行分组
func (c *Collector) groupPointsByRegisterType(points []config.Point) []BatchGroup {
	var groups []BatchGroup

	for _, point := range points {
		// 尝试与现有组合并
		merged := false
		for i := range groups {
			if groups[i].RegisterType == point.RegisterType {
				// 检查是否可以连续（新点位紧接着现有组）
				lastPoint := groups[i].Points[len(groups[i].Points)-1]
				if lastPoint.Address+lastPoint.Length == point.Address {
					// 可以连续，添加到现有组
					groups[i].Points = append(groups[i].Points, point)
					groups[i].TotalLength += point.Length
					merged = true
					break
				}
			}
		}

		if !merged {
			// 创建新的组
			newGroup := BatchGroup{
				RegisterType: point.RegisterType,
				StartAddress: point.Address,
				Points:       []config.Point{point},
				TotalLength:  point.Length,
			}
			groups = append(groups, newGroup)
		}
	}

	// 对组内的点位按地址排序
	for i := range groups {
		sortPointsByAddress(groups[i].Points)
	}

	return groups
}

// getModbusRegisterType 将配置的寄存器类型转换为modbus库的类型
func (c *Collector) getModbusRegisterType(registerType config.RegisterType) (modbus.RegType, error) {
	// 注意：当前使用的modbus库主要支持保持寄存器(0x03)
	// 对于其他寄存器类型，需要后续扩展或使用其他库
	switch registerType {
	case config.RegisterTypeInputRegister:
		return modbus.INPUT_REGISTER, nil
	case config.RegisterTypeHoldingRegister:
		return modbus.HOLDING_REGISTER, nil
	default:
		return modbus.HOLDING_REGISTER, fmt.Errorf("unsupported register type: %s", registerType)
	}
}

// sortPointsByAddress 按地址对点位排序
func sortPointsByAddress(points []config.Point) {
	for i := 0; i < len(points)-1; i++ {
		for j := i + 1; j < len(points); j++ {
			if points[i].Address > points[j].Address {
				points[i], points[j] = points[j], points[i]
			}
		}
	}
}

// readBatch 批量读取寄存器数据
func (c *Collector) readBatch(group BatchGroup) ([]uint16, error) {
	switch group.RegisterType {
	case config.RegisterTypeCoil:
		return c.readCoilBatch(group)
	case config.RegisterTypeDiscreteInput:
		return c.readDiscreteInputBatch(group)
	case config.RegisterTypeInputRegister:
		return c.readInputRegisterBatch(group)
	case config.RegisterTypeHoldingRegister:
		return c.readHoldingRegisterBatch(group)
	default:
		return c.readHoldingRegisterBatch(group) // 默认使用保持寄存器
	}
}

// readCoilBatch 批量读取线圈寄存器
func (c *Collector) readCoilBatch(group BatchGroup) ([]uint16, error) {
	c.logger.Debug("reading coil batch using specialized ReadCoils method",
		zap.String("device", c.deviceName),
		zap.Uint16("start_address", group.StartAddress),
		zap.Uint16("total_length", group.TotalLength))

	// 检查Modbus协议限制 - 线圈最大读取2000个点
	const maxCoilRead = 2000
	if group.TotalLength > maxCoilRead {
		c.logger.Warn("coil batch size exceeds Modbus protocol limit, splitting into smaller batches",
			zap.String("device", c.deviceName),
			zap.Uint16("requested_length", group.TotalLength),
			zap.Int("max_allowed", maxCoilRead))

		// 分批读取
		return c.readCoilBatchSplit(group, maxCoilRead)
	}

	// 使用专门的线圈读取方法
	results, err := c.client.ReadCoils(group.StartAddress, group.TotalLength)
	if err != nil {
		c.logger.Warn("ReadCoils failed, falling back to individual reads",
			zap.String("device", c.deviceName),
			zap.Uint16("start_address", group.StartAddress),
			zap.Uint16("total_length", group.TotalLength),
			zap.Error(err))
		return c.readBatchIndividual(group)
	}

	// 将bool结果转换为uint16
	regs := make([]uint16, len(results))
	for i, result := range results {
		if result {
			regs[i] = 1
		} else {
			regs[i] = 0
		}
	}

	c.logger.Debug("successful coil batch read using ReadCoils",
		zap.String("device", c.deviceName),
		zap.Uint16("start_address", group.StartAddress),
		zap.Uint16("total_length", group.TotalLength),
		zap.Int("coils_read", len(results)))

	return regs, nil
}

// readCoilBatchSplit 分批读取线圈（处理超过2000限制）
func (c *Collector) readCoilBatchSplit(group BatchGroup, maxRead uint16) ([]uint16, error) {
	var allRegs []uint16
	startAddress := group.StartAddress
	remainingLength := group.TotalLength

	for remainingLength > 0 {
		// 计算本次读取长度
		batchLength := remainingLength
		if batchLength > maxRead {
			batchLength = maxRead
		}

		c.logger.Debug("reading coil sub-batch",
			zap.String("device", c.deviceName),
			zap.Uint16("start_address", startAddress),
			zap.Uint16("batch_length", batchLength),
			zap.Uint16("remaining", remainingLength))

		// 读取子批次
		results, err := c.client.ReadCoils(startAddress, batchLength)
		if err != nil {
			c.logger.Error("sub-batch ReadCoils failed",
				zap.String("device", c.deviceName),
				zap.Uint16("start_address", startAddress),
				zap.Uint16("batch_length", batchLength),
				zap.Error(err))
			return allRegs, err
		}

		// 将bool结果转换为uint16并添加到结果集
		batchRegs := make([]uint16, len(results))
		for i, result := range results {
			if result {
				batchRegs[i] = 1
			} else {
				batchRegs[i] = 0
			}
		}
		allRegs = append(allRegs, batchRegs...)

		// 更新剩余参数
		startAddress += batchLength
		remainingLength -= batchLength
	}

	c.logger.Debug("successful split coil batch read",
		zap.String("device", c.deviceName),
		zap.Uint16("total_requested", group.TotalLength),
		zap.Int("total_read", len(allRegs)))

	return allRegs, nil
}

// readDiscreteInputBatch 批量读取离散输入寄存器
func (c *Collector) readDiscreteInputBatch(group BatchGroup) ([]uint16, error) {
	c.logger.Debug("reading discrete input batch using specialized ReadDiscreteInputs method",
		zap.String("device", c.deviceName),
		zap.Uint16("start_address", group.StartAddress),
		zap.Uint16("total_length", group.TotalLength))

	// 检查Modbus协议限制 - 离散输入最大读取2000个点
	const maxDiscreteInputRead = 2000
	if group.TotalLength > maxDiscreteInputRead {
		c.logger.Warn("discrete input batch size exceeds Modbus protocol limit, splitting into smaller batches",
			zap.String("device", c.deviceName),
			zap.Uint16("requested_length", group.TotalLength),
			zap.Int("max_allowed", maxDiscreteInputRead))

		// 分批读取
		return c.readDiscreteInputBatchSplit(group, maxDiscreteInputRead)
	}

	// 使用专门的离散输入读取方法
	results, err := c.client.ReadDiscreteInputs(group.StartAddress, group.TotalLength)
	if err != nil {
		c.logger.Warn("ReadDiscreteInputs failed, falling back to individual reads",
			zap.String("device", c.deviceName),
			zap.Uint16("start_address", group.StartAddress),
			zap.Uint16("total_length", group.TotalLength),
			zap.Error(err))
		return c.readBatchIndividual(group)
	}

	// 将bool结果转换为uint16
	regs := make([]uint16, len(results))
	for i, result := range results {
		if result {
			regs[i] = 1
		} else {
			regs[i] = 0
		}
	}

	c.logger.Debug("successful discrete input batch read using ReadDiscreteInputs",
		zap.String("device", c.deviceName),
		zap.Uint16("start_address", group.StartAddress),
		zap.Uint16("total_length", group.TotalLength),
		zap.Int("discrete_inputs_read", len(results)))

	return regs, nil
}

// readDiscreteInputBatchSplit 分批读取离散输入（处理超过2000限制）
func (c *Collector) readDiscreteInputBatchSplit(group BatchGroup, maxRead uint16) ([]uint16, error) {
	var allRegs []uint16
	startAddress := group.StartAddress
	remainingLength := group.TotalLength

	for remainingLength > 0 {
		// 计算本次读取长度
		batchLength := remainingLength
		if batchLength > maxRead {
			batchLength = maxRead
		}

		c.logger.Debug("reading discrete input sub-batch",
			zap.String("device", c.deviceName),
			zap.Uint16("start_address", startAddress),
			zap.Uint16("batch_length", batchLength),
			zap.Uint16("remaining", remainingLength))

		// 读取子批次
		results, err := c.client.ReadDiscreteInputs(startAddress, batchLength)
		if err != nil {
			c.logger.Error("sub-batch ReadDiscreteInputs failed",
				zap.String("device", c.deviceName),
				zap.Uint16("start_address", startAddress),
				zap.Uint16("batch_length", batchLength),
				zap.Error(err))
			return allRegs, err
		}

		// 将bool结果转换为uint16并添加到结果集
		batchRegs := make([]uint16, len(results))
		for i, result := range results {
			if result {
				batchRegs[i] = 1
			} else {
				batchRegs[i] = 0
			}
		}
		allRegs = append(allRegs, batchRegs...)

		// 更新剩余参数
		startAddress += batchLength
		remainingLength -= batchLength
	}

	c.logger.Debug("successful split discrete input batch read",
		zap.String("device", c.deviceName),
		zap.Uint16("total_requested", group.TotalLength),
		zap.Int("total_read", len(allRegs)))

	return allRegs, nil
}

// readInputRegisterBatch 批量读取输入寄存器
func (c *Collector) readInputRegisterBatch(group BatchGroup) ([]uint16, error) {
	c.logger.Debug("reading input register batch using optimized method",
		zap.String("device", c.deviceName),
		zap.Uint16("start_address", group.StartAddress),
		zap.Uint16("total_length", group.TotalLength))

	// 检查Modbus协议限制 - 输入寄存器通常最大读取125个字（250字节）
	const maxInputRegisterRead = 125
	if group.TotalLength > maxInputRegisterRead {
		c.logger.Warn("input register batch size exceeds Modbus protocol limit, splitting into smaller batches",
			zap.String("device", c.deviceName),
			zap.Uint16("requested_length", group.TotalLength),
			zap.Int("max_allowed", maxInputRegisterRead))

		// 分批读取
		return c.readInputRegisterBatchSplit(group, maxInputRegisterRead)
	}

	// 转换为保持寄存器读取（因为当前库的限制）
	regType, err := c.getModbusRegisterType(config.RegisterTypeInputRegister)
	if err != nil {
		return nil, err
	}

	regs, err := c.client.ReadRegisters(group.StartAddress, group.TotalLength, regType)
	if err != nil {
		c.logger.Warn("input register batch read failed, falling back to individual reads",
			zap.String("device", c.deviceName),
			zap.Uint16("start_address", group.StartAddress),
			zap.Uint16("total_length", group.TotalLength),
			zap.Error(err))
		return c.readBatchIndividual(group)
	}

	c.logger.Debug("successful input register batch read",
		zap.String("device", c.deviceName),
		zap.Uint16("start_address", group.StartAddress),
		zap.Uint16("total_length", group.TotalLength),
		zap.Int("registers_read", len(regs)))

	return regs, nil
}

// readInputRegisterBatchSplit 分批读取输入寄存器（处理超过125限制）
func (c *Collector) readInputRegisterBatchSplit(group BatchGroup, maxRead uint16) ([]uint16, error) {
	var allRegs []uint16
	startAddress := group.StartAddress
	remainingLength := group.TotalLength

	for remainingLength > 0 {
		// 计算本次读取长度
		batchLength := remainingLength
		if batchLength > maxRead {
			batchLength = maxRead
		}

		c.logger.Debug("reading input register sub-batch",
			zap.String("device", c.deviceName),
			zap.Uint16("start_address", startAddress),
			zap.Uint16("batch_length", batchLength),
			zap.Uint16("remaining", remainingLength))

		// 读取子批次
		regType, err := c.getModbusRegisterType(config.RegisterTypeInputRegister)
		if err != nil {
			return allRegs, err
		}

		batchRegs, err := c.client.ReadRegisters(startAddress, batchLength, regType)
		if err != nil {
			c.logger.Error("sub-batch input register read failed",
				zap.String("device", c.deviceName),
				zap.Uint16("start_address", startAddress),
				zap.Uint16("batch_length", batchLength),
				zap.Error(err))
			return allRegs, err
		}

		allRegs = append(allRegs, batchRegs...)

		// 更新剩余参数
		startAddress += batchLength
		remainingLength -= batchLength
	}

	c.logger.Debug("successful split input register batch read",
		zap.String("device", c.deviceName),
		zap.Uint16("total_requested", group.TotalLength),
		zap.Int("total_read", len(allRegs)))

	return allRegs, nil
}

// readHoldingRegisterBatch 批量读取保持寄存器
func (c *Collector) readHoldingRegisterBatch(group BatchGroup) ([]uint16, error) {
	c.logger.Debug("reading holding register batch using optimized method",
		zap.String("device", c.deviceName),
		zap.Uint16("start_address", group.StartAddress),
		zap.Uint16("total_length", group.TotalLength))

	// 检查Modbus协议限制 - 保持寄存器最大读取123个寄存器（246字节）
	const maxHoldingRegisterRead = 123
	if group.TotalLength > maxHoldingRegisterRead {
		c.logger.Warn("holding register batch size exceeds Modbus protocol limit, splitting into smaller batches",
			zap.String("device", c.deviceName),
			zap.Uint16("requested_length", group.TotalLength),
			zap.Int("max_allowed", maxHoldingRegisterRead))

		// 分批读取
		return c.readHoldingRegisterBatchSplit(group, maxHoldingRegisterRead)
	}

	// 使用保持寄存器的批量读取
	regType, err := c.getModbusRegisterType(config.RegisterTypeHoldingRegister)
	if err != nil {
		return nil, err
	}

	regs, err := c.client.ReadRegisters(group.StartAddress, group.TotalLength, regType)
	if err != nil {
		c.logger.Warn("holding register batch read failed, falling back to individual reads",
			zap.String("device", c.deviceName),
			zap.Uint16("start_address", group.StartAddress),
			zap.Uint16("total_length", group.TotalLength),
			zap.Error(err))
		return c.readBatchIndividual(group)
	}

	c.logger.Debug("successful holding register batch read",
		zap.String("device", c.deviceName),
		zap.Uint16("start_address", group.StartAddress),
		zap.Uint16("total_length", group.TotalLength),
		zap.Int("registers_read", len(regs)))

	return regs, nil
}

// readHoldingRegisterBatchSplit 分批读取保持寄存器（处理超过125限制）
func (c *Collector) readHoldingRegisterBatchSplit(group BatchGroup, maxRead uint16) ([]uint16, error) {
	var allRegs []uint16
	startAddress := group.StartAddress
	remainingLength := group.TotalLength

	for remainingLength > 0 {
		// 计算本次读取长度
		batchLength := remainingLength
		if batchLength > maxRead {
			batchLength = maxRead
		}

		c.logger.Debug("reading holding register sub-batch",
			zap.String("device", c.deviceName),
			zap.Uint16("start_address", startAddress),
			zap.Uint16("batch_length", batchLength),
			zap.Uint16("remaining", remainingLength))

		// 读取子批次
		regType, err := c.getModbusRegisterType(config.RegisterTypeHoldingRegister)
		if err != nil {
			return allRegs, err
		}

		batchRegs, err := c.client.ReadRegisters(startAddress, batchLength, regType)
		if err != nil {
			c.logger.Error("sub-batch holding register read failed",
				zap.String("device", c.deviceName),
				zap.Uint16("start_address", startAddress),
				zap.Uint16("batch_length", batchLength),
				zap.Error(err))
			return allRegs, err
		}

		allRegs = append(allRegs, batchRegs...)

		// 更新剩余参数
		startAddress += batchLength
		remainingLength -= batchLength
	}

	c.logger.Debug("successful split holding register batch read",
		zap.String("device", c.deviceName),
		zap.Uint16("total_requested", group.TotalLength),
		zap.Int("total_read", len(allRegs)))

	return allRegs, nil
}

// readBatchIndividual 降级到逐个读取
func (c *Collector) readBatchIndividual(group BatchGroup) ([]uint16, error) {
	regType, err := c.getModbusRegisterType(group.RegisterType)
	if err != nil {
		return nil, err
	}

	regs := make([]uint16, 0, group.TotalLength)

	for _, point := range group.Points {
		for i := uint16(0); i < point.Length; i++ {
			value, err := c.client.ReadRegister(point.Address+i, regType)
			if err != nil {
				c.logger.Error("individual read failed",
					zap.String("device", c.deviceName),
					zap.Uint16("address", point.Address+i),
					zap.String("register_type", string(group.RegisterType)),
					zap.Error(err))
				return regs, err
			}
			regs = append(regs, value)
		}
	}

	return regs, nil
}

// collectBatchGroup 采集批量组数据
func (c *Collector) collectBatchGroup(group BatchGroup) ([]PointValue, error) {
	var values []PointValue

	// 批量读取寄存器数据
	registers, err := c.readBatch(group)
	if err != nil {
		c.logger.Warn("failed to read batch group",
			zap.String("device", c.deviceName),
			zap.String("register_type", string(group.RegisterType)),
			zap.Uint16("start_address", group.StartAddress),
			zap.Uint16("total_length", group.TotalLength),
			zap.Error(err))

		// 返回所有点位为BAD状态
		for _, point := range group.Points {
			values = append(values, PointValue{
				Name:        point.Name,
				Value:       nil,
				DataType:    string(point.DataType),
				Unit:        point.Unit,
				Description: point.Description,
				Timestamp:   time.Now(),
				Quality:     "BAD",
				Device:      c.deviceName,
			})
		}
		return values, nil
	}

	// 解析批量数据
	regOffset := 0
	for _, point := range group.Points {
		// 从批量数据中提取此点位的数据
		pointRegisters := registers[regOffset : regOffset+int(point.Length)]
		value, err := c.parseRegistersForPoint(point, pointRegisters)
		if err != nil {
			c.logger.Warn("failed to parse point from batch data",
				zap.String("device", c.deviceName),
				zap.String("point", point.Name),
				zap.Uint16("address", point.Address),
				zap.Error(err))

			// 添加错误状态的数据
			values = append(values, PointValue{
				Name:        point.Name,
				Value:       nil,
				DataType:    string(point.DataType),
				Unit:        point.Unit,
				Description: point.Description,
				Timestamp:   time.Now(),
				Quality:     "BAD",
				Device:      c.deviceName,
			})
		} else {
			// 应用缩放
			if point.Scale != 1.0 {
				switch v := value.(type) {
				case float64:
					value = v * point.Scale
				case float32:
					value = float32(float64(v) * point.Scale)
				case int16:
					value = int16(float64(v) * point.Scale)
				case uint16:
					value = uint16(float64(v) * point.Scale)
				case int32:
					value = int32(float64(v) * point.Scale)
				case uint32:
					value = uint32(float64(v) * point.Scale)
				}
			}

			values = append(values, PointValue{
				Name:        point.Name,
				Value:       value,
				DataType:    string(point.DataType),
				Unit:        point.Unit,
				Description: point.Description,
				Timestamp:   time.Now(),
				Quality:     "GOOD",
				Device:      c.deviceName,
			})
		}
		regOffset += int(point.Length)
	}

	return values, nil
}

// parseRegistersForPoint 从寄存器数据中解析点位值
func (c *Collector) parseRegistersForPoint(point config.Point, registers []uint16) (interface{}, error) {
	if len(registers) < int(point.Length) {
		return nil, fmt.Errorf("insufficient registers for point %s, expected %d, got %d",
			point.Name, point.Length, len(registers))
	}

	// 获取设备级别的字节序配置
	deviceByteOrder := c.device.ByteOrder
	if deviceByteOrder == "" {
		deviceByteOrder = config.ByteOrderBigEndian // 默认大端序
	}

	switch point.DataType {
	case config.DataTypeInt16:
		return int16(registers[0]), nil
	case config.DataTypeUInt16:
		return registers[0], nil
	case config.DataTypeInt32:
		if len(registers) < 2 {
			return nil, fmt.Errorf("insufficient registers for int32")
		}
		bytes := make([]byte, 4)
		c.putBytesWithOrder(bytes, registers, deviceByteOrder)
		return c.getInt32WithOrder(bytes, deviceByteOrder), nil
	case config.DataTypeUInt32:
		if len(registers) < 2 {
			return nil, fmt.Errorf("insufficient registers for uint32")
		}
		bytes := make([]byte, 4)
		c.putBytesWithOrder(bytes, registers, deviceByteOrder)
		return c.getUint32WithOrder(bytes, deviceByteOrder), nil
	case config.DataTypeFloat32:
		if len(registers) < 2 {
			return nil, fmt.Errorf("insufficient registers for float32")
		}
		bytes := make([]byte, 4)
		c.putBytesWithOrder(bytes, registers, deviceByteOrder)
		bits := c.getUint32WithOrder(bytes, deviceByteOrder)
		return math.Float32frombits(bits), nil
	case config.DataTypeFloat64:
		if len(registers) < 4 {
			return nil, fmt.Errorf("insufficient registers for float64")
		}
		bytes := make([]byte, 8)
		c.putFloat64BytesWithOrder(bytes, registers, deviceByteOrder)
		// Little Endian需要用LittleEndian解析
		var bits uint64
		if deviceByteOrder == config.ByteOrderLittleEndian {
			bits = binary.LittleEndian.Uint64(bytes)
		} else {
			bits = binary.BigEndian.Uint64(bytes)
		}
		return math.Float64frombits(bits), nil
	case config.DataTypeBool:
		return registers[0] != 0, nil
	case config.DataTypeString:
		// 将寄存器值转换为字节
		bytes := make([]byte, len(registers)*2)
		for i, reg := range registers {
			binary.BigEndian.PutUint16(bytes[i*2:], reg)
		}
		// 移除空字符
		var result []byte
		for _, b := range bytes {
			if b != 0 {
				result = append(result, b)
			} else {
				break
			}
		}
		return string(result), nil
	default:
		return nil, fmt.Errorf("unsupported data type: %s", point.DataType)
	}
}

// collectSinglePoint 采集单个点位数据
func (c *Collector) collectSinglePoint(point config.Point) (interface{}, error) {
	switch point.DataType {
	case config.DataTypeInt16:
		return c.readInt16(point.Address, point.RegisterType)
	case config.DataTypeUInt16:
		return c.readUInt16(point.Address, point.RegisterType)
	case config.DataTypeInt32:
		return c.readInt32(point.Address, point.RegisterType)
	case config.DataTypeUInt32:
		return c.readUInt32(point.Address, point.RegisterType)
	case config.DataTypeFloat32:
		return c.readFloat32(point.Address, point.RegisterType)
	case config.DataTypeFloat64:
		return c.readFloat64(point.Address, point.RegisterType)
	case config.DataTypeBool:
		return c.readBool(point.Address, point.RegisterType)
	case config.DataTypeString:
		return c.readString(point.Address, point.Length, point.RegisterType)
	default:
		return nil, fmt.Errorf("unsupported data type: %s", point.DataType)
	}
}

// ReadSinglePoint 读取单个点位数据（公开方法）
func (c *Collector) ReadSinglePoint(point config.Point) (PointValue, error) {
	// 确保连接可用
	if err := c.ensureConnected(); err != nil {
		return PointValue{}, fmt.Errorf("failed to connect: %w", err)
	}

	// 加锁保护客户端操作
	c.opMutex.Lock()
	defer c.opMutex.Unlock()

	value, err := c.collectSinglePoint(point)
	if err != nil {
		return PointValue{}, err
	}

	return PointValue{
		Name:        point.Name,
		Value:       value,
		DataType:    string(point.DataType),
		Unit:        point.Unit,
		Description: point.Description,
		Timestamp:   time.Now(),
		Quality:     "GOOD",
		Device:      c.deviceName,
	}, nil
}

// readInt16 读取int16数据
func (c *Collector) readInt16(address uint16, registerType config.RegisterType) (int16, error) {
	regType, err := c.getModbusRegisterType(registerType)
	if err != nil {
		return 0, err
	}
	value, err := c.client.ReadRegister(address, regType)
	if err != nil {
		return 0, err
	}
	return int16(value), nil
}

// readUInt16 读取uint16数据
func (c *Collector) readUInt16(address uint16, registerType config.RegisterType) (uint16, error) {
	regType, err := c.getModbusRegisterType(registerType)
	if err != nil {
		return 0, err
	}
	value, err := c.client.ReadRegister(address, regType)
	if err != nil {
		return 0, err
	}
	return value, nil
}

// readInt32 读取int32数据
func (c *Collector) readInt32(address uint16, registerType config.RegisterType) (int32, error) {
	regType, err := c.getModbusRegisterType(registerType)
	if err != nil {
		return 0, err
	}
	// 批量读取两个连续的寄存器
	registers, err := c.client.ReadRegisters(address, 2, regType)
	if err != nil {
		return 0, err
	}

	// 使用设备配置的字节序
	bytes := make([]byte, 4)
	c.putBytesWithOrder(bytes, registers, c.device.ByteOrder)
	return c.getInt32WithOrder(bytes, c.device.ByteOrder), nil
}

// readUInt32 读取uint32数据
func (c *Collector) readUInt32(address uint16, registerType config.RegisterType) (uint32, error) {
	regType, err := c.getModbusRegisterType(registerType)
	if err != nil {
		return 0, err
	}
	// 批量读取两个连续的寄存器
	registers, err := c.client.ReadRegisters(address, 2, regType)
	if err != nil {
		return 0, err
	}

	// 使用设备配置的字节序
	bytes := make([]byte, 4)
	c.putBytesWithOrder(bytes, registers, c.device.ByteOrder)
	return c.getUint32WithOrder(bytes, c.device.ByteOrder), nil
}

// readFloat32 读取float32数据
func (c *Collector) readFloat32(address uint16, registerType config.RegisterType) (float32, error) {
	regType, err := c.getModbusRegisterType(registerType)
	if err != nil {
		return 0, err
	}
	// 批量读取两个连续的寄存器
	registers, err := c.client.ReadRegisters(address, 2, regType)
	if err != nil {
		return 0, err
	}

	// 使用设备配置的字节序
	bytes := make([]byte, 4)
	c.putBytesWithOrder(bytes, registers, c.device.ByteOrder)
	bits := c.getUint32WithOrder(bytes, c.device.ByteOrder)
	return math.Float32frombits(bits), nil
}

// readFloat64 读取float64数据
func (c *Collector) readFloat64(address uint16, registerType config.RegisterType) (float64, error) {
	regType, err := c.getModbusRegisterType(registerType)
	if err != nil {
		return 0, err
	}
	// 批量读取四个连续的寄存器
	registers, err := c.client.ReadRegisters(address, 4, regType)
	if err != nil {
		return 0, err
	}

	// 使用设备配置的字节序
	bytes := make([]byte, 8)
	c.putFloat64BytesWithOrder(bytes, registers, c.device.ByteOrder)
	// Float64解析需要特殊处理
	var bits uint64
	if c.device.ByteOrder == config.ByteOrderLittleEndian {
		bits = binary.LittleEndian.Uint64(bytes)
	} else {
		bits = binary.BigEndian.Uint64(bytes)
	}
	return math.Float64frombits(bits), nil
}

// readBool 读取bool数据
func (c *Collector) readBool(address uint16, registerType config.RegisterType) (bool, error) {
	regType, err := c.getModbusRegisterType(registerType)
	if err != nil {
		return false, err
	}
	value, err := c.client.ReadRegister(address, regType)
	if err != nil {
		return false, err
	}
	return value != 0, nil
}

// readString 读取字符串数据
func (c *Collector) readString(address uint16, length uint16, registerType config.RegisterType) (string, error) {
	regType, err := c.getModbusRegisterType(registerType)
	if err != nil {
		return "", err
	}
	// 读取多个连续的寄存器
	regs := make([]uint16, length)
	for i := uint16(0); i < length; i++ {
		reg, err := c.client.ReadRegister(address+i, regType)
		if err != nil {
			return "", err
		}
		regs[i] = reg
	}

	// 将寄存器值转换为字节
	bytes := make([]byte, len(regs)*2)
	for i, reg := range regs {
		binary.BigEndian.PutUint16(bytes[i*2:], reg)
	}

	// 移除空字符
	var result []byte
	for _, b := range bytes {
		if b != 0 {
			result = append(result, b)
		} else {
			break // 遇到第一个空字符就停止
		}
	}

	return string(result), nil
}

// Close 关闭采集器
func (c *Collector) Close() error {
	c.Disconnect()
	return nil
}

// GetConnectionStats 获取连接统计信息
func (c *Collector) GetConnectionStats() map[string]interface{} {
	c.stateMutex.RLock()
	defer c.stateMutex.RUnlock()

	return map[string]interface{}{
		"device":       c.deviceName,
		"connected":    c.connected,
		"last_connect": c.lastConnect,
		"retry_count":  c.retryCount,
		"max_retries":  c.maxRetries,
	}
}

// putBytesWithOrder 根据字节序将寄存器值放入字节数组
func (c *Collector) putBytesWithOrder(bytes []byte, registers []uint16, byteOrder config.ByteOrder) {
	if len(registers) < 2 {
		return
	}

	// 原始32位值：假设要表示 0x12345678
	// registers[0] = 0x1234 (可能包含高位或低位)
	// registers[1] = 0x5678 (可能包含低位或高位)

	switch byteOrder {
	case config.ByteOrderBigEndian:
		// 大端序：ABCD - Motorola格式
		// registers[0]是高16位，registers[1]是低16位
		binary.BigEndian.PutUint16(bytes[0:2], registers[0]) // 12 34
		binary.BigEndian.PutUint16(bytes[2:4], registers[1]) // 56 78
		// 结果：12 34 56 78 (ABCD)

	case config.ByteOrderLittleEndian:
		// 小端序：DCBA
		// 根据实际设备反馈，Little Endian设备会：
		// 1. 将32位值的低16位存储在寄存器0，高16位存储在寄存器1
		// 2. 对每个16位寄存器内部进行字节交换

		// 步骤1：对每个寄存器进行字节交换（还原设备的数据变换）
		// 寄存器0存储的是原始低16位的字节交换值
		// 寄存器1存储的是原始高16位的字节交换值
		swappedReg0 := (registers[0] >> 8) | ((registers[0] & 0xFF) << 8) // 还原寄存器0的字节交换
		swappedReg1 := (registers[1] >> 8) | ((registers[1] & 0xFF) << 8) // 还原寄存器1的字节交换

		// 步骤2：按小端序存储还原后的寄存器值
		// 注意：寄存器0是低16位，寄存器1是高16位
		// 所以字节序列应该是：[reg1高字节, reg1低字节, reg0高字节, reg0低字节]
		binary.LittleEndian.PutUint16(bytes[0:2], swappedReg0) // 低16位，小端序
		binary.LittleEndian.PutUint16(bytes[2:4], swappedReg1) // 高16位，小端序
		// 后续用小端序解析，得到正确的32位值

	case config.ByteOrderSwap:
		// 字节交换：BADC - 每个寄存器内字节交换
		// 设备存储时每个16位寄存器内字节是交换的
		// 我们需要反向处理来得到正确的值
		// registers[0]存储的是BA形式的16位值，需要转换成AB
		// registers[1]存储的是DC形式的16位值，需要转换成CD
		swappedReg0 := (registers[0] >> 8) | ((registers[0] & 0xFF) << 8) // BA -> AB
		swappedReg1 := (registers[1] >> 8) | ((registers[1] & 0xFF) << 8) // DC -> CD
		binary.BigEndian.PutUint16(bytes[0:2], swappedReg0)               // AB
		binary.BigEndian.PutUint16(bytes[2:4], swappedReg1)               // CD
		// 结果：12 34 56 78 (ABCD) - 但用swap方式解析

	case config.ByteOrderMiddleEndian:
		// 中端序：CDAB - PDP-11格式
		// 交换两个寄存器的位置
		binary.BigEndian.PutUint16(bytes[0:2], registers[1]) // 56 78
		binary.BigEndian.PutUint16(bytes[2:4], registers[0]) // 12 34
		// 结果：56 78 12 34 (CDAB)
	}
}

// getInt32WithOrder 根据字节序从字节数组读取int32
func (c *Collector) getInt32WithOrder(bytes []byte, byteOrder config.ByteOrder) int32 {
	switch byteOrder {
	case config.ByteOrderBigEndian:
		return int32(binary.BigEndian.Uint32(bytes))
	case config.ByteOrderLittleEndian:
		return int32(binary.LittleEndian.Uint32(bytes))
	case config.ByteOrderSwap, config.ByteOrderMiddleEndian:
		// 对于交换和中端序，我们已经重新排列了字节，使用大端序读取
		return int32(binary.BigEndian.Uint32(bytes))
	default:
		return int32(binary.BigEndian.Uint32(bytes))
	}
}

// getUint32WithOrder 根据字节序从字节数组读取uint32
func (c *Collector) getUint32WithOrder(bytes []byte, byteOrder config.ByteOrder) uint32 {
	switch byteOrder {
	case config.ByteOrderBigEndian:
		return binary.BigEndian.Uint32(bytes)
	case config.ByteOrderLittleEndian:
		return binary.LittleEndian.Uint32(bytes)
	case config.ByteOrderSwap, config.ByteOrderMiddleEndian:
		// 对于交换和中端序，我们已经重新排列了字节，使用大端序读取
		return binary.BigEndian.Uint32(bytes)
	default:
		return binary.BigEndian.Uint32(bytes)
	}
}

// putFloat64BytesWithOrder 根据字节序将float64寄存器值放入字节数组
func (c *Collector) putFloat64BytesWithOrder(bytes []byte, registers []uint16, byteOrder config.ByteOrder) {
	if len(registers) < 4 {
		return
	}

	switch byteOrder {
	case config.ByteOrderBigEndian:
		// 大端序：ABCDEFGH
		binary.BigEndian.PutUint16(bytes[0:2], registers[0])
		binary.BigEndian.PutUint16(bytes[2:4], registers[1])
		binary.BigEndian.PutUint16(bytes[4:6], registers[2])
		binary.BigEndian.PutUint16(bytes[6:8], registers[3])
	case config.ByteOrderLittleEndian:
		// 小端序：HGFEDCBA
		// 对每个寄存器进行字节交换
		swappedReg0 := (registers[0] >> 8) | ((registers[0] & 0xFF) << 8)
		swappedReg1 := (registers[1] >> 8) | ((registers[1] & 0xFF) << 8)
		swappedReg2 := (registers[2] >> 8) | ((registers[2] & 0xFF) << 8)
		swappedReg3 := (registers[3] >> 8) | ((registers[3] & 0xFF) << 8)
		// 按小端序存储
		binary.LittleEndian.PutUint16(bytes[0:2], swappedReg0)
		binary.LittleEndian.PutUint16(bytes[2:4], swappedReg1)
		binary.LittleEndian.PutUint16(bytes[4:6], swappedReg2)
		binary.LittleEndian.PutUint16(bytes[6:8], swappedReg3)
	case config.ByteOrderSwap:
		// 字节交换：每个寄存器内字节交换
		temp0 := make([]byte, 2)
		temp1 := make([]byte, 2)
		temp2 := make([]byte, 2)
		temp3 := make([]byte, 2)
		binary.BigEndian.PutUint16(temp0, registers[0])
		binary.BigEndian.PutUint16(temp1, registers[1])
		binary.BigEndian.PutUint16(temp2, registers[2])
		binary.BigEndian.PutUint16(temp3, registers[3])
		bytes[0], bytes[1] = temp0[1], temp0[0]
		bytes[2], bytes[3] = temp1[1], temp1[0]
		bytes[4], bytes[5] = temp2[1], temp2[0]
		bytes[6], bytes[7] = temp3[1], temp3[0]
	case config.ByteOrderMiddleEndian:
		// 中端序：CDABEFGH
		binary.BigEndian.PutUint16(bytes[0:2], registers[2])
		binary.BigEndian.PutUint16(bytes[2:4], registers[3])
		binary.BigEndian.PutUint16(bytes[4:6], registers[0])
		binary.BigEndian.PutUint16(bytes[6:8], registers[1])
	}
}

// WritePoint 写入点位值到设备
func (c *Collector) WritePoint(point config.Point, value interface{}) error {
	// 确保连接
	if err := c.ensureConnected(); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	// 加锁保护客户端操作
	c.opMutex.Lock()
	defer c.opMutex.Unlock()

	// 根据寄存器类型分别处理
	switch point.RegisterType {
	case config.RegisterTypeCoil:
		return c.writeCoil(point, value)
	case config.RegisterTypeHoldingRegister:
		return c.writeHoldingRegister(point, value)
	default:
		return fmt.Errorf("unsupported register type for write: %s (only Coil and HoldingRegister are writable)", point.RegisterType)
	}
}

// writeCoil 写入线圈
func (c *Collector) writeCoil(point config.Point, value interface{}) error {
	var boolValue bool
	switch v := value.(type) {
	case bool:
		boolValue = v
	case int, int8, int16, int32, int64:
		boolValue = toInt64(v) != 0
	case uint, uint8, uint16, uint32, uint64:
		boolValue = toUint64(v) != 0
	case float32, float64:
		boolValue = toFloat64(v) != 0
	default:
		return fmt.Errorf("unsupported value type for coil: %T", value)
	}

	err := c.client.WriteCoil(point.Address, boolValue)
	if err != nil {
		return fmt.Errorf("failed to write coil: %w", err)
	}

	c.logger.Info("coil written successfully",
		zap.String("device", c.deviceName),
		zap.String("point", point.Name),
		zap.Bool("value", boolValue))

	return nil
}

// writeHoldingRegister 写入保持寄存器
func (c *Collector) writeHoldingRegister(point config.Point, value interface{}) error {
	// 根据数据类型转换值
	var registers []uint16
	var err error

	switch point.DataType {
	case config.DataTypeInt16:
		val := toInt16(value)
		registers = c.int16ToRegisters(val, c.device.ByteOrder)

	case config.DataTypeUInt16:
		val := toUint16(value)
		registers = c.uint16ToRegisters(val, c.device.ByteOrder)

	case config.DataTypeInt32:
		val := toInt32(value)
		registers = c.int32ToRegisters(val, c.device.ByteOrder)

	case config.DataTypeUInt32:
		val := toUint32(value)
		registers = c.uint32ToRegisters(val, c.device.ByteOrder)

	case config.DataTypeFloat32:
		val := toFloat32(value)
		registers = c.float32ToRegisters(val, c.device.ByteOrder)

	case config.DataTypeFloat64:
		val := toFloat64(value)
		registers = c.float64ToRegisters(val, c.device.ByteOrder)

	default:
		return fmt.Errorf("unsupported data type: %s (only int16/uint16/int32/uint32/float32/float64 are writable for holding register)", point.DataType)
	}

	// 写入寄存器
	err = c.client.WriteRegisters(point.Address, registers)
	if err != nil {
		return fmt.Errorf("failed to write register: %w", err)
	}

	c.logger.Info("holding register written successfully",
		zap.String("device", c.deviceName),
		zap.String("point", point.Name),
		zap.Any("value", value))

	return nil
}

// toString 转换为string
func toString(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case []byte:
		return string(val)
	default:
		return fmt.Sprintf("%v", val)
	}
}

// toInt64 转换为int64
func toInt64(v interface{}) int64 {
	switch val := v.(type) {
	case int:
		return int64(val)
	case int8:
		return int64(val)
	case int16:
		return int64(val)
	case int32:
		return int64(val)
	case int64:
		return val
	case uint:
		return int64(val)
	case uint8:
		return int64(val)
	case uint16:
		return int64(val)
	case uint32:
		return int64(val)
	case uint64:
		return int64(val)
	case float32:
		return int64(val)
	case float64:
		return int64(val)
	default:
		return 0
	}
}

// toUint64 转换为uint64
func toUint64(v interface{}) uint64 {
	switch val := v.(type) {
	case int:
		return uint64(val)
	case int8:
		return uint64(val)
	case int16:
		return uint64(val)
	case int32:
		return uint64(val)
	case int64:
		return uint64(val)
	case uint:
		return uint64(val)
	case uint8:
		return uint64(val)
	case uint16:
		return uint64(val)
	case uint32:
		return uint64(val)
	case uint64:
		return val
	case float32:
		return uint64(val)
	case float64:
		return uint64(val)
	default:
		return 0
	}
}

// toInt16 转换为int16
func toInt16(v interface{}) int16 {
	return int16(toInt64(v))
}

// toUint16 转换为uint16
func toUint16(v interface{}) uint16 {
	return uint16(toUint64(v))
}

// toInt32 转换为int32
func toInt32(v interface{}) int32 {
	return int32(toInt64(v))
}

// toUint32 转换为uint32
func toUint32(v interface{}) uint32 {
	return uint32(toUint64(v))
}

// toFloat32 转换为float32
func toFloat32(v interface{}) float32 {
	switch val := v.(type) {
	case float32:
		return val
	case float64:
		return float32(val)
	case int, int8, int16, int32, int64:
		return float32(toInt64(val))
	case uint, uint8, uint16, uint32, uint64:
		return float32(toUint64(val))
	default:
		return 0
	}
}

// toFloat64 转换为float64
func toFloat64(v interface{}) float64 {
	switch val := v.(type) {
	case float32:
		return float64(val)
	case float64:
		return val
	case int, int8, int16, int32, int64:
		return float64(toInt64(val))
	case uint, uint8, uint16, uint32, uint64:
		return float64(toUint64(val))
	default:
		return 0
	}
}

// int32ToRegisters 将int32值转换为寄存器数组（根据字节序）
func (c *Collector) int32ToRegisters(value int32, byteOrder config.ByteOrder) []uint16 {
	bytes := make([]byte, 4)

	switch byteOrder {
	case config.ByteOrderBigEndian:
		binary.BigEndian.PutUint32(bytes, uint32(value))
	case config.ByteOrderLittleEndian:
		binary.BigEndian.PutUint32(bytes, uint32(value))
		regs := []uint16{
			binary.BigEndian.Uint16(bytes[2:4]),
			binary.BigEndian.Uint16(bytes[0:2]),
		}
		regs[0] = (regs[0] >> 8) | ((regs[0] & 0xFF) << 8)
		regs[1] = (regs[1] >> 8) | ((regs[1] & 0xFF) << 8)
		return regs
	case config.ByteOrderSwap:
		binary.BigEndian.PutUint32(bytes, uint32(value))
		bytes = c.swapBytesInRegisters(bytes)
	case config.ByteOrderMiddleEndian:
		binary.BigEndian.PutUint32(bytes, uint32(value))
		bytes = c.swapRegisterPairs(bytes)
	default:
		binary.BigEndian.PutUint32(bytes, uint32(value))
	}

	return []uint16{
		binary.BigEndian.Uint16(bytes[0:2]),
		binary.BigEndian.Uint16(bytes[2:4]),
	}
}

// uint32ToRegisters 将uint32值转换为寄存器数组（根据字节序）
func (c *Collector) uint32ToRegisters(value uint32, byteOrder config.ByteOrder) []uint16 {
	bytes := make([]byte, 4)

	switch byteOrder {
	case config.ByteOrderBigEndian:
		binary.BigEndian.PutUint32(bytes, value)
	case config.ByteOrderLittleEndian:
		binary.BigEndian.PutUint32(bytes, value)
		regs := []uint16{
			binary.BigEndian.Uint16(bytes[2:4]),
			binary.BigEndian.Uint16(bytes[0:2]),
		}
		regs[0] = (regs[0] >> 8) | ((regs[0] & 0xFF) << 8)
		regs[1] = (regs[1] >> 8) | ((regs[1] & 0xFF) << 8)
		return regs
	case config.ByteOrderSwap:
		binary.BigEndian.PutUint32(bytes, value)
		bytes = c.swapBytesInRegisters(bytes)
	case config.ByteOrderMiddleEndian:
		binary.BigEndian.PutUint32(bytes, value)
		bytes = c.swapRegisterPairs(bytes)
	default:
		binary.BigEndian.PutUint32(bytes, value)
	}

	return []uint16{
		binary.BigEndian.Uint16(bytes[0:2]),
		binary.BigEndian.Uint16(bytes[2:4]),
	}
}

// float32ToRegisters 将float32值转换为寄存器数组（根据字节序）
func (c *Collector) float32ToRegisters(value float32, byteOrder config.ByteOrder) []uint16 {
	bits := math.Float32bits(value)
	bytes := make([]byte, 4)

	switch byteOrder {
	case config.ByteOrderBigEndian:
		binary.BigEndian.PutUint32(bytes, bits)
	case config.ByteOrderLittleEndian:
		binary.BigEndian.PutUint32(bytes, bits)
		regs := []uint16{
			binary.BigEndian.Uint16(bytes[2:4]),
			binary.BigEndian.Uint16(bytes[0:2]),
		}
		// 对每个寄存器进行字节交换（Little Endian需要）
		regs[0] = (regs[0] >> 8) | ((regs[0] & 0xFF) << 8)
		regs[1] = (regs[1] >> 8) | ((regs[1] & 0xFF) << 8)
		return regs
	case config.ByteOrderSwap:
		binary.BigEndian.PutUint32(bytes, bits)
		bytes = c.swapBytesInRegisters(bytes)
	case config.ByteOrderMiddleEndian:
		binary.BigEndian.PutUint32(bytes, bits)
		bytes = c.swapRegisterPairs(bytes)
	default:
		binary.BigEndian.PutUint32(bytes, bits)
	}

	return []uint16{
		binary.BigEndian.Uint16(bytes[0:2]),
		binary.BigEndian.Uint16(bytes[2:4]),
	}
}

// float64ToRegisters 将float64值转换为寄存器数组（根据字节序）
func (c *Collector) float64ToRegisters(value float64, byteOrder config.ByteOrder) []uint16 {
	bits := math.Float64bits(value)
	bytes := make([]byte, 8)

	switch byteOrder {
	case config.ByteOrderBigEndian:
		binary.BigEndian.PutUint64(bytes, bits)
	case config.ByteOrderLittleEndian:
		binary.BigEndian.PutUint64(bytes, bits)
		regs := []uint16{
			binary.BigEndian.Uint16(bytes[6:8]),
			binary.BigEndian.Uint16(bytes[4:6]),
			binary.BigEndian.Uint16(bytes[2:4]),
			binary.BigEndian.Uint16(bytes[0:2]),
		}
		// 对每个寄存器进行字节交换（Little Endian需要）
		for i := range regs {
			regs[i] = (regs[i] >> 8) | ((regs[i] & 0xFF) << 8)
		}
		return regs
	case config.ByteOrderSwap:
		binary.BigEndian.PutUint64(bytes, bits)
		bytes = c.swapBytesInRegisters(bytes)
	case config.ByteOrderMiddleEndian:
		binary.BigEndian.PutUint64(bytes, bits)
		bytes = c.swapRegisterPairs64(bytes)
	default:
		binary.BigEndian.PutUint64(bytes, bits)
	}

	return []uint16{
		binary.BigEndian.Uint16(bytes[0:2]),
		binary.BigEndian.Uint16(bytes[2:4]),
		binary.BigEndian.Uint16(bytes[4:6]),
		binary.BigEndian.Uint16(bytes[6:8]),
	}
}

// swapBytesInRegisters 交换字节（每个16位寄存器内的字节交换）
func (c *Collector) swapBytesInRegisters(bytes []byte) []byte {
	result := make([]byte, len(bytes))
	for i := 0; i < len(bytes); i += 2 {
		if i+1 < len(bytes) {
			result[i] = bytes[i+1]
			result[i+1] = bytes[i]
		}
	}
	return result
}

// swapRegisterPairs 交换32位值的两个寄存器（用于Middle Endian）
func (c *Collector) swapRegisterPairs(bytes []byte) []byte {
	result := make([]byte, 4)
	result[0] = bytes[2]
	result[1] = bytes[3]
	result[2] = bytes[0]
	result[3] = bytes[1]
	return result
}

// swapRegisterPairs64 交换64位值的四个寄存器（用于Middle Endian）
func (c *Collector) swapRegisterPairs64(bytes []byte) []byte {
	result := make([]byte, 8)
	result[0] = bytes[4]
	result[1] = bytes[5]
	result[2] = bytes[6]
	result[3] = bytes[7]
	result[4] = bytes[0]
	result[5] = bytes[1]
	result[6] = bytes[2]
	result[7] = bytes[3]
	return result
}

// int16ToRegisters 将int16值转换为寄存器数组（根据字节序）
func (c *Collector) int16ToRegisters(value int16, byteOrder config.ByteOrder) []uint16 {
	var reg uint16

	switch byteOrder {
	case config.ByteOrderLittleEndian, config.ByteOrderSwap:
		// 字节交换
		b := []byte{byte(value >> 8), byte(value)}
		reg = uint16(b[0])<<8 | uint16(b[1])
		reg = (reg >> 8) | ((reg & 0xFF) << 8)
	default:
		reg = uint16(value)
	}

	return []uint16{reg}
}

// uint16ToRegisters 将uint16值转换为寄存器数组（根据字节序）
func (c *Collector) uint16ToRegisters(value uint16, byteOrder config.ByteOrder) []uint16 {
	var reg uint16

	switch byteOrder {
	case config.ByteOrderLittleEndian, config.ByteOrderSwap:
		// 字节交换
		reg = (value >> 8) | ((value & 0xFF) << 8)
	default:
		reg = value
	}

	return []uint16{reg}
}
