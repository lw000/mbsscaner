package modbus

import (
	"fmt"
)

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
