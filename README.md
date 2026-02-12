# MBS Scanner - Modbus数据采集服务

基于Go语言开发的Modbus数据采集服务，支持多设备并发采集，数据推送到Kafka消息队列，支持Windows/Linux服务运行。

## 功能特性

- ✅ 支持Modbus TCP协议数据采集
- ✅ 多设备并发采集，独立配置采集间隔
- ✅ CSV文件配置采集点位，支持多种数据类型
- ✅ Kafka消息队列数据推送
- ✅ Zap高性能日志，支持大小和时间滚动
- ✅ go-svc服务管理，支持Windows服务和Linux守护进程
- ✅ TOML配置文件，配置验证和默认值
- ✅ 优雅的服务启动和停止

## 支持的数据类型

- `int16` - 16位有符号整数
- `uint16` - 16位无符号整数  
- `int32` - 32位有符号整数
- `uint32` - 32位无符号整数
- `float32` - 32位浮点数（IEEE 754）
- `float64` - 64位浮点数（IEEE 754）
- `bool` - 布尔值
- `string` - 字符串

## 快速开始

### 1. 编译项目

```bash
# 下载依赖
go mod tidy

# 编译
go build -o mbsscaner main.go
```

### 2. 配置文件

复制示例配置文件并修改：

```bash
cp config.toml.example config.toml
```

主要配置项：
- Modbus设备地址、端口、从站ID
- Kafka代理地址和主题
- 日志级别和输出方式
- 采集间隔和点位文件路径

### 3. 点位配置

CSV文件格式说明：
```csv
name,address,data_type,length,scale,unit,description
temp_001,0,float32,2,0.1,°C,温度传感器001
```

字段说明：
- `name` - 点位名称（唯一）
- `address` - Modbus寄存器地址
- `data_type` - 数据类型
- `length` - 数据长度（寄存器数量）
- `scale` - 缩放因子
- `unit` - 单位
- `description` - 描述

### 4. 运行服务

```bash
# 直接运行
./mbsscaner -config config.toml

# 查看版本
./mbsscaner -version

# 查看帮助
./mbsscaner -help
```

## 配置示例

### TOML配置文件

```toml
# 服务配置
[service]
name = "mbsscaner"
display_name = "MBS Scanner Service"
description = "Modbus Data Scanner Service"

# Modbus设备配置
[modbus]
[[modbus.devices]]
name = "temperature_sensors"
host = "192.168.1.100"
port = 502
slave_id = 1
timeout = 5
interval = 10
points_file = "config/temperature_points.csv"

# Kafka配置
[kafka]
brokers = ["localhost:9092"]
topic = "modbus-data"
compression = "gzip"
batch_size = 100
flush_frequency = 100

# 日志配置
[logger]
level = "info"
output = "both"
filename = "logs/mbsscaner.log"
max_size = 100
max_backups = 5
max_age = 30
compress = true
```

### CSV点位文件

```csv
name,address,data_type,register_type,length,scale,unit,description
temp_001,0,float32,holding_register,2,0.1,°C,温度传感器001
pressure_001,2,float32,holding_register,2,0.01,kPa,压力传感器001
flow_rate_001,4,float32,holding_register,2,0.001,m³/h,流量计001
status_001,6,bool,holding_register,1,1,,设备状态
```

字段说明：
- `name` - 点位名称（唯一）
- `address` - Modbus寄存器地址
- `data_type` - 数据类型
- `register_type` - 寄存器类型（可选，默认holding_register）
- `length` - 数据长度（寄存器数量）
- `scale` - 缩放因子
- `unit` - 单位
- `description` - 描述

#### 支持的寄存器类型：
- `coil` - 线圈寄存器 (功能码 0x01)
- `discrete_input` - 离散输入寄存器 (功能码 0x02)
- `input_register` - 输入寄存器 (功能码 0x04)
- `holding_register` - 保持寄存器 (功能码 0x03)

**注意：** 当前版本主要支持保持寄存器，其他类型会在日志中给出警告并转换为保持寄存器读取。

## Kafka消息格式

采集到的数据以JSON格式推送到Kafka：

```json
{
  "device": "temperature_sensors",
  "values": [
    {
      "name": "temp_001",
      "value": 25.6,
      "dataType": "float32",
      "unit": "°C",
      "description": "温度传感器001",
      "timestamp": "2024-01-01T12:00:00Z",
      "quality": "GOOD",
      "device": "temperature_sensors"
    }
  ],
  "timestamp": "2024-01-01T12:00:00Z",
  "metadata": {
    "point_count": 1,
    "source": "mbsscaner"
  }
}
```

## 项目结构

```
mbsscaner/
├── main.go                    # 主程序入口
├── go.mod                     # Go模块文件
├── config.toml.example        # 配置文件示例
├── config/                    # 配置模块
│   ├── config.go              # 配置结构定义
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
└── logs/                      # 日志目录（运行时创建）
```

## 依赖库

- `github.com/simonvetter/modbus` - Modbus客户端
- `github.com/IBM/sarama` - Kafka客户端
- `github.com/judwhite/go-svc` - 服务管理
- `go.uber.org/zap` - 高性能日志
- `github.com/BurntSushi/toml` - TOML配置解析
- `gopkg.in/natefinch/lumberjack.v2` - 日志滚动

## 服务安装（Windows）

作为Windows服务运行：

```bash
# 安装服务
sc create "MBS Scanner" binPath= "C:\path\to\mbsscaner.exe -config C:\path\to\config.toml"

# 启动服务
sc start "MBS Scanner"

# 停止服务
sc stop "MBS Scanner"

# 删除服务
sc delete "MBS Scanner"
```

## 服务安装（Linux）

作为systemd服务运行：

创建服务文件 `/etc/systemd/system/mbsscaner.service`：

```ini
[Unit]
Description=MBS Scanner Service
After=network.target

[Service]
Type=simple
User=mbsscaner
Group=mbsscaner
WorkingDirectory=/opt/mbsscaner
ExecStart=/opt/mbsscaner/mbsscaner -config /opt/mbsscaner/config.toml
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
```

启动服务：

```bash
systemctl enable mbsscaner
systemctl start mbsscaner
systemctl status mbsscaner
```

## 故障排除

1. **连接Modbus设备失败**
   - 检查设备IP地址和端口
   - 确认网络连通性
   - 检查从站ID是否正确

2. **Kafka推送失败**
   - 检查Kafka服务是否运行
   - 确认主题是否存在
   - 检查网络连接

3. **日志文件无法创建**
   - 检查日志目录权限
   - 确认磁盘空间充足

## 许可证

MIT License