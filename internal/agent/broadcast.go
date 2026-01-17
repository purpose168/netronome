// Copyright (c) 2024-2025, s0up 和 autobrr 贡献者.
// SPDX-License-Identifier: GPL-2.0-or-later
//
// Netronome SSE 广播模块
// 包名: agent
// 功能: 实现基于 Server-Sent Events (SSE) 的实时数据广播功能
// 作者: s0up 和 autobrr 贡献者
// 创建日期: 2024
// 许可证: GPL-2.0-or-later
//
// 主要功能:
// 1. SSE 连接处理：处理客户端的 SSE 连接请求
// 2. 客户端管理：注册、注销和跟踪连接的 SSE 客户端
// 3. 数据广播：将实时监控数据广播给所有连接的客户端
// 4. 优雅关闭：处理连接断开和资源清理
//
// 技术特点:
// - 使用 Server-Sent Events (SSE) 技术实现实时数据推送
// - 使用 Go 通道 (channel) 安全地传递数据
// - 使用互斥锁 (sync.RWMutex) 保证并发安全
// - 使用 Gin 框架的 Stream 功能实现 SSE 响应
// - 支持优雅关闭和资源清理
// - 高并发设计，支持多个客户端同时连接

package agent

import (
	"context"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
)

// handleSSE 处理基于 Server-Sent Events (SSE) 的实时数据流式传输连接
// 功能：建立并维护 SSE 连接，为客户端提供实时数据推送服务
// 参数：
//
//	c *gin.Context: Gin 框架上下文，用于处理 HTTP 请求和响应
//
// 返回值：无直接返回值，通过 c.Stream 向客户端持续发送 SSE 事件
// 错误处理：
//  1. 如果 stream 参数不是 "live-data"，返回 400 Bad Request 错误
//
// SSE 技术说明：
// Server-Sent Events (SSE) 是一种基于 HTTP 的单向通信协议，允许服务器主动向客户端推送实时数据
// - 客户端通过普通 HTTP GET 请求建立连接
// - 服务器保持连接打开，随时发送事件数据
// - 数据格式为 UTF-8 文本，包含事件类型和数据内容
// - 非常适合实时监控、通知推送等场景
// - 相比 WebSocket，SSE 更简单，不需要专门的协议支持
//
// 工作流程：
// 1. 验证 stream 参数，确保其值为 "live-data"
// 2. 创建客户端通道 (clientChan)，用于接收要发送给客户端的数据
// 3. 注册客户端：将客户端通道添加到 Agent 的 clients 映射中
// 4. 设置资源清理函数：在连接断开时清理客户端通道和映射
// 5. 使用 c.Stream 建立 SSE 响应流：
//   - 监听客户端通道，接收要发送的数据
//   - 使用 c.SSEvent 发送 SSE 事件
//   - 监听请求上下文，在连接断开时停止流
//
// 并发安全：
// - 使用互斥锁 (clientsMu) 保证并发安全地访问和修改 clients 映射
// - 客户端通道是 goroutine 安全的，用于在不同 goroutine 之间传递数据
//
// 资源管理：
// - 使用 defer 函数确保在函数返回时清理资源
// - 从 clients 映射中删除客户端通道
// - 关闭客户端通道，防止资源泄漏
func (a *Agent) handleSSE(c *gin.Context) {
	stream := c.Query("stream")
	if stream != "live-data" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 stream 参数"})
		return
	}

	// 创建客户端通道
	clientChan := make(chan string, 100)

	// 注册客户端
	a.clientsMu.Lock()
	a.clients[clientChan] = true
	a.clientsMu.Unlock()

	// 连接断开时清理资源
	defer func() {
		a.clientsMu.Lock()
		delete(a.clients, clientChan)
		a.clientsMu.Unlock()
		close(clientChan)
	}()

	// 建立 SSE 流
	c.Stream(func(w io.Writer) bool {
		select {
		case data := <-clientChan:
			// 发送 SSE 事件
			c.SSEvent("message", data)
			return true // 继续流
		case <-c.Request.Context().Done():
			return false // 停止流，连接断开
		}
	})
}

// broadcaster 将监控数据分发给所有连接的 SSE 客户端
// 功能：接收监控数据并将其广播给所有已注册的 SSE 客户端
// 参数：
//
//	ctx context.Context: 上下文，用于控制广播协程的生命周期
//
// 返回值：无直接返回值
// 错误处理：无直接错误处理，通过上下文控制协程的退出
//
// 广播机制说明：
// - 使用中心监控数据通道 (a.monitorData) 接收来自各个数据源的监控数据
// - 遍历所有已注册的客户端通道，将数据发送给每个客户端
// - 使用非阻塞发送方式，避免因单个客户端问题导致整个广播系统阻塞
// - 支持通过上下文优雅地停止广播协程
//
// 工作流程：
// 1. 进入无限循环，等待监控数据或上下文结束信号
// 2. 监听监控数据通道 (a.monitorData)，接收要广播的数据
// 3. 获取读锁，确保并发安全地遍历客户端映射
// 4. 遍历所有客户端通道：
//   - 使用 select 语句尝试将数据发送到客户端通道
//   - 如果客户端通道已满，跳过该客户端（防止阻塞）
//
// 5. 释放读锁
// 6. 如果接收到上下文结束信号，退出循环并结束协程
//
// 并发安全考虑：
// - 使用读锁 (clientsMu.RLock()) 保证并发安全地遍历 clients 映射
// - 读锁允许多个协程同时读取映射，提高并发性能
// - 使用非阻塞发送 (select with default) 避免客户端问题影响整个广播系统
// - 客户端通道是 goroutine 安全的，用于在不同 goroutine 之间传递数据
//
// 技术说明：
// - 这是一个长期运行的函数，通常在单独的 goroutine 中执行
// - 使用 context.Context 控制协程的生命周期，实现优雅退出
// - 使用 for 循环和 select 语句构建事件驱动的广播系统
// - 支持高并发场景，能够处理大量客户端连接
// - 容错设计：单个客户端通道满不会影响其他客户端的数据接收
func (a *Agent) broadcaster(ctx context.Context) {
	for {
		select {
		case data := <-a.monitorData:
			a.clientsMu.RLock()
			for client := range a.clients {
				select {
				case client <- data:
				default:
					// 客户端通道已满，跳过
				}
			}
			a.clientsMu.RUnlock()
		case <-ctx.Done():
			return
		}
	}
}
