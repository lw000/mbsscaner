# 字节序配置说明

## 概述

MBS Scanner 支持 int32、uint32、float32 数据类型的字节序配置，以适应不同设备的字节序需求。

**重要更新**: 字节序配置已从CSV点位级别移动到TOML设备级别，简化了配置管理。

## 支持的字节序

| 字节序 | 说明 | 示例 | 适合场景 |
|--------|------|------|----------|
| big | 大端序 (Big Endian) | ABCD | 标准网络字节序，大多数现代设备 |
| little | 小端序 (Little Endian) | DCBA | Intel x86架构设备 |
| swap | 字节交换 | BADC | 交换每个寄存器内的字节顺序 |
| middle | 中端序 (Middle Endian) | CDAB | 交换寄存器顺序，某些工业设备 |

## 配置方法

### 1. TOML 设备配置

在 TOML 配置文件的设备节点中添加 `byte_order` 字段：

```toml
[[modbus.devices]]
name = "siemens_plc"
host = "192.168.1.100"
port = 502
slave_id = 1
timeout = 5
interval = 2
points_file = "config/siemens_points.csv"
byte_order = "big"  # 设备级别字节序配置

[[modbus.devices]]
name = "omron_plc"
host = "192.168.1.101"
port = 502
slave_id = 1
timeout = 5
interval = 3
points_file = "config/omron_points.csv"
byte_order = "little"  # 小端序
```

### 2. CSV 点位配置

CSV 配置文件现在**不需要** `byte_order` 列：

```csv
name,address,data_type,register_type,scale,unit,description
temp_sensor,100,int32,holding_register,0.1,°C,温度传感器
pressure_sensor,102,int32,holding_register,0.01,kPa,压力传感器
flow_rate,104,float32,holding_register,0.001,m³/s,流量计
voltage,106,uint32,holding_register,0.01,V,电压表
```

### 3. 字段说明

- `byte_order`: TOML设备配置中的可选字段，如果不指定，默认为 `big`
- 支持的值：`big`, `little`, `swap`, `middle`
- 字节序配置应用于整个设备的所有int32、uint32、float32数据类型点位
- 对于 int16、uint16、bool 数据类型，字节序配置无效

## 字节序详解

假设要存储的32位值：**0x12345678**

### 大端序 (Big Endian) - ABCD
```
寄存器 0: 0x1234 (高16位)
寄存器 1: 0x5678 (低16位)
字节序列: 12 34 56 78
使用场景: 西门子PLC、网络协议等
```

### 小端序 (Little Endian) - DCBA
```
寄存器 0: 0x1234
寄存器 1: 0x5678
字节序列: 78 56 34 12
使用场景: Intel x86架构、某些PC系统
```

### 字节交换 (Swap) - BADC
```
设备实际存储：
寄存器 0: 0x3412 (字节已交换)
寄存器 1: 0x7856 (字节已交换)
字节序列: 12 34 56 78 (解析时还原)
使用场景: 某些工业控制器、特殊设备
```

### 中端序 (Middle Endian) - CDAB
```
寄存器 0: 0x5678 (原低16位)
寄存器 1: 0x1234 (原高16位)
字节序列: 12 34 56 78 (解析时还原)
使用场景: 三菱PLC、某些日系设备
```

## 字节序工作原理

系统会根据设备配置的字节序，自动处理从Modbus寄存器读取的数据，确保最终得到正确的数值。用户只需要在TOML配置中指定设备使用的字节序格式。

## 实际使用示例

### 配置示例：不同设备类型

#### TOML 配置文件：

```toml
[[modbus.devices]]
name = "siemens_plc"
host = "192.168.1.100"
port = 502
slave_id = 1
timeout = 5
interval = 2
points_file = "config/siemens_points.csv"
byte_order = "big"  # 西门子使用大端序

[[modbus.devices]]
name = "omron_plc"
host = "192.168.1.101"
port = 502
slave_id = 1
timeout = 5
interval = 3
points_file = "config/omron_points.csv"
byte_order = "little"  # 欧姆龙使用小端序

[[modbus.devices]]
name = "schneider_plc"
host = "192.168.1.102"
port = 502
slave_id = 1
timeout = 5
interval = 2
points_file = "config/schneider_points.csv"
byte_order = "swap"  # 施耐德使用字节交换

[[modbus.devices]]
name = "mitsubishi_plc"
host = "192.168.1.103"
port = 502
slave_id = 1
timeout = 5
interval = 4
points_file = "config/mitsubishi_points.csv"
byte_order = "middle"  # 三菱使用中端序
```

#### 对应的 CSV 文件：

```csv
# siemens_points.csv
name,address,data_type,register_type,scale,unit,description
siemens_temp,100,int32,holding_register,0.1,°C,西门子温度传感器
siemens_pressure,102,uint32,holding_register,0.01,kPa,西门子压力传感器

# omron_points.csv
name,address,data_type,register_type,scale,unit,description
omron_pressure,200,int32,holding_register,0.01,MPa,欧姆龙压力传感器
omron_flow,202,float32,holding_register,0.001,m³/h,欧姆龙流量计

# schneider_points.csv
name,address,data_type,register_type,scale,unit,description
schneider_flow,300,float32,holding_register,0.001,m³/h,施耐德流量计
schneider_voltage,302,uint32,holding_register,0.1,V,施耐德电压表

# mitsubishi_points.csv
name,address,data_type,register_type,scale,unit,description
mitsubishi_voltage,400,uint32,holding_register,0.1,V,三菱电压表
mitsubishi_current,402,int32,holding_register,0.001,A,三菱电流表
```

## 注意事项

1. **默认值**: 如果不指定 `byte_order`，系统默认使用大端序 (`big`)
2. **数据类型**: 字节序配置仅对 32 位数据类型（int32、uint32、float32）有效
3. **兼容性**: 不同厂商的设备可能使用不同的字节序，请参考设备手册
4. **测试**: 建议先用已知值测试确定正确的字节序配置

## 故障排除

### 常见问题

1. **数值异常**: 如果读取的数值明显异常（如极大的负值或乱码），通常是字节序配置错误
2. **配置错误**: 字节序值拼写错误会导致加载失败
3. **兼容性**: 某些设备可能需要特殊的字节序，不在标准四种之内

### 验证方法

1. 使用已知固定值的寄存器进行测试
2. 对比不同字节序配置下的读取结果
3. 查看设备手册确认字节序规范

## 技术实现

系统使用以下方法处理字节序：

1. **读取寄存器**: 读取连续的两个 16 位寄存器
2. **字节组装**: 根据配置的字节序组装 4 字节数组
3. **数据转换**: 使用相应的字节序转换函数获取最终数值
4. **应用缩放**: 根据缩放因子进行最终计算

通过灵活的字节序配置，MBS Scanner 能够适配各种工业设备的通信需求。