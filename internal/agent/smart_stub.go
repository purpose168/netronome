// Copyright (c) 2024-2025, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later
//
// SMART功能的空实现模块
// 这个文件仅在以下条件下编译：
// 1. 定义了nosmart构建标签
// 2. 或者当前平台不是Linux且不是Darwin（macOS）
//
// Go语言特定概念解释：
// - 条件编译标签（build tags）: 控制代码在什么条件下编译
//   - go:build: 新的构建标签语法（Go 1.17+）
//   - +build: 旧的构建标签语法（兼容旧版本Go）
// - 构建标签逻辑: || 表示或，!表示非，()表示分组
// - 空实现（Stub）: 为了满足接口或编译要求而提供的简单实现
// - SMART (Self-Monitoring, Analysis and Reporting Technology):
//   自我监测、分析与报告技术，用于监控硬盘健康状态
//
//go:build nosmart || (!linux && !darwin)
// +build nosmart !linux,!darwin

package agent

// getDiskInfo 用于不支持SMART的平台的空实现
// @Description 在不支持SMART功能的平台上返回空的磁盘信息
// @Param string 磁盘设备路径（未使用）
// @Return model string 空字符串
// @Return serial string 空字符串
func (a *Agent) getDiskInfo(_ string) (model string, serial string) {
	return "", ""
}

// getHDDTemperatures 用于不支持SMART的平台的空实现
// @Description 在不支持SMART功能的平台上返回空的硬盘温度统计
// @Return []TemperatureStats 空的温度统计切片
func (a *Agent) getHDDTemperatures() []TemperatureStats {
	return []TemperatureStats{}
}
