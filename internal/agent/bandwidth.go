// Copyright (c) 2024-2025, s0up 和 autobrr 贡献者.
// SPDX-License-Identifier: GPL-2.0-or-later
//
// Netronome 网络带宽监控模块
// 包名: agent
// 功能: 实现网络带宽监控、历史数据导出和峰值统计功能
// 作者: s0up 和 autobrr 贡献者
// 创建日期: 2024
// 许可证: GPL-2.0-or-later
//
// 主要功能:
// 1. 实时带宽监控：通过 vnstat 工具采集实时网络流量数据
// 2. 历史数据导出：提供历史网络带宽数据的 JSON 格式导出
// 3. 峰值统计：记录和提供网络下载/上传的峰值速度
// 4. 数据广播：将实时监控数据广播给所有连接的客户端
//
// 技术特点:
// - 使用 Go 协程 (goroutine) 实现异步监控
// - 使用通道 (channel) 安全地传递监控数据
// - 使用上下文 (context) 控制监控协程的生命周期
// - 使用互斥锁 (sync.RWMutex) 保证并发安全
// - 集成 vnstat 工具进行网络流量数据采集
// - 使用 Gin 框架处理 HTTP 请求
// - 使用 zerolog 进行结构化日志记录

package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

// handleHistoricalExport 导出所有历史带宽监控数据
// 功能：通过 vnstat 工具获取指定网络接口的历史带宽数据，并以 JSON 格式返回给客户端
// 参数：
//
//	c *gin.Context: Gin 框架上下文，用于处理 HTTP 请求和响应
//
// 返回值：无直接返回值，通过 c.JSON 和 c.Data 向客户端返回响应
// 错误处理：
//  1. 执行 vnstat 命令失败时返回 500 错误
//  2. 解析 vnstat 输出的 JSON 数据失败时返回 500 错误
//  3. 重新编码增强后的 JSON 数据失败时返回 500 错误
//
// 工作流程：
// 1. 获取客户端请求中的 "interface" 查询参数，如果为空则使用配置中的默认接口
// 2. 构建 vnstat 命令行参数，使用 "--json a" 获取所有历史数据
// 3. 执行 vnstat 命令，获取历史带宽数据
// 4. 解析 JSON 数据并添加服务器时间和时区信息
// 5. 重新编码增强后的 JSON 数据
// 6. 设置适当的响应头并返回数据给客户端
//
// 技术说明：
// - 使用 Go 的 os/exec 包执行外部命令 vnstat
// - 使用 json 包解析和编码 JSON 数据
// - 使用 Gin 框架处理 HTTP 请求和响应
// - 添加时区信息是为了确保客户端能够正确显示时间
func (a *Agent) handleHistoricalExport(c *gin.Context) {
	// 获取可选的接口参数
	iface := c.Query("interface")
	if iface == "" {
		iface = a.config.Interface
	}

	// 构建获取所有历史数据的 vnstat 命令
	args := []string{"--json", "a"}
	if iface != "" {
		args = append(args, "--iface", iface)
	}

	// 执行 vnstat 命令
	cmd := exec.Command("vnstat", args...)
	output, err := cmd.Output()
	if err != nil {
		log.Error().Err(err).Msg("导出历史数据失败")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "导出历史数据失败",
			"details": err.Error(),
		})
		return
	}

	// 解析 JSON 以添加时区信息
	var bandwidthData map[string]any
	if err := json.Unmarshal(output, &bandwidthData); err != nil {
		log.Error().Err(err).Msg("解析带宽数据 JSON 失败")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "解析带宽数据失败",
		})
		return
	}

	// 添加服务器时间信息用于时区处理
	now := time.Now()
	bandwidthData["server_time"] = now.Format(time.RFC3339)
	bandwidthData["server_time_unix"] = now.Unix()
	_, offset := now.Zone()
	bandwidthData["timezone_offset"] = offset // 与 UTC 的偏移量（秒）

	// 使用增强的信息重新编码
	enrichedOutput, err := json.Marshal(bandwidthData)
	if err != nil {
		log.Error().Err(err).Msg("编码带宽数据失败")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "编码数据失败",
		})
		return
	}

	// 设置 JSON 响应的适当头信息
	c.Header("Content-Type", "application/json")
	c.Header("Content-Disposition", "inline; filename=\"monitor-historical.json\"")

	// 返回增强的 JSON 数据
	c.Data(http.StatusOK, "application/json", enrichedOutput)
}

// runBandwidthMonitor 运行 vnstat 命令并将数据发送到广播通道
// 功能：启动 vnstat 实时监控，采集网络流量数据，并将其发送到监控数据通道
// 参数：
//
//	ctx context.Context: 上下文，用于控制监控协程的生命周期
//
// 返回值：无直接返回值
// 错误处理：
//  1. 创建 stdout 管道失败时记录错误并返回
//  2. 启动 vnstat 命令失败时记录错误并返回
//  3. 解析 JSON 数据失败时记录警告并跳过该数据
//  4. 扫描 stdout 失败时记录错误
//  5. vnstat 命令执行失败时记录错误
//
// 工作流程：
// 1. 构建 vnstat 命令行参数，使用 "--live --json" 获取实时数据
// 2. 创建带上下文的命令，以便可以通过上下文控制命令的终止
// 3. 设置标准输出管道，用于读取 vnstat 的输出
// 4. 启动 vnstat 命令
// 5. 使用 scanner 逐行读取 vnstat 的输出
// 6. 解析每行 JSON 数据并验证其格式
// 7. 更新网络下载/上传的峰值速度统计
// 8. 将解析后的数据发送到监控数据通道
// 9. 记录调试日志
// 10. 处理扫描错误和命令执行错误
//
// 技术说明：
// - 使用 Go 的 os/exec 包执行外部命令 vnstat
// - 使用 context.Context 控制命令的生命周期，实现优雅退出
// - 使用 bufio.Scanner 逐行读取命令输出
// - 使用 json 包解析 JSON 数据
// - 使用互斥锁 (peakMu) 保证并发安全地更新峰值统计
// - 使用通道 (monitorData) 安全地传递监控数据
// - 使用 select 语句发送数据到通道，避免通道满时阻塞
// - 这是一个长期运行的函数，通常在单独的 goroutine 中执行
func (a *Agent) runBandwidthMonitor(ctx context.Context) {
	// 构建 vnstat 命令
	args := []string{"--live", "--json"}
	if a.config.Interface != "" {
		args = append(args, "--iface", a.config.Interface)
	}

	cmd := exec.CommandContext(ctx, "vnstat", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Error().Err(err).Msg("创建 stdout 管道失败")
		return
	}

	if err := cmd.Start(); err != nil {
		log.Error().Err(err).Msg("启动 vnstat 失败")
		return
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()

		// 解析 JSON 以验证其格式
		var data MonitorLiveData
		if err := json.Unmarshal([]byte(line), &data); err != nil {
			log.Warn().Err(err).Str("line", line).Msg("解析带宽数据 JSON 失败")
			continue
		}

		// 跟踪峰值速度
		a.peakMu.Lock()
		now := time.Now()
		if data.Rx.Bytespersecond > a.peakRx {
			a.peakRx = data.Rx.Bytespersecond
			a.peakRxTimestamp = now
		}
		if data.Tx.Bytespersecond > a.peakTx {
			a.peakTx = data.Tx.Bytespersecond
			a.peakTxTimestamp = now
		}
		a.peakMu.Unlock()

		// 发送到广播器
		select {
		case a.monitorData <- line:
		default:
			// 通道已满，跳过
		}

		log.Trace().
			Str("rx", data.Rx.Ratestring).
			Str("tx", data.Tx.Ratestring).
			Msg("广播带宽监控数据")
	}

	if err := scanner.Err(); err != nil {
		log.Error().Err(err).Msg("扫描器错误")
	}

	if err := cmd.Wait(); err != nil {
		log.Error().Err(err).Msg("vnstat 命令执行失败")
	}
}

// handlePeakStats 返回峰值带宽统计信息
// 功能：提供网络下载和上传的峰值速度统计信息，包括峰值速度、人类可读的速度字符串和峰值出现时间
// 参数：
//
//	c *gin.Context: Gin 框架上下文，用于处理 HTTP 请求和响应
//
// 返回值：无直接返回值，通过 c.JSON 向客户端返回 JSON 格式的峰值统计信息
// 错误处理：无直接错误处理，所有操作都在并发安全的环境下执行
//
// 工作流程：
// 1. 获得读锁，确保并发安全地读取峰值统计信息
// 2. 创建 PeakStats 结构体实例，包含以下信息：
//   - 峰值下载速度（字节/秒）
//   - 峰值上传速度（字节/秒）
//   - 人类可读的峰值下载速度字符串
//   - 人类可读的峰值上传速度字符串
//   - 峰值下载速度出现的时间戳
//   - 峰值上传速度出现的时间戳
//   - 当前时间戳（作为数据更新时间）
//
// 3. 释放读锁
// 4. 使用 c.JSON 返回 JSON 格式的峰值统计信息
//
// 技术说明：
// - 使用读锁 (peakMu.RLock()) 保证并发安全地读取峰值统计信息
// - 使用 formatBytesPerSecond 函数将字节/秒转换为人类可读的格式（如 "1.23 Mbit/s"）
// - 使用 Gin 框架的 c.JSON 方法返回 JSON 格式的响应
// - 这是一个快速响应的函数，因为它只读取已缓存的数据，不执行任何耗时操作
func (a *Agent) handlePeakStats(c *gin.Context) {
	a.peakMu.RLock()
	stats := PeakStats{
		PeakRx:          a.peakRx,
		PeakTx:          a.peakTx,
		PeakRxString:    formatBytesPerSecond(a.peakRx),
		PeakTxString:    formatBytesPerSecond(a.peakTx),
		PeakRxTimestamp: a.peakRxTimestamp,
		PeakTxTimestamp: a.peakTxTimestamp,
		UpdatedAt:       time.Now(),
	}
	a.peakMu.RUnlock()

	c.JSON(http.StatusOK, stats)
}

// formatBytesPerSecond 将字节/秒转换为人类可读的字符串格式
// 功能：将原始的字节/秒数值转换为易于人类理解的格式（如 "1.23 Mbit/s"）
// 参数：
//
//	bytes int: 字节/秒数值
//
// 返回值：
//
//	string: 人类可读的速度字符串，如 "1.23 Mbit/s"
//
// 错误处理：无直接错误处理，所有输入值都会返回有效字符串
//
// 实现逻辑：
// 1. 处理特殊情况：如果输入为 0，则直接返回 "0 B/s"
// 2. 定义单位转换常量 k = 1024（二进制单位制）
// 3. 定义速度单位数组，从 "B/s"（字节/秒）到 "TiB/s"（太字节/秒）
// 4. 初始化计数器 i 和浮点字节数 bytesFloat
// 5. 使用循环将字节数转换为合适的单位：
//   - 当字节数大于等于 k 且未达到最大单位时
//   - 将字节数除以 k（转换为更大的单位）
//   - 递增单位索引 i
//
// 6. 使用 fmt.Sprintf 将转换后的字节数格式化为带有两位小数的字符串
// 7. 返回带有适当单位的人类可读速度字符串
//
// 技术说明：
// - 使用二进制单位制（1 KiB = 1024 B）而不是十进制单位制（1 KB = 1000 B）
// - 支持的单位范围：B/s（字节/秒）到 TiB/s（太字节/秒）
// - 使用 fmt.Sprintf 进行格式化，确保输出格式一致
// - 这是一个纯函数，没有副作用，适合在并发环境中使用
//
// 使用场景：
// - 在用户界面中显示网络速度
// - 在日志中记录人类可读的速度信息
// - 在 API 响应中提供易于理解的速度数据
func formatBytesPerSecond(bytes int) string {
	if bytes == 0 {
		return "0 B/s"
	}

	const k = 1024
	sizes := []string{"B/s", "KiB/s", "MiB/s", "GiB/s", "TiB/s"}

	i := 0
	bytesFloat := float64(bytes)
	for bytesFloat >= k && i < len(sizes)-1 {
		bytesFloat /= k
		i++
	}

	return fmt.Sprintf("%.2f %s", bytesFloat, sizes[i])
}
