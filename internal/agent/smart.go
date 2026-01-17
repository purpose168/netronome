// Copyright (c) 2024-2025, s0up 和 autobrr 贡献者.
// SPDX-License-Identifier: GPL-2.0-or-later

//go:build !nosmart && (linux || darwin)
// +build !nosmart
// +build linux darwin

// Netronome SMART 监控模块
// 包名: agent
// 功能: 实现基于 SMART（自我监控、分析和报告技术）的磁盘信息和温度监控
// 作者: s0up 和 autobrr 贡献者
// 创建日期: 2024
// 许可证: GPL-2.0-or-later

// 条件编译说明：
// - !nosmart: 当未定义 nosmart 构建标签时编译
// - linux || darwin: 仅在 Linux 和 macOS 平台上编译
// 可通过 `go build -tags nosmart` 禁用 SMART 功能

// SMART（自我监控、分析和报告技术）是一种磁盘监控技术：
// 1. 允许硬盘和 SSD 监控自身的健康状况
// 2. 提供温度、错误率、寿命等关键指标
// 3. 支持预测硬件故障，提前进行数据备份
// 4. 支持 SATA 和 NVMe 设备类型
// 5. 需要 root 权限或适当的设备访问权限

// 支持的设备类型：
// - SATA 设备：传统机械硬盘（HDD）和 SATA SSD
// - NVMe 设备：新一代高速固态存储设备
//
// ATA 字符串格式说明：
// SATA 设备的识别信息（如型号、序列号）使用特殊的字节对交换格式存储
// 这是 ATA 规范的要求，需要进行字节交换才能正确显示中文

package agent

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/anatol/smart.go"
	"github.com/rs/zerolog/log"
)

// swapBytes 交换 ATA 字符串中的字节对
// ATA 规范要求设备的识别信息（如型号、序列号）使用字节对交换格式存储
// 例如："A1B2C3" 在存储时会变成 "1A2B3C"
// 此函数用于将存储格式转换为正常可读格式
//
// 参数：
//
//	b: 包含 ATA 格式字符串的字节数组
//
// 返回值：
//
//	[]byte: 转换后的正常格式字节数组
//
// 实现逻辑：
// 1. 创建与输入等长的新字节数组
// 2. 遍历输入数组，每次处理两个字节
// 3. 将第一个字节与第二个字节交换位置
// 4. 如果输入长度为奇数，最后一个字节保持不变
// 5. 返回交换后的字节数组
func (a *Agent) swapBytes(b []byte) []byte {
	swapped := make([]byte, len(b))
	for i := 0; i < len(b)-1; i += 2 {
		swapped[i] = b[i+1] // 将第二个字节放到第一个位置
		swapped[i+1] = b[i] // 将第一个字节放到第二个位置
	}
	if len(b)%2 == 1 {
		swapped[len(b)-1] = b[len(b)-1] // 如果长度为奇数，最后一个字节保持不变
	}
	return swapped
}

// getDiskInfo 获取指定设备路径的磁盘型号和序列号信息
// 该函数通过 SMART API 访问磁盘的识别信息，支持 SATA 和 NVMe 设备类型
//
// 参数：
//
//	devicePath: 磁盘设备路径（如 /dev/sda, /dev/nvme0n1）
//
// 返回值：
//
//	model: 磁盘型号名称（已去除首尾空格）
//	serial: 磁盘序列号（已去除首尾空格）
//	如果无法访问设备或获取信息失败，返回空字符串
//
// 实现逻辑：
// 1. 尝试打开指定的设备路径
// 2. 使用 defer 确保设备在函数退出时关闭
// 3. 根据设备类型调用不同的识别方法：
//   - SATA 设备：使用 d.Identify() 获取识别信息，需要调用 swapBytes 转换 ATA 字符串格式
//   - NVMe 设备：使用 d.Identify() 获取识别信息，字符串格式正常，不需要转换
//
// 4. 去除首尾空格后返回型号和序列号
func (a *Agent) getDiskInfo(devicePath string) (model string, serial string) {
	dev, err := smart.Open(devicePath)
	if err != nil {
		return "", "" // 如果无法打开设备，返回空字符串
	}
	defer dev.Close() // 确保设备在函数退出时关闭

	// 根据设备类型获取识别信息
	switch d := dev.(type) {
	case *smart.SataDevice:
		if ident, err := d.Identify(); err == nil {
			// SATA 设备的 ATA 字符串需要交换字节对
			model = strings.TrimSpace(string(a.swapBytes(ident.ModelNumberRaw[:])))
			serial = strings.TrimSpace(string(a.swapBytes(ident.SerialNumberRaw[:])))
		}
	case *smart.NVMeDevice:
		if ctrl, _, err := d.Identify(); err == nil {
			// NVMe 设备的字符串格式正常，不需要交换字节
			model = strings.TrimSpace(string(ctrl.ModelNumberRaw[:]))
			serial = strings.TrimSpace(string(ctrl.SerialNumberRaw[:]))
		}
	}

	return model, serial
}

// getHDDTemperatures 通过 SMART 数据获取磁盘温度信息
// 该函数扫描系统中所有支持的存储设备，获取它们的温度数据
// 支持 SATA 和 NVMe 设备类型，返回格式化的温度统计信息
//
// 返回值：
//
//	[]TemperatureStats: 包含所有检测到的磁盘温度信息的数组
//	如果没有设备支持 SMART 或无法获取温度数据，返回空数组
//
// 实现逻辑：
//  1. 根据当前平台获取所有存储设备路径
//  2. 遍历每个设备路径：
//     a. 获取设备的基本名称（如 sda, nvme0n1）
//     b. 尝试使用 SMART 打开设备
//     c. 如果打开失败，记录调试信息并继续下一个设备
//     d. 读取设备的通用属性（包括温度）
//     e. 如果读取失败，关闭设备并继续下一个设备
//     f. 检查温度是否有效（0°C 到 100°C 之间）
//     g. 确定设备类型（HDD 或 NVMe）并获取型号信息
//     h. 格式化传感器标签和键名
//     i. 创建并添加 TemperatureStats 对象到结果数组
//     j. 记录调试信息
//     k. 关闭设备
//  3. 返回所有有效温度数据
//
// 注意事项：
// - 需要 root 权限或适当的设备访问权限
// - 某些设备可能不支持 SMART 或温度监控
// - 临界温度设置为 60°C（HDD 通常在 60°C 开始警告，SSD 在 70°C）
func (a *Agent) getHDDTemperatures() []TemperatureStats {
	var temps []TemperatureStats

	// 根据当前平台获取所有存储设备路径
	devicePaths := a.getDevicePaths()

	for _, devicePath := range devicePaths {
		name := filepath.Base(devicePath) // 获取设备基本名称（如 sda, nvme0n1）

		// 尝试使用 SMART 打开设备
		dev, err := smart.Open(devicePath)
		if err != nil {
			log.Trace().Str("device", devicePath).Err(err).Msg("无法打开设备以获取 SMART 数据（可能需要 root 权限或平台不支持）")
			continue
		}

		// 使用通用属性 API 获取温度数据
		attrs, err := dev.ReadGenericAttributes()
		if err != nil {
			dev.Close()
			log.Trace().Str("device", devicePath).Err(err).Msg("无法读取设备的通用属性")
			continue
		}

		// 检查温度数据是否有效（0°C 到 100°C 之间）
		if attrs != nil && attrs.Temperature > 0 && attrs.Temperature < 100 {
			deviceType := "HDD" // 默认设备类型为 HDD
			modelName := ""     // 设备型号名称

			// 获取设备型号信息，根据设备类型使用不同的处理方式
			switch d := dev.(type) {
			case *smart.SataDevice:
				if ident, err := d.Identify(); err == nil {
					// SATA 设备的 ATA 字符串需要交换字节对
					modelName = strings.TrimSpace(string(a.swapBytes(ident.ModelNumberRaw[:])))
					// 注意：这里简化处理，默认将 SATA 设备视为 HDD
					// 实际上可以通过旋转速率（0 RPM 为 SSD）来判断，但某些 SSD 可能不报告 0
					deviceType = "HDD"
				}
			case *smart.NVMeDevice:
				deviceType = "NVMe" // NVMe 设备通常是 SSD
				if ctrl, _, err := d.Identify(); err == nil {
					// NVMe 设备的字符串格式正常，不需要交换字节
					modelName = strings.TrimSpace(string(ctrl.ModelNumberRaw[:]))
				}
			}

			// 格式化传感器标签
			label := fmt.Sprintf("%s %s", deviceType, strings.ToUpper(name))
			if modelName != "" {
				// 如果获取到型号信息，使用型号+设备名称作为标签
				label = fmt.Sprintf("%s (%s)", modelName, strings.ToUpper(name))
			}

			// 创建温度统计对象并添加到结果数组
			temps = append(temps, TemperatureStats{
				SensorKey:   fmt.Sprintf("smart_%s", name), // 传感器唯一标识
				Temperature: float64(attrs.Temperature),    // 温度值（摄氏度）
				Label:       label,                         // 传感器标签
				Critical:    60.0,                          // 临界温度（HDD 通常在 60°C 警告）
			})

			// 记录调试信息
			log.Debug().
				Str("device", name).
				Uint64("temp", attrs.Temperature).
				Str("type", deviceType).
				Str("model", modelName).
				Msg("通过 SMART 添加磁盘温度数据")
		}

		dev.Close() // 关闭设备
	}

	return temps // 返回所有有效温度数据
}
