// Copyright (c) 2024-2025, s0up 和 autobrr 贡献者.
// SPDX-License-Identifier: GPL-2.0-or-later

// Netronome Tailscale 集成模块
// 包名: agent
// 功能: 实现与 Tailscale 网络的集成，支持两种连接方式：
//       1. tsnet 模式：独立运行 Tailscale 节点
//       2. host 模式：使用主机已有的 tailscaled 服务
// 作者: s0up 和 autobrr 贡献者
// 创建日期: 2024
// 许可证: GPL-2.0-or-later

// Tailscale 相关概念解释：
// 1. Tailscale：一种基于 WireGuard 的零配置 VPN 服务，可轻松创建安全的点对点网络
//    - 自动处理 NAT 穿透、加密和身份验证
//    - 为每个节点分配唯一的 Tailscale IP 地址
//    - 支持通过 MagicDNS 访问节点
//
// 2. tsnet 模式：
//    - 独立运行的 Tailscale 节点，不依赖主机的 tailscaled 服务
//    - 适合容器化环境或没有安装 tailscaled 的系统
//    - 需要配置认证密钥（AuthKey）进行注册
//    - 状态存储在本地目录中
//
// 3. host 模式：
//    - 复用主机已有的 tailscaled 服务连接
//    - 不需要额外的认证密钥
//    - 适合已经安装并配置了 tailscaled 的系统
//
// 4. MagicDNS：
//    - Tailscale 提供的 DNS 服务，允许通过节点名称访问设备
//    - 格式：<节点名>.<Tailnet 域名>.ts.net
//    - 代码中会移除域名后缀，只保留节点名称
//
// 5. Ephemeral 节点：
//    - 临时节点，断开连接后会从 Tailnet 中移除
//    - 适合临时设备或自动化场景
//
// 6. AuthKey：
//    - 用于自动注册 Tailscale 节点的密钥
//    - 可以在 Tailscale 管理界面生成
//    - 支持一次性或可重复使用的密钥
//
// 7. 状态目录：
//    - 存储 tsnet 节点的状态信息和证书
//    - 需要有写入权限
//    - 支持 ~ 符号表示用户主目录

package agent

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"tailscale.com/tsnet"

	ts "github.com/autobrr/netronome/internal/tailscale"
)

// startWithTailscale 以 Tailscale tsnet 模式启动代理服务器
// 该函数使用 Tailscale 的 tsnet 库在独立模式下运行 Tailscale 节点
// 这意味着代理将创建自己的 Tailscale 节点，而不依赖主机上现有的 tailscaled 服务
//
// 参数：
//
//	ctx: 上下文对象，用于控制代理的生命周期
//
// 返回值：
//
//	error: 如果启动过程中发生错误，则返回错误信息；否则返回 nil
//
// 主要功能：
// 1. 启动带宽监控（在单独的 goroutine 中）
// 2. 启动广播器（在单独的 goroutine 中）
// 3. 配置 Tailscale 节点名称和状态目录
// 4. 创建并配置 tsnet 服务器实例
// 5. 启动 tsnet 服务器
// 6. 设置 HTTP 路由
// 7. 在 Tailscale 网络上监听指定端口
// 8. 设置优雅关闭机制
// 9. 记录 Tailscale 节点状态和连接信息
func (a *Agent) startWithTailscale(ctx context.Context) error {
	// 启动带宽监控（在单独的 goroutine 中运行）
	go a.runBandwidthMonitor(ctx)

	// 启动广播器（在单独的 goroutine 中运行）
	go a.broadcaster(ctx)

	// 配置 tsnet 节点名称
	hostname := a.tailscaleConfig.Hostname
	if hostname == "" {
		// 如果未指定主机名，则生成默认主机名
		sysHostname, _ := os.Hostname()
		hostname = fmt.Sprintf("netronome-agent-%s", sysHostname)
	}

	// 展开状态目录路径（处理 ~ 符号）
	stateDir := a.tailscaleConfig.StateDir
	if strings.HasPrefix(stateDir, "~/") {
		home, _ := os.UserHomeDir()
		stateDir = filepath.Join(home, stateDir[2:])
	}

	// 如果状态目录不存在，则创建它
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return fmt.Errorf("创建 tsnet 状态目录失败: %w", err)
	}

	a.tsnetServer = &tsnet.Server{
		Dir:       stateDir,
		Hostname:  hostname,
		AuthKey:   a.tailscaleConfig.AuthKey,
		Ephemeral: a.tailscaleConfig.Ephemeral,
		Logf: func(format string, args ...any) {
			log.Debug().Msgf("[tsnet] "+format, args...) // 配置日志输出
		},
	}

	// 如果指定了控制 URL，则设置它（用于自定义 Tailscale 控制服务器）
	if a.tailscaleConfig.ControlURL != "" {
		a.tsnetServer.ControlURL = a.tailscaleConfig.ControlURL
	}

	// 启动 tsnet 服务器
	log.Info().Str("hostname", hostname).Msg("启动 Tailscale 节点...")
	if err := a.tsnetServer.Start(); err != nil {
		return fmt.Errorf("启动 tsnet 失败: %w", err)
	}

	// 设置 HTTP 路由
	router := a.setupRoutes()

	// 在 Tailscale 网络上监听指定端口
	port := a.config.Port
	if a.tailscaleConfig.AgentPort > 0 {
		port = a.tailscaleConfig.AgentPort // 使用 Tailscale 特定的端口（如果指定） // 使用 Tailscale 特定的端口（如果指定）
	}
	ln, err := a.tsnetServer.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("在 Tailscale 网络上监听失败: %w", err)
	}

	server := &http.Server{
		Handler: router,
	}

	// 设置优雅关闭机制
	go func() {
		<-ctx.Done() // 等待上下文取消信号 // 等待上下文取消信号
		log.Info().Msg("正在关闭 Tailscale 代理服务器...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Error().Err(err).Msg("优雅关闭服务器失败")
		}
		if a.tsnetServer != nil {
			a.tsnetServer.Close() // 关闭 tsnet 服务器
		}
	}()

	// 获取 Tailscale 本地客户端以获取状态信息
	localClient, err := a.tsnetServer.LocalClient()
	if err != nil {
		log.Warn().Err(err).Msg("获取 Tailscale 本地客户端失败")
	} else {
		status, err := localClient.Status(ctx)
		if err != nil {
			log.Warn().Err(err).Msg("获取 Tailscale 状态失败")
		} else if status.Self != nil {
			logEvent := log.Info().
				Str("hostname", hostname).
				Int("port", port)

			if len(status.Self.TailscaleIPs) > 0 {
				logEvent = logEvent.Str("tailscale_ip", status.Self.TailscaleIPs[0].String())
			}

			logEvent.Msg("监控 SSE 代理在 Tailscale 网络上监听")

			if a.config.APIKey != "" {
				log.Info().Msg("API 密钥认证已启用")
			}
		}
	}

	if err := server.Serve(ln); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("服务器服务失败: %w", err)
	}

	return nil
}

// startWithHostTailscale 使用主机已有的 tailscaled 服务启动代理
// 该函数使用主机上已运行的 tailscaled 服务来连接 Tailscale 网络
// 与 tsnet 模式不同，这种模式不创建独立的 Tailscale 节点，而是复用主机的 Tailscale 连接
//
// 参数：
//
//	ctx: 上下文对象，用于控制代理的生命周期
//
// 返回值：
//
//	error: 如果启动过程中发生错误，则返回错误信息；否则返回 nil
//
// 主要功能：
// 1. 启动带宽监控（在单独的 goroutine 中）
// 2. 启动广播器（在单独的 goroutine 中）
// 3. 获取主机的 Tailscale 客户端连接
// 4. 获取主机的 Tailscale 节点信息
// 5. 设置 HTTP 路由
// 6. 在 Tailscale 网络上监听指定端口
// 7. 设置优雅关闭机制
// 8. 记录代理在 Tailscale 网络上的监听信息
func (a *Agent) startWithHostTailscale(ctx context.Context) error {
	// 启动带宽监控（在单独的 goroutine 中运行）
	go a.runBandwidthMonitor(ctx)
	// 启动广播器（在单独的 goroutine 中运行）
	go a.broadcaster(ctx)

	// 获取主机的 Tailscale 客户端
	hostClient, err := ts.GetHostClient()
	if err != nil {
		return fmt.Errorf("连接主机 tailscaled 服务失败: %w", err)
	}

	// 获取主机的 Tailscale 节点信息
	hostname, ips, err := ts.GetSelfInfo(hostClient)
	if err != nil {
		return fmt.Errorf("获取 Tailscale 信息失败: %w", err)
	}

	log.Info().
		Str("hostname", hostname).
		Strs("tailscale_ips", ips).
		Msg("使用主机的 tailscaled 服务运行代理")

	// 启动普通服务器并绑定到 Tailscale IP
	router := a.setupRoutes()
	server := &http.Server{
		Handler: router,
	}

	// 在 Tailscale 网络上监听指定端口
	port := a.config.Port
	if a.tailscaleConfig.AgentPort > 0 {
		port = a.tailscaleConfig.AgentPort
	}
	ln, err := ts.ListenOnTailscale(hostClient, port)
	if err != nil {
		// 如果无法绑定到特定的 Tailscale IP，则回退到所有接口
		log.Warn().Err(err).Msg("绑定到 Tailscale IP 失败，使用所有接口")
		addr := fmt.Sprintf("%s:%d", a.config.Host, port)
		ln, err = net.Listen("tcp", addr)
		if err != nil {
			return fmt.Errorf("监听失败: %w", err)
		}
	}

	// 设置优雅关闭机制
	go func() {
		<-ctx.Done()
		log.Info().Msg("正在关闭代理服务器...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Error().Err(err).Msg("优雅关闭服务器失败")
		}
	}()

	log.Info().
		Str("address", ln.Addr().String()).
		Int("port", port).
		Msg("监控 SSE 代理通过主机的 tailscaled 服务监听")

	if a.config.APIKey != "" {
		log.Info().Msg("API 密钥认证已启用")
	}

	if err := server.Serve(ln); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("服务器服务失败: %w", err)
	}

	return nil
}

// GetTailscaleStatus 获取当前 Tailscale 连接状态
// 该函数返回代理的 Tailscale 连接状态信息
// 支持两种模式的状态查询：tsnet 模式和 host 模式
//
// 返回值：
//
//	map[string]any: 包含 Tailscale 状态信息的映射
//	error: 如果获取状态过程中发生错误，则返回错误信息；否则返回 nil
//
// 返回的状态信息包括：
//   - enabled: 是否启用 Tailscale
//   - status: 连接状态（disabled, connecting, connected, error）
//   - method: 使用的连接方法（tsnet, host）
//   - hostname: Tailscale 节点名称
//   - tailscale_ips: Tailscale IP 地址列表
//   - online: 是否在线
//   - error: 错误信息（如果有）
//
// 主要功能：
// 1. 检查 Tailscale 是否启用
// 2. 获取有效的连接方法
// 3. 根据连接方法获取对应的状态信息
// 4. 处理 tsnet 模式的状态查询
// 5. 处理 host 模式的状态查询
// 6. 返回格式化的状态信息
func (a *Agent) GetTailscaleStatus() (map[string]any, error) {
	if !a.useTailscale || a.tailscaleConfig == nil {
		return map[string]any{
			"enabled": false,
			"status":  "disabled",
		}, nil
	}

	method, _ := a.tailscaleConfig.GetEffectiveMethod()
	result := map[string]any{
		"enabled": true,
		"method":  method,
	}

	// Handle tsnet mode
	if method == "tsnet" && a.tsnetServer != nil {
		localClient, err := a.tsnetServer.LocalClient()
		if err != nil {
			return nil, fmt.Errorf("获取 Tailscale 本地客户端失败: %w", err)
		}

		status, err := localClient.Status(context.Background())
		if err != nil {
			return nil, fmt.Errorf("获取 Tailscale 状态失败: %w", err)
		}

		if status.Self != nil {
			// 使用实际的 Tailscale 机器名称（去掉 DNSName 的后缀）
			hostname := status.Self.DNSName
			// 移除 MagicDNS 后缀，只保留机器名称
			if hostname != "" && strings.Contains(hostname, ".") {
				hostname = strings.Split(hostname, ".")[0]
			}
			// 如果 DNSName 为空，则回退到 HostName
			if hostname == "" {
				hostname = status.Self.HostName
			}
			result["hostname"] = hostname

			var ips []string
			for _, ip := range status.Self.TailscaleIPs {
				ips = append(ips, ip.String())
			}
			result["tailscale_ips"] = ips
			result["online"] = status.Self.Online
			result["status"] = "connected"
		} else {
			// 如果未连接，则回退到配置的主机名
			result["hostname"] = a.tsnetServer.Hostname
			result["status"] = "connecting"
		}
	} else if method == "host" {
		// 处理 host 模式的状态查询
		hostClient, err := ts.GetHostClient()
		if err != nil {
			result["status"] = "host_unavailable"
			result["error"] = err.Error()
		} else {
			hostname, ips, err := ts.GetSelfInfo(hostClient)
			if err != nil {
				result["status"] = "error"
				result["error"] = err.Error()
			} else {
				result["hostname"] = hostname
				result["tailscale_ips"] = ips
				result["status"] = "connected"
			}
		}
	}

	return result, nil
}
