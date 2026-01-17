// Copyright (c) 2024-2025, s0up 和 autobrr 贡献者.
// SPDX-License-Identifier: GPL-2.0-or-later
//
// Netronome 主程序入口
// 包名: main
// 功能: 网络性能测试和监控工具的主命令行界面
// 作者: s0up 和 autobrr 贡献者
// 创建日期: 2024
// 许可证: GPL-2.0-or-later

package main

import (
	// Go 标准库包
	"bytes"         // 字节操作库
	"context"       // 上下文管理，用于控制 goroutine 和请求生命周期
	"fmt"           // 格式化输入输出
	"io"            // 输入输出接口定义
	"net/http"      // HTTP 客户端和服务器实现
	"os"            // 操作系统功能接口
	"os/signal"     // 系统信号处理
	"path/filepath" // 文件路径操作
	"syscall"       // 系统调用接口
	"time"          // 时间和定时器功能

	// 第三方库
	"github.com/joho/godotenv"  // 环境变量加载库
	"github.com/rs/zerolog/log" // 结构化日志库
	"github.com/spf13/cobra"    // 命令行界面框架
	"golang.org/x/term"         // 终端相关功能，如密码输入

	// 内部包
	"github.com/autobrr/netronome/internal/agent"              // 监控代理实现
	"github.com/autobrr/netronome/internal/config"             // 配置管理
	"github.com/autobrr/netronome/internal/database"           // 数据库操作
	"github.com/autobrr/netronome/internal/logger"             // 日志初始化
	"github.com/autobrr/netronome/internal/monitor"            // 网络监控服务
	"github.com/autobrr/netronome/internal/notifications"      // 通知系统
	"github.com/autobrr/netronome/internal/scheduler"          // 调度器服务
	"github.com/autobrr/netronome/internal/server"             // HTTP 服务器实现
	"github.com/autobrr/netronome/internal/speedtest"          // 网速测试功能
	appversion "github.com/autobrr/netronome/internal/version" // 应用版本信息
)

var (
	// 构建时变量 (通过 ldflags 设置)
	version   = "dev"     // 版本号
	buildTime = "unknown" // 构建时间
	commit    = "unknown" // Git 提交哈希值

	// 配置路径
	configPath string // 配置文件路径

	// 根命令
	rootCmd = &cobra.Command{
		Use:   "netronome",
		Short: "Netronome 是一款网络性能测试和监控工具",
		Long: `Netronome 是一款网络性能测试和监控工具，可帮助您
长期跟踪和分析网络性能。`,
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true, // 禁用默认的 completion 命令
		},
	}

	// 服务命令
	serveCmd = &cobra.Command{
		Use:   "serve",
		Short: "启动 Netronome 服务器",
		RunE:  runServer, // 命令执行函数
	}

	// 生成配置命令
	generateConfigCmd = &cobra.Command{
		Use:   "generate-config",
		Short: "生成默认配置文件",
		RunE:  generateConfig, // 命令执行函数
	}

	// 修改密码命令
	changePasswordCmd = &cobra.Command{
		Use:   "change-password [username]",
		Short: "修改用户密码",
		Args:  cobra.ExactArgs(1), // 要求恰好一个参数（用户名）
		RunE:  changePassword,     // 命令执行函数
	}

	// 创建用户命令
	createUserCmd = &cobra.Command{
		Use:   "create-user [username]",
		Short: "创建新用户",
		Args:  cobra.ExactArgs(1), // 要求恰好一个参数（用户名）
		RunE:  createUser,         // 命令执行函数
	}

	// 代理命令
	agentCmd = &cobra.Command{
		Use:   "agent",
		Short: "启动监控代理",
		Long: `启动监控代理，用于广播带宽和系统使用数据。
此代理可由远程 Netronome 服务器监控。

示例：
  # 使用默认设置启动代理
  netronome agent

  # 使用自定义端口和 API 密钥启动代理
  netronome agent --port 8300 --api-key mysecretkey

  # 启动代理监控特定网络接口
  netronome agent --interface eth0

  # 使用 Tailscale 启动代理以实现安全连接
  netronome agent --tailscale

  # 使用 Tailscale 和自定义主机名启动代理
  netronome agent --tailscale --tailscale-hostname my-server

  # 使用 Tailscale 和认证密钥启动代理（无头部署）
  netronome agent --tailscale --tailscale-auth-key tskey-auth-xxx

  # 使用配置文件
  netronome agent --config /etc/netronome/agent.toml`,
		RunE: runAgent,
	}
)

func init() {
	// 尝试加载 .env 文件（如果存在）
	if err := godotenv.Load(); err != nil {
		// 找不到 .env 文件，忽略错误
	}

	// 添加持久化标志：所有子命令都能使用的配置文件路径参数
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "配置文件路径")

	// 为 agent 命令添加参数
	agentCmd.Flags().StringP("host", "H", "0.0.0.0", "要绑定的 IP 地址")
	agentCmd.Flags().IntP("port", "p", 8200, "要监听的端口")
	agentCmd.Flags().StringP("interface", "i", "", "要监控的网络接口（留空表示监控所有接口）")
	agentCmd.Flags().StringP("api-key", "k", "", "用于认证的 API 密钥")
	agentCmd.Flags().StringP("log-level", "l", "", "日志级别 (trace, debug, info, warn, error)")
	agentCmd.Flags().StringSlice("disk-include", []string{}, "要监控的额外磁盘挂载点（例如：/mnt/storage）")
	agentCmd.Flags().StringSlice("disk-exclude", []string{}, "要排除监控的磁盘挂载点（例如：/boot）")
	agentCmd.Flags().Bool("tailscale", false, "启用 Tailscale 安全连接")
	agentCmd.Flags().String("tailscale-hostname", "", "自定义 Tailscale 主机名（默认：netronome-agent-<hostname>）")
	agentCmd.Flags().String("tailscale-auth-key", "", "用于自动注册的 Tailscale 认证密钥")
	agentCmd.Flags().String("tailscale-state-dir", "", "Tailscale 状态目录（默认：~/.config/netronome/tsnet）")
	agentCmd.Flags().String("tailscale-method", "auto", "Tailscale 方法：auto, host 或 tsnet（默认：auto）")

	// 将所有子命令添加到根命令
	rootCmd.AddCommand(serveCmd)          // 服务命令
	rootCmd.AddCommand(generateConfigCmd) // 生成配置命令
	rootCmd.AddCommand(changePasswordCmd) // 修改密码命令
	rootCmd.AddCommand(createUserCmd)     // 创建用户命令
	rootCmd.AddCommand(agentCmd)          // 代理命令
	rootCmd.AddCommand(updateCmd)         // 更新命令
	rootCmd.AddCommand(versionCmd)        // 版本命令
}

// main 函数是程序的入口点
func main() {
	// 使用构建时变量初始化版本信息
	appversion.Set(version, buildTime, commit)
	SetVersion(version, buildTime, commit) // 设置命令行版本信息

	// 执行根命令
	if err := rootCmd.Execute(); err != nil {
		// 如果命令执行失败，将错误信息输出到标准错误流并退出
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// generateConfig 生成默认配置文件
// 参数:
//
//	cmd: Cobra 命令对象
//	args: 命令行参数列表
//
// 返回值:
//
//	error: 如果发生错误则返回错误信息，否则返回 nil
func generateConfig(cmd *cobra.Command, args []string) error {
	// 初始化日志（使用默认设置）
	logger.Init(config.LoggingConfig{Level: "info"}, config.ServerConfig{}, false)

	// 创建新的默认配置对象
	cfg := config.New()

	// 如果未指定配置文件路径，则自动确定默认路径
	if configPath == "" {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			// 尝试使用 ~/.config/netronome 目录
			configDir := filepath.Join(homeDir, ".config")
			netronomeDir := filepath.Join(configDir, config.AppName)
			if err := os.MkdirAll(netronomeDir, 0755); err == nil {
				configPath = filepath.Join(netronomeDir, "config.toml")
			} else {
				// 回退到平台特定的用户配置目录
				if configDir, err := os.UserConfigDir(); err == nil {
					netronomeDir := filepath.Join(configDir, config.AppName)
					if err := os.MkdirAll(netronomeDir, 0755); err == nil {
						configPath = filepath.Join(netronomeDir, "config.toml")
					} else {
						log.Warn().
							Err(err).
							Msg("无法创建配置目录，回退到当前工作目录")
						configPath = "config.toml"
					}
				} else {
					log.Warn().
						Err(err).
						Msg("无法确定用户配置目录，回退到当前工作目录")
					configPath = "config.toml"
				}
			}
		}
	}

	// 检查配置文件是否已存在
	if _, err := os.Stat(configPath); err == nil {
		return fmt.Errorf("配置文件已存在于 %s", configPath)
	}

	// 创建配置文件
	f, err := os.Create(configPath)
	if err != nil {
		return fmt.Errorf("创建配置文件失败: %w", err)
	}
	defer f.Close() // 确保文件在函数结束时关闭

	// 将默认配置写入文件（TOML格式）
	if err := cfg.WriteToml(f); err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}

	// 记录成功日志
	log.Info().Str("path", configPath).Msg("已生成默认配置文件")
	return nil
}

// runServer 启动 Netronome 服务器
// 参数:
//
//	cmd: Cobra 命令对象
//	args: 命令行参数列表
//
// 返回值:
//
//	error: 如果发生错误则返回错误信息，否则返回 nil
func runServer(cmd *cobra.Command, args []string) error {
	// 首先使用默认设置初始化日志（静默模式）
	logger.Init(config.LoggingConfig{Level: "info"}, config.ServerConfig{}, true)

	// 确保配置文件存在，如果不存在则创建默认配置
	configPath, err := config.EnsureConfig(configPath)
	if err != nil {
		return fmt.Errorf("确保配置文件存在失败: %w", err)
	}

	// 加载配置
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("加载配置失败: %w", err)
	}

	// 使用加载的配置重新初始化日志（非静默模式）
	logger.Init(cfg.Logging, cfg.Server, false)

	// 初始化数据库连接
	db := database.New(cfg.Database)
	if err := db.InitializeTables(context.Background()); err != nil {
		return fmt.Errorf("初始化数据库表失败: %w", err)
	}

	// 创建通知服务
	notifier, err := notifications.NewNotifier(db)
	if err != nil {
		return fmt.Errorf("创建通知服务失败: %w", err)
	}

	// 创建网速测试服务
	speedtestSvc := speedtest.New(db, cfg.SpeedTest, notifier, cfg)

	// 创建丢包服务变量（用于后续初始化）
	var packetLossService *speedtest.PacketLossService

	// 如果启用了丢包监控，则创建丢包服务
	if cfg.PacketLoss.Enabled {
		// 这里先创建服务，稍后再设置实际的广播器
		packetLossService = speedtest.NewPacketLossService(
			db,
			notifier,
			nil, // 广播器稍后设置
			cfg.PacketLoss.MaxConcurrentMonitors,
			cfg.PacketLoss.PrivilegedMode,
		)
	}

	// 创建监控服务变量（用于后续初始化）
	var monitorService *monitor.Service

	// 创建调度器服务，用于管理定时任务
	schedulerSvc := scheduler.New(db, speedtestSvc, packetLossService, notifier)

	// 创建服务器处理程序，整合所有服务
	serverHandler := server.NewServer(
		speedtestSvc,
		db,
		schedulerSvc,
		cfg,
		packetLossService,
		monitorService,
		notifier,
	)

	// 设置网速测试服务的广播更新函数
	speedtestSvc.SetBroadcastUpdate(serverHandler.BroadcastUpdate)
	speedtestSvc.SetBroadcastTracerouteUpdate(serverHandler.BroadcastTracerouteUpdate)

	// 设置丢包服务的广播器（如果已启用）
	if packetLossService != nil {
		packetLossService.SetBroadcast(serverHandler.BroadcastPacketLossUpdate)
		packetLossService.SetScheduler(schedulerSvc)

		// 不要在这里启动监控器 - 让调度器来处理
	}

	// 如果启用了监控服务，则创建并设置监控服务
	if cfg.Monitor.Enabled {
		// 如果启用了自动发现，则使用支持 Tailscale 的监控服务
		// 这适用于 host 和 tsnet 两种模式
		if cfg.Tailscale.IsServerDiscoveryMode() {
			monitorService = monitor.NewServiceWithTailscale(
				db,
				&cfg.Monitor,
				&cfg.Tailscale,
				serverHandler.BroadcastMonitorUpdate,
				notifier,
			)
			method, _ := cfg.Tailscale.GetEffectiveMethod()
			log.Info().Str("method", method).Msg("监控服务已创建，支持 Tailscale 发现")
		} else {
			monitorService = monitor.NewService(
				db,
				&cfg.Monitor,
				serverHandler.BroadcastMonitorUpdate,
				notifier,
			)
		}
		// 将监控服务设置到服务器处理程序中
		serverHandler.SetMonitorService(monitorService)

		// 启动监控服务
		if err := monitorService.Start(); err != nil {
			log.Error().Err(err).Msg("启动监控服务失败")
		}
	}

	// 初始化服务器（注册路由和静态文件）
	serverHandler.Initialize()

	// 启动调度器
	serverHandler.StartScheduler(context.Background())

	// 构建服务器地址
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	srv := &http.Server{
		Addr:    addr,                 // 服务器监听地址
		Handler: serverHandler.Router, // HTTP 请求处理程序
	}

	// 在单独的 goroutine 中启动 HTTP 服务器
	go func() {
		log.Info().Str("addr", addr).Msg("正在启动服务器")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			// 如果服务器启动失败（不是正常关闭），则记录错误并终止程序
			log.Fatal().Err(err).Msg("启动服务器失败")
		}
	}()

	// 创建信号通道，用于接收系统中断信号
	quit := make(chan os.Signal, 1)
	// 监听 SIGINT（Ctrl+C）和 SIGTERM（系统终止）信号
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	// 阻塞等待信号
	<-quit

	log.Info().Msg("正在关闭服务器...")

	// 创建上下文，设置 5 秒超时，用于优雅关闭服务器
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel() // 确保在函数结束时取消上下文

	// 优雅关闭 HTTP 服务器
	if err := srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("服务器强制关闭: %w", err)
	}

	// 如果监控服务正在运行，则停止监控服务
	if monitorService != nil {
		monitorService.Stop()
	}

	// 关闭数据库连接，确保 WAL（预写日志）被检查点处理
	if err := db.Close(); err != nil {
		log.Error().Err(err).Msg("关闭数据库失败")
	}

	log.Info().Msg("服务器已退出")
	return nil
}

// changePassword 修改用户密码
// 参数:
//
//	cmd: Cobra 命令对象
//	args: 命令行参数列表，包含一个用户名参数
//
// 返回值:
//
//	error: 如果发生错误则返回错误信息，否则返回 nil
func changePassword(cmd *cobra.Command, args []string) error {
	// 初始化日志
	logger.Init(config.LoggingConfig{Level: "info"}, config.ServerConfig{}, false)

	// 确保配置文件存在
	configPath, err := config.EnsureConfig(configPath)
	if err != nil {
		return fmt.Errorf("确保配置文件存在失败: %w", err)
	}

	// 加载配置
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("加载配置失败: %w", err)
	}

	// 初始化数据库连接
	db := database.New(cfg.Database)
	if err := db.InitializeTables(context.Background()); err != nil {
		return fmt.Errorf("初始化数据库表失败: %w", err)
	}
	defer db.Close() // 确保在函数结束时关闭数据库连接

	// 获取用户名参数
	username := args[0]

	var password []byte

	// 检查是否在终端环境中运行
	if term.IsTerminal(int(syscall.Stdin)) {
		// 在终端中运行，使用安全的密码输入方式（不显示输入内容）
		fmt.Print("输入新密码: ")
		password, err = term.ReadPassword(int(syscall.Stdin))
		if err != nil {
			return fmt.Errorf("读取密码失败: %w", err)
		}
		fmt.Println() // 换行，使输出更美观
	} else {
		// 不在终端中运行，从标准输入读取密码
		password, err = io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("从标准输入读取密码失败: %w", err)
		}
		password = bytes.TrimSpace(password) // 去除前后空白字符
	}

	// 检查密码是否为空
	if len(password) == 0 {
		return fmt.Errorf("密码不能为空")
	}

	// 验证密码（目前被注释掉）
	//if err := auth.ValidatePassword(string(password)); err != nil {
	//	return fmt.Errorf("密码无效: %w", err)
	//}

	// 更新用户密码
	if err := db.UpdatePassword(context.Background(), username, string(password)); err != nil {
		return fmt.Errorf("更新密码失败: %w", err)
	}

	// 记录成功日志
	log.Info().Str("username", username).Msg("密码更新成功")
	return nil
}

// createUser 创建新用户
// 参数:
//
//	cmd: Cobra 命令对象
//	args: 命令行参数列表，包含一个用户名参数
//
// 返回值:
//
//	error: 如果发生错误则返回错误信息，否则返回 nil
func createUser(cmd *cobra.Command, args []string) error {
	// 初始化日志
	logger.Init(config.LoggingConfig{Level: "info"}, config.ServerConfig{}, false)

	// 确保配置文件存在
	configPath, err := config.EnsureConfig(configPath)
	if err != nil {
		return fmt.Errorf("确保配置文件存在失败: %w", err)
	}

	// 加载配置
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("加载配置失败: %w", err)
	}

	// 初始化数据库连接
	db := database.New(cfg.Database)
	if err := db.InitializeTables(context.Background()); err != nil {
		return fmt.Errorf("初始化数据库表失败: %w", err)
	}
	defer db.Close() // 确保在函数结束时关闭数据库连接

	// 获取用户名参数
	username := args[0]

	var password []byte

	// 检查是否在终端环境中运行
	if term.IsTerminal(int(syscall.Stdin)) {
		// 在终端中运行，使用安全的密码输入方式（不显示输入内容）
		fmt.Print("输入密码: ")
		password, err = term.ReadPassword(int(syscall.Stdin))
		if err != nil {
			return fmt.Errorf("读取密码失败: %w", err)
		}
		fmt.Println() // 换行，使输出更美观
	} else {
		// 不在终端中运行，从标准输入读取密码
		password, err = io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("从标准输入读取密码失败: %w", err)
		}
		password = bytes.TrimSpace(password) // 去除前后空白字符
	}

	// 检查密码是否为空
	if len(password) == 0 {
		return fmt.Errorf("密码不能为空")
	}

	// 验证密码（目前被注释掉）
	//if err := auth.ValidatePassword(string(password)); err != nil {
	//	return fmt.Errorf("密码无效: %w", err)
	//}

	// 创建新用户
	user, err := db.CreateUser(context.Background(), username, string(password))
	if err != nil {
		return fmt.Errorf("创建用户失败: %w", err)
	}

	// 记录成功日志
	log.Info().Str("username", username).Int64("id", user.ID).Msg("用户创建成功")
	return nil
}

// runAgent 启动监控代理
// 参数:
//
//	cmd: Cobra 命令对象
//	args: 命令行参数列表
//
// 返回值:
//
//	error: 如果发生错误则返回错误信息，否则返回 nil
func runAgent(cmd *cobra.Command, args []string) error {
	// 从命令行参数获取配置
	host, _ := cmd.Flags().GetString("host")                      // 主机地址
	port, _ := cmd.Flags().GetInt("port")                         // 端口号
	iface, _ := cmd.Flags().GetString("interface")                // 网络接口
	apiKey, _ := cmd.Flags().GetString("api-key")                 // API密钥
	logLevel, _ := cmd.Flags().GetString("log-level")             // 日志级别
	diskIncludes, _ := cmd.Flags().GetStringSlice("disk-include") // 要包含的磁盘挂载点
	diskExcludes, _ := cmd.Flags().GetStringSlice("disk-exclude") // 要排除的磁盘挂载点

	// 如果提供了配置文件路径，则加载配置
	var cfg *config.Config
	if configPath != "" {
		var err error
		cfg, err = config.Load(configPath)
		if err != nil {
			// 如果配置加载失败，使用默认设置初始化日志
			logger.Init(config.LoggingConfig{Level: "info"}, config.ServerConfig{}, false)
			log.Warn().Err(err).Msg("加载配置失败，使用默认设置")
			cfg = config.New() // 使用默认配置
		}
	} else {
		// 没有提供配置文件，使用默认配置
		cfg = config.New()
	}

	// 如果命令行指定了日志级别，则覆盖配置文件中的设置
	if cmd.Flags().Changed("log-level") && logLevel != "" {
		cfg.Logging.Level = logLevel
	}

	// 使用配置初始化日志
	logger.Init(cfg.Logging, cfg.Server, false)

	// 使用命令行参数覆盖配置文件中的设置（如果有更改）
	if cmd.Flags().Changed("host") {
		cfg.Agent.Host = host
	}
	if cmd.Flags().Changed("port") {
		cfg.Agent.Port = port
	}
	if cmd.Flags().Changed("interface") {
		cfg.Agent.Interface = iface
	}
	if cmd.Flags().Changed("api-key") {
		cfg.Agent.APIKey = apiKey
	}
	if cmd.Flags().Changed("disk-include") {
		cfg.Agent.DiskIncludes = diskIncludes
	}
	if cmd.Flags().Changed("disk-exclude") {
		cfg.Agent.DiskExcludes = diskExcludes
	}

	// 处理 Tailscale 相关的命令行参数
	useTailscale, _ := cmd.Flags().GetBool("tailscale")                  // 是否启用 Tailscale
	tailscaleHostname, _ := cmd.Flags().GetString("tailscale-hostname")  // Tailscale 主机名
	tailscaleAuthKey, _ := cmd.Flags().GetString("tailscale-auth-key")   // Tailscale 认证密钥
	tailscaleStateDir, _ := cmd.Flags().GetString("tailscale-state-dir") // Tailscale 状态目录
	tailscaleMethod, _ := cmd.Flags().GetString("tailscale-method")      // Tailscale 连接方法

	// 创建代理服务
	var agentService *agent.Agent
	if useTailscale || cfg.Tailscale.Agent.Enabled {
		// 如果启用了 Tailscale，则使用 Tailscale 配置
		// 使用命令行参数覆盖 Tailscale 配置（如果有更改）
		if cmd.Flags().Changed("tailscale") {
			cfg.Tailscale.Enabled = useTailscale
			cfg.Tailscale.Agent.Enabled = useTailscale
		}
		if cmd.Flags().Changed("tailscale-hostname") {
			cfg.Tailscale.Hostname = tailscaleHostname
		}
		if cmd.Flags().Changed("tailscale-auth-key") {
			cfg.Tailscale.AuthKey = tailscaleAuthKey
		}
		if cmd.Flags().Changed("tailscale-state-dir") {
			cfg.Tailscale.StateDir = tailscaleStateDir
		}
		if cmd.Flags().Changed("tailscale-method") {
			cfg.Tailscale.Method = tailscaleMethod
		}

		// 创建支持 Tailscale 的代理服务
		agentService = agent.NewWithTailscale(&cfg.Agent, &cfg.Tailscale)
		log.Info().Msg("正在启动支持 Tailscale 的代理")
	} else {
		// 创建普通代理服务
		agentService = agent.New(&cfg.Agent)
	}

	// 设置上下文和取消函数，用于控制代理服务的生命周期
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // 确保在函数结束时取消上下文

	// 处理中断信号
	sigChan := make(chan os.Signal, 1)
	// 监听 SIGINT（Ctrl+C）和 SIGTERM（系统终止）信号
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	// 在单独的 goroutine 中处理信号
	go func() {
		<-sigChan
		log.Info().Msg("收到中断信号")
		cancel() // 取消上下文，通知代理服务停止
	}()

	// 启动代理服务
	return agentService.Start(ctx)
}
