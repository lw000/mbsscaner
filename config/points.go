package config

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// DataType 数据类型枚举
type DataType string

const (
	DataTypeInt16   DataType = "int16"
	DataTypeUInt16  DataType = "uint16"
	DataTypeInt32   DataType = "int32"
	DataTypeUInt32  DataType = "uint32"
	DataTypeFloat32 DataType = "float32"
	DataTypeFloat64 DataType = "float64"
	DataTypeBool    DataType = "bool"
	DataTypeString  DataType = "string"
)

// RegisterType Modbus寄存器类型枚举
type RegisterType string

const (
	RegisterTypeCoil            RegisterType = "coil"             // 线圈寄存器 (0x01)
	RegisterTypeDiscreteInput   RegisterType = "discrete_input"   // 离散输入寄存器 (0x02)
	RegisterTypeInputRegister   RegisterType = "input_register"   // 输入寄存器 (0x04)
	RegisterTypeHoldingRegister RegisterType = "holding_register" // 保持寄存器 (0x03)
)

// Point 采集点位
type Point struct {
	Name         string       `json:"name"`         // 点位名称
	Address      uint16       `json:"address"`      // 寄存器地址
	DataType     DataType     `json:"dataType"`     // 数据类型
	RegisterType RegisterType `json:"registerType"` // 寄存器类型
	Length       uint16       `json:"length"`       // 数据长度（用于字符串类型）
	Scale        float64      `json:"scale"`        // 缩放因子
	Unit         string       `json:"unit"`         // 单位
	Description  string       `json:"description"`  // 描述
}

// LoadPoints 从CSV文件加载采集点位
func LoadPoints(csvFile string) ([]Point, error) {
	file, err := os.Open(csvFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open CSV file %s: %w", csvFile, err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV file %s: %w", csvFile, err)
	}

	if len(records) < 2 {
		return nil, fmt.Errorf("CSV file must contain header and at least one data row")
	}

	var points []Point
	header := records[0]

	// 验证CSV头部（基础必需字段）
	requiredHeaders := []string{"name", "address", "data_type", "register_type"}
	for _, expected := range requiredHeaders {
		found := false
		for _, h := range header {
			if strings.ToLower(strings.TrimSpace(h)) == expected {
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("CSV file must contain column: %s", expected)
		}
	}

	// 解析数据行
	for i, record := range records[1:] {
		if len(record) == 0 || (len(record) == 1 && strings.TrimSpace(record[0]) == "") {
			continue // 跳过空行
		}

		point, err := parsePointRecord(header, record)
		if err != nil {
			return nil, fmt.Errorf("failed to parse record %d: %w", i+2, err)
		}

		points = append(points, point)
	}

	if len(points) == 0 {
		return nil, fmt.Errorf("no valid points found in CSV file")
	}

	return points, nil
}

// parsePointRecord 解析单行点位记录
func parsePointRecord(header, record []string) (Point, error) {
	var point Point

	// 创建字段映射
	fieldMap := make(map[string]string)
	for i, h := range header {
		if i < len(record) {
			fieldMap[strings.ToLower(strings.TrimSpace(h))] = strings.TrimSpace(record[i])
		}
	}

	// 解析必需字段
	name, ok := fieldMap["name"]
	if !ok || name == "" {
		return point, fmt.Errorf("name is required")
	}
	point.Name = name

	addressStr, ok := fieldMap["address"]
	if !ok || addressStr == "" {
		return point, fmt.Errorf("address is required")
	}
	address, err := strconv.ParseUint(addressStr, 10, 16)
	if err != nil {
		return point, fmt.Errorf("invalid address %s: %w", addressStr, err)
	}
	point.Address = uint16(address)

	dataTypeStr, ok := fieldMap["data_type"]
	if !ok || dataTypeStr == "" {
		return point, fmt.Errorf("data_type is required")
	}
	dataType := DataType(strings.ToLower(dataTypeStr))
	if !isValidDataType(dataType) {
		return point, fmt.Errorf("invalid data_type %s, supported types: %v", dataTypeStr, getSupportedDataTypes())
	}
	point.DataType = dataType

	// 解析寄存器类型
	registerTypeStr, ok := fieldMap["register_type"]
	if !ok || registerTypeStr == "" {
		// 默认使用保持寄存器
		point.RegisterType = RegisterTypeHoldingRegister
	} else {
		registerType := RegisterType(strings.ToLower(strings.TrimSpace(registerTypeStr)))
		if !isValidRegisterType(registerType) {
			return point, fmt.Errorf("invalid register_type %s, supported types: %v", registerTypeStr, getSupportedRegisterTypes())
		}
		point.RegisterType = registerType
	}

	// 解析可选字段
	if lengthStr, ok := fieldMap["length"]; ok && lengthStr != "" {
		length, err := strconv.ParseUint(lengthStr, 10, 16)
		if err != nil {
			return point, fmt.Errorf("invalid length %s: %w", lengthStr, err)
		}
		point.Length = uint16(length)
	} else {
		// 设置默认长度
		switch dataType {
		case DataTypeInt16, DataTypeUInt16, DataTypeBool:
			point.Length = 1
		case DataTypeInt32, DataTypeUInt32, DataTypeFloat32:
			point.Length = 2
		case DataTypeFloat64:
			point.Length = 4
		case DataTypeString:
			point.Length = 10 // 默认字符串长度
		}
	}

	if scaleStr, ok := fieldMap["scale"]; ok && scaleStr != "" {
		scale, err := strconv.ParseFloat(scaleStr, 64)
		if err != nil {
			return point, fmt.Errorf("invalid scale %s: %w", scaleStr, err)
		}
		point.Scale = scale
	} else {
		point.Scale = 1.0 // 默认不缩放
	}

	if unit, ok := fieldMap["unit"]; ok {
		point.Unit = unit
	}

	if description, ok := fieldMap["description"]; ok {
		point.Description = description
	}

	return point, nil
}

// isValidDataType 检查数据类型是否有效
func isValidDataType(dataType DataType) bool {
	switch dataType {
	case DataTypeInt16, DataTypeUInt16, DataTypeInt32, DataTypeUInt32,
		DataTypeFloat32, DataTypeFloat64, DataTypeBool, DataTypeString:
		return true
	default:
		return false
	}
}

// getSupportedDataTypes 获取支持的数据类型列表
func getSupportedDataTypes() []string {
	return []string{
		string(DataTypeInt16), string(DataTypeUInt16), string(DataTypeInt32),
		string(DataTypeUInt32), string(DataTypeFloat32), string(DataTypeFloat64),
		string(DataTypeBool), string(DataTypeString),
	}
}

// ValidatePoints 验证点位配置
func ValidatePoints(points []Point) error {
	if len(points) == 0 {
		return fmt.Errorf("points cannot be empty")
	}

	// 检查重复的点位名称
	nameMap := make(map[string]bool)
	addressMap := make(map[string]bool) // 使用"address:registerType"作为key

	for i, point := range points {
		if point.Name == "" {
			return fmt.Errorf("point %d: name cannot be empty", i)
		}

		if nameMap[point.Name] {
			return fmt.Errorf("point %d: duplicate name '%s'", i, point.Name)
		}
		nameMap[point.Name] = true

		// 检查相同寄存器类型的地址重复
		addressKey := fmt.Sprintf("%d:%s", point.Address, point.RegisterType)
		if addressMap[addressKey] {
			return fmt.Errorf("point %d: duplicate address %d for register type %s", i, point.Address, point.RegisterType)
		}
		addressMap[addressKey] = true

		if point.Length <= 0 {
			return fmt.Errorf("point %d: length must be positive", i)
		}

		if point.Scale == 0 {
			return fmt.Errorf("point %d: scale cannot be zero", i)
		}
	}

	return nil
}

// isValidRegisterType 检查寄存器类型是否有效
func isValidRegisterType(registerType RegisterType) bool {
	switch registerType {
	case RegisterTypeCoil, RegisterTypeDiscreteInput, RegisterTypeInputRegister, RegisterTypeHoldingRegister:
		return true
	default:
		return false
	}
}

// getSupportedRegisterTypes 获取支持的寄存器类型列表
func getSupportedRegisterTypes() []string {
	return []string{
		string(RegisterTypeCoil),
		string(RegisterTypeDiscreteInput),
		string(RegisterTypeInputRegister),
		string(RegisterTypeHoldingRegister),
	}
}
