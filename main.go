package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"mbsscaner/config"
	"mbsscaner/pkg/service"
)

const (
	version = "1.0.0"
	appName = "MBS Scanner"
)

var (
	configFile  = flag.String("config", "config.toml", "Configuration file path")
	versionFlag = flag.Bool("version", false, "Show version information")
	helpFlag    = flag.Bool("help", false, "Show help information")
)

func main() {
	flag.Parse()

	if *versionFlag {
		printVersion()
		return
	}

	if *helpFlag {
		printHelp()
		return
	}

	// 检查配置文件是否存在
	if !config.Exists(*configFile) {
		fmt.Printf("Error: Configuration file '%s' not found\n", *configFile)
		fmt.Println("Use -help to see usage information")
		os.Exit(1)
	}

	// 将配置文件路径转换为绝对路径
	absConfigPath, err := filepath.Abs(*configFile)
	if err != nil {
		fmt.Printf("Error: Failed to get absolute path for config file: %v\n", err)
		os.Exit(1)
	}

	// 运行服务
	if err := service.RunAsService(absConfigPath); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

// printVersion 打印版本信息
func printVersion() {
	fmt.Printf("%s v%s\n", appName, version)
	fmt.Printf("A Modbus data scanner service with Kafka integration\n")
}

// printHelp 打印帮助信息
func printHelp() {
	fmt.Printf("%s v%s\n\n", appName, version)
	fmt.Printf("Usage: %s [options]\n\n", os.Args[0])
	fmt.Println("Options:")
	flag.PrintDefaults()
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Printf("  %s -config config.toml\n", os.Args[0])
	fmt.Printf("  %s -config /etc/mbsscaner/config.toml\n", os.Args[0])
	fmt.Println()
	fmt.Println("Configuration:")
	fmt.Println("  The configuration file should be in TOML format.")
	fmt.Println("  See config.toml.example for an example configuration.")
}
