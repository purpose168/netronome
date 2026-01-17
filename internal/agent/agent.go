// Copyright (c) 2024-2025, s0up 和 autobrr 贡献者.
// SPDX-License-Identifier: GPL-2.0-or-later
//
// Netronome 监控代理模块
// 包名: agent
// 功能: 提供网络性能监控代理的核心功能，支持 SSE（服务器发送事件）和 Tailscale
// 作者: s0up 和 autobrr 贡献者
// 创建日期: 2024
// 许可证: GPL-2.0-or-later

package agent

import (
	// Go 标准库包
	"context"  // 上下文管理，用于控制 goroutine 和请求生命周期
	"fmt"      // 格式化输入输出
	"net/http" // HTTP 客户端和服务器实现
	"time"     // 时间和定时器功能

	// 第三方库
	"github.com/rs/zerolog/log" // 结构化日志库

	// 内部包
	"github.com/autobrr/netronome/internal/config" // 配置管理
)

// New 创建一个新的 Agent 实例
// 参数:
//
//	cfg: 代理配置
//
// 返回值:
//
//	*Agent: 新创建的 Agent 实例
func New(cfg *config.AgentConfig) *Agent {
	return &Agent{
		config:      cfg,
		clients:     make(map[chan string]bool),
		monitorData: make(chan string, 100),
	}
}

// NewWithTailscale 创建一个支持 Tailscale 的新 Agent 实例
// 参数:
//
//	cfg: 代理配置
//	tsCfg: Tailscale 配置
//
// 返回值:
//
//	*Agent: 新创建的 Agent 实例
func NewWithTailscale(cfg *config.AgentConfig, tsCfg *config.TailscaleConfig) *Agent {
	return &Agent{
		config:          cfg,
		tailscaleConfig: tsCfg,
		clients:         make(map[chan string]bool),
		monitorData:     make(chan string, 100),
		useTailscale:    tsCfg != nil && tsCfg.IsAgentMode(),
	}
}

// Start 启动代理服务器
// 参数:
//
//	ctx: 上下文，用于控制代理服务器的生命周期
//
// 返回值:
//
//	error: 如果发生错误则返回错误信息，否则返回 nil
func (a *Agent) Start(ctx context.Context) error {
	// 如果启用了 Tailscale，则确定使用的方法
	if a.useTailscale && a.tailscaleConfig != nil {
		method, err := a.tailscaleConfig.GetEffectiveMethod()
		if err != nil {
			return fmt.Errorf("确定 Tailscale 方法失败: %w", err)
		}

		switch method {
		case "host":
			log.Info().Msg("使用主机的 tailscaled...")
			return a.startWithHostTailscale(ctx)
		case "tsnet":
			log.Info().Msg("使用嵌入式 tsnet...")
			return a.startWithTailscale(ctx)
		default:
			return fmt.Errorf("意外的 Tailscale 方法: %s", method)
		}
	}

	// 在单独的 goroutine 中启动带宽监控
	go a.runBandwidthMonitor(ctx)

	// 在单独的 goroutine 中启动广播器
	go a.broadcaster(ctx)

	// 设置路由
	router := a.setupRoutes()

	// 构建服务器地址
	addr := fmt.Sprintf("%s:%d", a.config.Host, a.config.Port)
	server := &http.Server{
		Addr:    addr,   // 服务器监听地址
		Handler: router, // HTTP 请求处理程序
	}

	// 处理优雅关闭
	go func() {
		<-ctx.Done() // 等待上下文取消信号
		log.Info().Msg("正在关闭代理服务器...")
		// 创建一个 5 秒超时的上下文，用于优雅关闭服务器
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel() // 确保在函数结束时取消上下文
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Error().Err(err).Msg("服务器优雅关闭失败")
		}
	}()

	// 根据是否有 API 密钥，记录不同的启动信息
	if a.config.APIKey != "" {
		log.Info().Str("addr", addr).Msg("正在启动带 API 密钥认证的监控 SSE 代理")
	} else {
		log.Info().Str("addr", addr).Msg("正在启动不带认证的监控 SSE 代理")
	}

	// 启动 HTTP 服务器
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		// 如果服务器启动失败（不是正常关闭），则返回错误
		return fmt.Errorf("启动服务器失败: %w", err)
	}

	return nil
}
