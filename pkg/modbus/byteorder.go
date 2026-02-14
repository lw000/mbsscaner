package modbus

import (
	"encoding/binary"
	"math"

	"mbsscaner/config"
)

// swapBytesInRegisters 交换字节（每个16位寄存器内的字节交换）
func swapBytesInRegisters(bytes []byte) []byte {
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
func swapRegisterPairs(bytes []byte) []byte {
	result := make([]byte, 4)
	result[0] = bytes[2]
	result[1] = bytes[3]
	result[2] = bytes[0]
	result[3] = bytes[1]
	return result
}

// swapRegisterPairs64 交换64位值的四个寄存器（用于Middle Endian）
func swapRegisterPairs64(bytes []byte) []byte {
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
func int16ToRegisters(value int16, byteOrder config.ByteOrder) []uint16 {
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
func uint16ToRegisters(value uint16, byteOrder config.ByteOrder) []uint16 {
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

// int32ToRegisters 将int32值转换为寄存器数组（根据字节序）
func int32ToRegisters(value int32, byteOrder config.ByteOrder) []uint16 {
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
		bytes = swapBytesInRegisters(bytes)
	case config.ByteOrderMiddleEndian:
		binary.BigEndian.PutUint32(bytes, uint32(value))
		bytes = swapRegisterPairs(bytes)
	default:
		binary.BigEndian.PutUint32(bytes, uint32(value))
	}

	return []uint16{
		binary.BigEndian.Uint16(bytes[0:2]),
		binary.BigEndian.Uint16(bytes[2:4]),
	}
}

// uint32ToRegisters 将uint32值转换为寄存器数组（根据字节序）
func uint32ToRegisters(value uint32, byteOrder config.ByteOrder) []uint16 {
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
		bytes = swapBytesInRegisters(bytes)
	case config.ByteOrderMiddleEndian:
		binary.BigEndian.PutUint32(bytes, value)
		bytes = swapRegisterPairs(bytes)
	default:
		binary.BigEndian.PutUint32(bytes, value)
	}

	return []uint16{
		binary.BigEndian.Uint16(bytes[0:2]),
		binary.BigEndian.Uint16(bytes[2:4]),
	}
}

// float32ToRegisters 将float32值转换为寄存器数组（根据字节序）
func float32ToRegisters(value float32, byteOrder config.ByteOrder) []uint16 {
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
		bytes = swapBytesInRegisters(bytes)
	case config.ByteOrderMiddleEndian:
		binary.BigEndian.PutUint32(bytes, bits)
		bytes = swapRegisterPairs(bytes)
	default:
		binary.BigEndian.PutUint32(bytes, bits)
	}

	return []uint16{
		binary.BigEndian.Uint16(bytes[0:2]),
		binary.BigEndian.Uint16(bytes[2:4]),
	}
}

// float64ToRegisters 将float64值转换为寄存器数组（根据字节序）
func float64ToRegisters(value float64, byteOrder config.ByteOrder) []uint16 {
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
		bytes = swapBytesInRegisters(bytes)
	case config.ByteOrderMiddleEndian:
		binary.BigEndian.PutUint64(bytes, bits)
		bytes = swapRegisterPairs64(bytes)
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

// putBytesWithOrder 根据字节序将寄存器值放入字节数组
func putBytesWithOrder(bytes []byte, registers []uint16, byteOrder config.ByteOrder) {
	if len(registers) < 2 {
		return
	}

	switch byteOrder {
	case config.ByteOrderBigEndian:
		binary.BigEndian.PutUint16(bytes[0:2], registers[0])
		binary.BigEndian.PutUint16(bytes[2:4], registers[1])

	case config.ByteOrderLittleEndian:
		swappedReg0 := (registers[0] >> 8) | ((registers[0] & 0xFF) << 8)
		swappedReg1 := (registers[1] >> 8) | ((registers[1] & 0xFF) << 8)
		binary.LittleEndian.PutUint16(bytes[0:2], swappedReg0)
		binary.LittleEndian.PutUint16(bytes[2:4], swappedReg1)

	case config.ByteOrderSwap:
		swappedReg0 := (registers[0] >> 8) | ((registers[0] & 0xFF) << 8)
		swappedReg1 := (registers[1] >> 8) | ((registers[1] & 0xFF) << 8)
		binary.BigEndian.PutUint16(bytes[0:2], swappedReg0)
		binary.BigEndian.PutUint16(bytes[2:4], swappedReg1)

	case config.ByteOrderMiddleEndian:
		binary.BigEndian.PutUint16(bytes[0:2], registers[1])
		binary.BigEndian.PutUint16(bytes[2:4], registers[0])
	}
}

// getInt32WithOrder 根据字节序从字节数组读取int32
func getInt32WithOrder(bytes []byte, byteOrder config.ByteOrder) int32 {
	switch byteOrder {
	case config.ByteOrderBigEndian:
		return int32(binary.BigEndian.Uint32(bytes))
	case config.ByteOrderLittleEndian:
		return int32(binary.LittleEndian.Uint32(bytes))
	case config.ByteOrderSwap, config.ByteOrderMiddleEndian:
		return int32(binary.BigEndian.Uint32(bytes))
	default:
		return int32(binary.BigEndian.Uint32(bytes))
	}
}

// getUint32WithOrder 根据字节序从字节数组读取uint32
func getUint32WithOrder(bytes []byte, byteOrder config.ByteOrder) uint32 {
	switch byteOrder {
	case config.ByteOrderBigEndian:
		return binary.BigEndian.Uint32(bytes)
	case config.ByteOrderLittleEndian:
		return binary.LittleEndian.Uint32(bytes)
	case config.ByteOrderSwap, config.ByteOrderMiddleEndian:
		return binary.BigEndian.Uint32(bytes)
	default:
		return binary.BigEndian.Uint32(bytes)
	}
}

// putFloat64BytesWithOrder 根据字节序将float64寄存器值放入字节数组
func putFloat64BytesWithOrder(bytes []byte, registers []uint16, byteOrder config.ByteOrder) {
	if len(registers) < 4 {
		return
	}

	switch byteOrder {
	case config.ByteOrderBigEndian:
		binary.BigEndian.PutUint16(bytes[0:2], registers[0])
		binary.BigEndian.PutUint16(bytes[2:4], registers[1])
		binary.BigEndian.PutUint16(bytes[4:6], registers[2])
		binary.BigEndian.PutUint16(bytes[6:8], registers[3])
	case config.ByteOrderLittleEndian:
		swappedReg0 := (registers[0] >> 8) | ((registers[0] & 0xFF) << 8)
		swappedReg1 := (registers[1] >> 8) | ((registers[1] & 0xFF) << 8)
		swappedReg2 := (registers[2] >> 8) | ((registers[2] & 0xFF) << 8)
		swappedReg3 := (registers[3] >> 8) | ((registers[3] & 0xFF) << 8)
		binary.LittleEndian.PutUint16(bytes[0:2], swappedReg0)
		binary.LittleEndian.PutUint16(bytes[2:4], swappedReg1)
		binary.LittleEndian.PutUint16(bytes[4:6], swappedReg2)
		binary.LittleEndian.PutUint16(bytes[6:8], swappedReg3)
	case config.ByteOrderSwap:
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
		binary.BigEndian.PutUint16(bytes[0:2], registers[2])
		binary.BigEndian.PutUint16(bytes[2:4], registers[3])
		binary.BigEndian.PutUint16(bytes[4:6], registers[0])
		binary.BigEndian.PutUint16(bytes[6:8], registers[1])
	}
}
