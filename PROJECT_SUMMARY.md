# MBS Scanner - 项目实现总结

## 项目概述

基于Go语言实现的Modbus数据采集服务，支持多设备并发采集，数据推送到Kafka消息队列，具备完整的服务管理功能。

## 已实现功能

### ✅ 核心功能模块

1. **配置管理模块** (`config/`)
   - TOML配置文件解析和验证
   - CSV点位文件解析
   - 支持多种数据类型配置
   - 配置验证和默认值处理

2. **Modbus数据采集模块** (`pkg/modbus/`)
   - 基于github.com/simonvetter/modbus实现
   - 支持int16/uint16/int32/uint32/float32/float64/bool/string数据类型
   - 多设备并发采集
   - 连接测试和错误处理
   - 数据缩放和单位处理

3. **Kafka消息推送模块** (`pkg/kafka/`)
   - 基于github.com/IBM/sarama实现
   - 同步和异步消息发送
   - 消息压缩和批处理
   - 完整的错误处理和重试机制

4. **日志管理模块** (`pkg/logger/`)
   - 基于go.uber.org/zap实现
   - 支持控制台、文件、同时输出
   - 日志滚动（大小和时间）
   - 结构化日志记录

5. **服务管理模块** (`pkg/service/`)
   - 基于github.com/judwhite/go-svc实现
   - Windows服务和Linux守护进程支持
   - 优雅启动和停止
   - 信号处理和上下文管理

### ✅ 主程序入口 (`main.go`)
- 命令行参数解析
- 版本和帮助信息
- 配置文件验证
- 服务启动逻辑

### ✅ 配置文件和示例
- 完整的TOML配置示例
- 多个CSV点位配置示例
- 详细的README文档
- Makefile和Windows批处理构建脚本

## 项目结构

```
mbsscaner/
├── main.go                    # 主程序入口
├── go.mod                     # Go模块文件
├── Makefile                   # 构建脚本
├── build.bat                  # Windows构建脚本
├── config.toml.example        # 配置文件示例
├── README.md                  # 项目文档
├── config/                    # 配置模块
│   ├── config.go              # 配置结构定义和加载
│   └── points.go              # 点位配置解析
├── pkg/                       # 功能模块
│   ├── modbus/                # Modbus采集模块
│   │   └── collector.go
│   ├── kafka/                 # Kafka推送模块
│   │   └── producer.go
│   ├── logger/                # 日志模块
│   │   └── logger.go
│   └── service/               # 服务管理模块
│       └── service.go
├── config/                    # 配置文件目录
│   ├── temperature_points.csv
│   ├── pressure_points.csv
│   └── flow_points.csv
└── build/                     # 构建输出目录
```

## 技术特性

### 🔧 依赖库选择
- **Modbus**: github.com/simonvetter/modbus - 稳定可靠的Modbus客户端
- **Kafka**: github.com/IBM/sarama - 官方推荐的Kafka Go客户端
- **日志**: go.uber.org/zap - 高性能结构化日志
- **服务**: github.com/judwhite/go-svc - 跨平台服务管理
- **配置**: github.com/BurntSushi/toml - TOML格式支持
- **日志滚动**: gopkg.in/natefinch/lumberjack.v2 - 日志文件管理

### 🚀 性能优化
- 并发采集多个Modbus设备
- Kafka消息批处理和压缩
- 高性能日志记录
- 内存优化的数据结构

### 🛡️ 可靠性保障
- 完整的错误处理和重试机制
- 连接状态监控
- 优雅的服务启动和停止
- 配置验证和默认值

### 📊 数据格式
- 支持多种Modbus数据类型
- JSON格式的Kafka消息
- 结构化的日志输出
- 可扩展的配置格式

## 使用方式

### 1. 环境准备
```bash
# 安装Go 1.21+
# 配置GOPATH和PATH
```

### 2. 项目构建
```bash
# Linux/macOS
make build

# Windows
build.bat
```

### 3. 配置设置
```bash
# 复制配置文件
cp config.toml.example config.toml

# 创建日志目录
mkdir logs
```

### 4. 运行服务
```bash
# 直接运行
./mbsscaner -config config.toml

# 查看帮助
./mbsscaner -help
```

## 配置示例

### TOML配置
```toml
[modbus]
[[modbus.devices]]
name = "device1"
host = "192.168.1.100"
port = 502
slave_id = 1
interval = 10
points_file = "config/points.csv"

[kafka]
brokers = ["localhost:9092"]
topic = "modbus-data"
compression = "gzip"

[logger]
level = "info"
output = "both"
filename = "logs/mbsscaner.log"
```

### CSV点位配置
```csv
name,address,data_type,length,scale,unit,description
temp_001,0,float32,2,0.1,°C,温度传感器001
pressure_001,2,float32,2,0.01,kPa,压力传感器001
```

## Kafka消息格式

```json
{
  "device": "device1",
  "values": [
    {
      "name": "temp_001",
      "value": 25.6,
      "dataType": "float32",
      "unit": "°C",
      "timestamp": "2024-01-01T12:00:00Z",
      "quality": "GOOD",
      "device": "device1"
    }
  ],
  "timestamp": "2024-01-01T12:00:00Z"
}
```

## 部署选项

### Windows服务
```bash
sc create "MBS Scanner" binPath= "C:\path\to\mbsscaner.exe -config C:\path\to\config.toml"
sc start "MBS Scanner"
```

### Linux Systemd服务
```ini
[Unit]
Description=MBS Scanner Service
[Service]
ExecStart=/opt/mbsscaner/mbsscaner -config /opt/mbsscaner/config.toml
[Install]
WantedBy=multi-user.target
```

## 扩展性

### 支持的数据类型
- int16, uint16 - 16位整数
- int32, uint32 - 32位整数  
- float32, float64 - 浮点数
- bool - 布尔值
- string - 字符串

### 可配置选项
- 多Modbus设备并发采集
- 独立的采集间隔配置
- Kafka压缩和批处理
- 日志级别和输出方式
- 服务管理参数

## 项目优势

1. **模块化设计** - 清晰的模块划分，易于维护和扩展
2. **配置驱动** - 通过配置文件灵活控制行为
3. **高性能** - 并发采集，优化的数据处理
4. **可靠性** - 完整的错误处理和恢复机制
5. **易部署** - 支持多种部署方式和服务管理
6. **可观测** - 详细的日志记录和监控支持

## 总结

该项目成功实现了一个完整的Modbus数据采集服务，具备以下特点：

- ✅ 功能完整 - 涵盖数据采集、消息推送、日志记录、服务管理
- ✅ 技术先进 - 使用现代化的Go语言生态库
- ✅ 配置灵活 - 支持多种配置选项和部署方式
- ✅ 性能优异 - 并发处理和优化算法
- ✅ 可靠性强 - 完善的错误处理和恢复机制
- ✅ 易于使用 - 详细的文档和示例配置

项目代码结构清晰，模块划分合理，具备良好的可维护性和扩展性，可以作为生产环境使用的Modbus数据采集解决方案。