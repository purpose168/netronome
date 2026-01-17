// Copyright (c) 2024-2025, s0up 和 autobrr 贡献者.
// SPDX-License-Identifier: GPL-2.0-or-later
//
// Netronome 监控代理类型定义
// 包名: agent
// 功能: 定义监控代理的核心数据结构和类型
// 作者: s0up 和 autobrr 贡献者
// 创建日期: 2024
// 许可证: GPL-2.0-or-later

package agent

import (
	// Go 标准库包
	"sync" // 同步原语，用于并发控制
	"time" // 时间和定时器功能

	// 第三方库
	"tailscale.com/tsnet" // Tailscale 网络集成库

	// 内部包
	"github.com/autobrr/netronome/internal/config" // 配置管理
)

// Agent 代表监控 SSE（服务器发送事件）代理
// 这是监控代理的核心数据结构，包含了配置、客户端管理、监控数据和性能统计等
// SSE（Server-Sent Events）是一种 HTTP 服务器向客户端推送实时数据的技术
// 客户端可以通过建立一个长连接来接收服务器发送的事件流
// 这种技术非常适合实时监控场景，因为它允许服务器主动向客户端发送数据更新
// 而不需要客户端定期轮询服务器
//
// Agent 结构体包含以下主要功能模块：
// 1. 配置管理：存储代理和 Tailscale 的配置信息
// 2. 客户端管理：跟踪连接的客户端并向它们广播监控数据
// 3. 数据采集：通过监控数据通道接收性能数据
// 4. 性能统计：记录和跟踪网络性能的峰值
// 5. Tailscale 集成：支持通过 Tailscale 网络进行安全连接
//
// 并发安全：使用读写互斥锁（sync.RWMutex）保护并发访问的数据结构
// 例如，clients 和 peak 相关字段都有对应的互斥锁来确保线程安全
//
// 通信机制：使用 Go 通道（channel）来安全地在不同 goroutine 之间传递数据
// monitorData 通道用于传递监控数据，clients 映射用于跟踪客户端连接
//
// Tailscale 支持：通过 tsnetServer 和 useTailscale 字段实现对 Tailscale 网络的集成
// 这允许代理在 Tailscale 网络中安全通信，不需要开放公共端口
//
// 峰值统计：peakRx 和 peakTx 字段记录网络下载和上传的峰值速度
// 这些峰值会定期更新，并包含对应的时间戳信息
//
// 总体而言，Agent 结构体是一个设计良好的并发安全数据结构，
// 它封装了监控代理的所有核心功能，提供了高效的实时数据采集和分发机制
// 非常适合构建网络性能监控系统
//
// 如何使用：
// 1. 使用 New 或 NewWithTailscale 函数创建 Agent 实例
// 2. 调用 Start 方法启动代理服务器
// 3. 客户端可以通过 HTTP 连接接收实时监控数据
//
// 代码示例：
//
//	cfg := &config.AgentConfig{Host: "0.0.0.0", Port: 8200}
//	agent := agent.New(cfg)
//	ctx, cancel := context.WithCancel(context.Background())
//	defer cancel()
//	if err := agent.Start(ctx); err != nil {
//	    log.Fatal().Err(err).Msg("Failed to start agent")
//	}
//	log.Info().Msg("Agent started successfully")
//	<-ctx.Done()
type Agent struct {
	config          *config.AgentConfig     // 代理配置
	tailscaleConfig *config.TailscaleConfig // Tailscale 配置
	clients         map[chan string]bool    // 连接的客户端映射，键为客户端通道，值为是否活跃
	clientsMu       sync.RWMutex            // 客户端映射的读写互斥锁，确保并发安全
	monitorData     chan string             // 监控数据通道，用于传递性能数据
	peakRx          int                     // 峰值下载速度（字节/秒）
	peakTx          int                     // 峰值上传速度（字节/秒）
	peakRxTimestamp time.Time               // 峰值下载速度记录时间戳
	peakTxTimestamp time.Time               // 峰值上传速度记录时间戳
	peakMu          sync.RWMutex            // 峰值统计的读写互斥锁，确保并发安全
	tsnetServer     *tsnet.Server           // Tailscale 网络服务器
	useTailscale    bool                    // 是否使用 Tailscale
}

// MonitorLiveData 表示来自 vnstat --live --json 的 JSON 结构
// vnstat 是一个网络流量监控工具，--live --json 参数可以获取实时流量数据
// 这个结构体用于解析和存储 vnstat 输出的实时监控数据
// 主要包含下载（Rx）和上传（Tx）两个方向的流量统计
// 数据以 JSON 格式传输和存储
//
// 字段说明：
//
//	Index: 监控索引
//	Seconds: 监控持续时间（秒）
//	Rx: 接收（下载）流量统计
//	Tx: 发送（上传）流量统计
//
// Rx 和 Tx 都包含以下字段：
//
//	Ratestring: 人类可读的速率字符串（如 "1.23 Mbit/s"）
//	Bytespersecond: 每秒字节数
//	Packetspersecond: 每秒数据包数
//	Bytes: 本次监控的总字节数
//	Packets: 本次监控的总数据包数
//	Totalbytes: 累计总字节数
//	Totalpackets: 累计总数据包数
//
// 这个结构体是实现实时网络监控的核心数据结构之一
// 它提供了详细的网络流量统计信息，包括速率、字节数和数据包数等
// 非常适合用于构建网络性能监控和分析系统
type MonitorLiveData struct {
	Index   int `json:"index"`
	Seconds int `json:"seconds"`
	Rx      struct {
		Ratestring       string `json:"ratestring"`
		Bytespersecond   int    `json:"bytespersecond"`
		Packetspersecond int    `json:"packetspersecond"`
		Bytes            int    `json:"bytes"`
		Packets          int    `json:"packets"`
		Totalbytes       int    `json:"totalbytes"`
		Totalpackets     int    `json:"totalpackets"`
	} `json:"rx"`
	Tx struct {
		Ratestring       string `json:"ratestring"`
		Bytespersecond   int    `json:"bytespersecond"`
		Packetspersecond int    `json:"packetspersecond"`
		Bytes            int    `json:"bytes"`
		Packets          int    `json:"packets"`
		Totalbytes       int    `json:"totalbytes"`
		Totalpackets     int    `json:"totalpackets"`
	} `json:"tx"`
}

// SystemInfo 表示系统信息
// 这个结构体用于存储和传输系统级别的基本信息
// 主要包含主机名、内核版本、系统运行时间、网络接口信息等
// 数据以 JSON 格式传输和存储，便于前端界面展示和数据处理
//
// 字段说明：
//
//	Hostname: 主机名称
//	Kernel: 操作系统内核版本
//	Uptime: 系统运行时间（秒）
//	Interfaces: 网络接口信息映射，键为接口名称，值为接口详细信息
//	VnstatVersion: vnstat 工具的版本信息
//	UpdatedAt: 数据更新时间戳
//
// 这个结构体是系统监控的基础数据结构之一
// 它提供了系统的整体概览信息，有助于监控系统状态和诊断问题
// 非常适合用于构建系统监控面板和状态仪表盘
//
// 使用场景：
// 1. 系统状态监控面板显示
// 2. 系统信息报告生成
// 3. 系统配置和版本管理
// 4. 网络接口配置和状态监控
//
// 数据更新机制：
// 通常由监控代理定期采集并更新这些信息
// 更新频率可以根据实际需求进行调整
// 更新后的数据会通过 SSE 或其他实时通信方式推送给客户端
//
// 接口关系：
// SystemInfo 结构体包含了 InterfaceInfo 结构体的映射
// 每个 InterfaceInfo 代表一个网络接口的详细信息
// 这种嵌套结构提供了层次化的系统信息视图
//
// JSON 序列化：
// 所有字段都使用了 JSON 标签，确保数据可以正确地序列化为 JSON 格式
// 这使得数据可以方便地在不同组件之间传输和共享
type SystemInfo struct {
	Hostname      string                   `json:"hostname"`
	Kernel        string                   `json:"kernel"`
	Uptime        int64                    `json:"uptime"` // 秒
	Interfaces    map[string]InterfaceInfo `json:"interfaces"`
	VnstatVersion string                   `json:"vnstat_version"`
	UpdatedAt     time.Time                `json:"updated_at"`
}

// InterfaceInfo 表示网络接口详细信息
// 这个结构体用于存储和传输单个网络接口的详细信息
// 主要包含接口名称、别名、IP地址、链路速度、状态等
// 数据以 JSON 格式传输和存储，便于前端界面展示和数据处理
//
// 字段说明：
//
//	Name: 网络接口名称（如 eth0、wlan0）
//	Alias: 网络接口别名（可选的友好名称）
//	IPAddress: 分配给该接口的 IP 地址
//	LinkSpeed: 接口链路速度（Mbps）
//	IsUp: 接口是否处于激活状态
//	BytesTotal: 接口累计传输的总字节数
//
// 这个结构体是网络监控的核心数据结构之一
// 它提供了单个网络接口的详细配置和状态信息
// 非常适合用于构建网络接口监控面板和流量分析系统
//
// 使用场景：
// 1. 网络接口状态监控
// 2. 网络流量分析和报告
// 3. 网络设备配置管理
// 4. 网络性能优化和故障排查
//
// 数据更新机制：
// 通常由监控代理定期采集并更新这些信息
// 更新频率可以根据实际需求进行调整
// 更新后的数据会通过 SSE 或其他实时通信方式推送给客户端
//
// 与其他结构体的关系：
// InterfaceInfo 结构体被包含在 SystemInfo 结构体中
// 以映射的形式存储所有网络接口的信息
// 这种嵌套结构提供了完整的系统网络信息视图
//
// JSON 序列化：
// 所有字段都使用了 JSON 标签，确保数据可以正确地序列化为 JSON 格式
// 这使得数据可以方便地在不同组件之间传输和共享
type InterfaceInfo struct {
	Name       string `json:"name"`
	Alias      string `json:"alias"`
	IPAddress  string `json:"ip_address"`
	LinkSpeed  int    `json:"link_speed"` // Mbps
	IsUp       bool   `json:"is_up"`
	BytesTotal int64  `json:"bytes_total"`
}

// PeakStats 表示峰值带宽统计信息
// 这个结构体用于存储和传输网络带宽的峰值统计数据
// 主要包含下载和上传方向的峰值速度、峰值出现时间等
// 数据以 JSON 格式传输和存储，便于前端界面展示和数据处理
//
// 字段说明：
//
//	PeakRx: 峰值下载速度（字节/秒）
//	PeakTx: 峰值上传速度（字节/秒）
//	PeakRxString: 人类可读的峰值下载速度字符串（如 "1.23 Mbit/s"）
//	PeakTxString: 人类可读的峰值上传速度字符串（如 "5.67 Mbit/s"）
//	PeakRxTimestamp: 峰值下载速度出现的时间戳
//	PeakTxTimestamp: 峰值上传速度出现的时间戳
//	UpdatedAt: 数据更新时间戳
//
// 这个结构体是网络性能监控的核心数据结构之一
// 它提供了网络带宽使用的峰值情况，有助于了解网络性能的极限
// 非常适合用于构建网络性能监控面板和流量分析系统
//
// 使用场景：
// 1. 网络性能极限分析
// 2. 网络容量规划和优化
// 3. 网络资源使用峰值监控
// 4. 网络故障排查和性能调优
//
// 数据更新机制：
// 通常由监控代理实时采集网络流量数据，并在发现新峰值时更新
// 更新后的数据会通过 SSE 或其他实时通信方式推送给客户端
// 峰值数据会持久化存储，以便长期分析和趋势预测
//
// 与其他结构体的关系：
// PeakStats 结构体可以独立使用，也可以与其他监控数据结合使用
// 例如，可以与 MonitorLiveData 结合展示实时流量和历史峰值对比
//
// JSON 序列化：
// 所有字段都使用了 JSON 标签，确保数据可以正确地序列化为 JSON 格式
// 这使得数据可以方便地在不同组件之间传输和共享
//
// 峰值计算逻辑：
// 峰值速度是通过实时监控网络流量数据计算得出的
// 当当前速度超过历史峰值时，会更新峰值记录并记录时间戳
// 峰值数据通常会定期重置（如每日、每周），以便进行周期性能分析
type PeakStats struct {
	PeakRx          int       `json:"peak_rx"` // 字节/秒
	PeakTx          int       `json:"peak_tx"` // 字节/秒
	PeakRxString    string    `json:"peak_rx_string"`
	PeakTxString    string    `json:"peak_tx_string"`
	PeakRxTimestamp time.Time `json:"peak_rx_timestamp"`
	PeakTxTimestamp time.Time `json:"peak_tx_timestamp"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// HardwareStats 表示系统硬件统计信息
// 这个结构体用于存储和传输系统硬件组件的统计数据
// 主要包含CPU、内存、磁盘和温度传感器等硬件组件的信息
// 数据以 JSON 格式传输和存储，便于前端界面展示和数据处理
//
// 字段说明：
//
//	CPU: CPU 统计信息
//	Memory: 内存统计信息
//	Disks: 磁盘统计信息列表
//	Temperature: 温度传感器数据列表（可选）
//	UpdatedAt: 数据更新时间戳
//
// 这个结构体是系统硬件监控的核心数据结构之一
// 它提供了系统硬件组件的详细状态和性能数据
// 非常适合用于构建系统硬件监控面板和性能分析系统
//
// 使用场景：
// 1. 系统硬件状态监控
// 2. 系统性能分析和优化
// 3. 硬件故障排查和预警
// 4. 系统资源使用趋势分析
// 5. 硬件升级和容量规划
//
// 数据更新机制：
// 通常由监控代理定期采集硬件统计数据
// 更新频率可以根据实际需求进行调整
// 更新后的数据会通过 SSE 或其他实时通信方式推送给客户端
// 硬件数据通常会持久化存储，以便长期分析和趋势预测
//
// 与其他结构体的关系：
// HardwareStats 结构体包含了多个子结构体：
// - CPUStats: CPU 统计信息
// - MemoryStats: 内存统计信息
// - DiskStats: 磁盘统计信息列表
// - TemperatureStats: 温度传感器数据列表
// 这些子结构体共同构成了完整的硬件监控数据模型
//
// JSON 序列化：
// 所有字段都使用了 JSON 标签，确保数据可以正确地序列化为 JSON 格式
// Temperature 字段使用了 omitempty 标签，表示如果该字段为空则不会包含在 JSON 输出中
// 这使得数据可以方便地在不同组件之间传输和共享
//
// 监控数据采集：
// 硬件统计数据通常通过操作系统提供的接口或第三方工具采集
// 例如，CPU 使用率可以通过 /proc/stat 文件获取
// 内存使用情况可以通过 /proc/meminfo 文件获取
// 磁盘使用情况可以通过 statfs 系统调用获取
// 温度传感器数据可以通过 lm-sensors 等工具获取
type HardwareStats struct {
	CPU         CPUStats           `json:"cpu"`
	Memory      MemoryStats        `json:"memory"`
	Disks       []DiskStats        `json:"disks"`
	Temperature []TemperatureStats `json:"temperature,omitempty"`
	UpdatedAt   time.Time          `json:"updated_at"`
}

// CPUStats 表示CPU使用统计信息
// 这个结构体用于存储和传输CPU的使用情况和性能数据
// 主要包含CPU使用率、核心数、线程数、型号、频率和负载平均值等
// 数据以 JSON 格式传输和存储，便于前端界面展示和数据处理
//
// 字段说明：
//
//	UsagePercent: CPU使用率百分比
//	Cores: CPU核心数
//	Threads: CPU线程数
//	Model: CPU型号名称
//	Frequency: CPU频率（MHz）
//	LoadAvg: CPU负载平均值数组（可选），包含1分钟、5分钟和15分钟的负载平均值
//
// 这个结构体是CPU监控的核心数据结构之一
// 它提供了CPU的详细状态和性能数据，有助于了解系统的CPU使用情况
// 非常适合用于构建系统性能监控面板和CPU性能分析系统
//
// 使用场景：
// 1. CPU使用率监控和预警
// 2. 系统性能瓶颈分析
// 3. 应用程序CPU消耗分析
// 4. 系统资源规划和优化
// 5. 硬件性能评估和比较
//
// 数据更新机制：
// 通常由监控代理定期采集CPU统计数据
// 更新频率可以根据实际需求进行调整（如每秒或每5秒更新一次）
// 更新后的数据会通过 SSE 或其他实时通信方式推送给客户端
//
// 与其他结构体的关系：
// CPUStats 结构体是 HardwareStats 结构体的一个子结构体
// 它提供了系统硬件监控中CPU组件的详细信息
//
// JSON 序列化：
// 所有字段都使用了 JSON 标签，确保数据可以正确地序列化为 JSON 格式
// LoadAvg 字段使用了 omitempty 标签，表示如果该字段为空则不会包含在 JSON 输出中
// 这使得数据可以方便地在不同组件之间传输和共享
//
// CPU数据采集：
// CPU统计数据通常通过操作系统提供的接口或工具采集
// 例如，在Linux系统中：
// - CPU使用率可以通过 /proc/stat 文件计算得出
// - CPU型号和核心数可以通过 /proc/cpuinfo 文件获取
// - CPU频率可以通过 /sys/devices/system/cpu/cpu*/cpufreq 文件获取
// - 负载平均值可以通过 /proc/loadavg 文件获取
//
// CPU负载平均值说明：
// LoadAvg 数组包含三个值，分别代表1分钟、5分钟和15分钟的平均负载
// 负载平均值表示在特定时间段内，等待CPU处理的进程数量
// 对于多核系统，理想的负载平均值应该等于或略低于CPU核心数
type CPUStats struct {
	UsagePercent float64   `json:"usage_percent"`
	Cores        int       `json:"cores"`
	Threads      int       `json:"threads"`
	Model        string    `json:"model"`
	Frequency    float64   `json:"frequency"`          // MHz
	LoadAvg      []float64 `json:"load_avg,omitempty"` // 1分钟、5分钟、15分钟负载平均值
}

// MemoryStats 表示内存使用统计信息
// 这个结构体用于存储和传输系统内存的使用情况和统计数据
// 主要包含物理内存、缓存、缓冲区、ZFS ARC缓存和交换空间等信息
// 数据以 JSON 格式传输和存储，便于前端界面展示和数据处理
//
// 字段说明：
//
//	Total: 物理内存总量（字节）
//	Used: 已使用的物理内存（字节）
//	Free: 空闲的物理内存（字节）
//	Available: 可用于分配的物理内存（字节）
//	UsedPercent: 物理内存使用率百分比
//	Cached: 缓存的内存（字节）
//	Buffers: 缓冲区内存（字节）
//	ZFSArc: ZFS ARC缓存大小（字节）
//	SwapTotal: 交换空间总量（字节）
//	SwapUsed: 已使用的交换空间（字节）
//	SwapPercent: 交换空间使用率百分比
//
// 这个结构体是内存监控的核心数据结构之一
// 它提供了系统内存的详细状态和使用情况，有助于了解系统的内存资源分配
// 非常适合用于构建系统性能监控面板和内存使用分析系统
//
// 使用场景：
// 1. 内存使用率监控和预警
// 2. 系统内存泄漏检测
// 3. 应用程序内存消耗分析
// 4. 系统内存资源规划和优化
// 5. 虚拟内存和交换空间使用监控
//
// 数据更新机制：
// 通常由监控代理定期采集内存统计数据
// 更新频率可以根据实际需求进行调整（如每秒或每5秒更新一次）
// 更新后的数据会通过 SSE 或其他实时通信方式推送给客户端
//
// 与其他结构体的关系：
// MemoryStats 结构体是 HardwareStats 结构体的一个子结构体
// 它提供了系统硬件监控中内存组件的详细信息
//
// JSON 序列化：
// 所有字段都使用了 JSON 标签，确保数据可以正确地序列化为 JSON 格式
// 这使得数据可以方便地在不同组件之间传输和共享
//
// 内存数据采集：
// 内存统计数据通常通过操作系统提供的接口或工具采集
// 例如，在Linux系统中：
// - 物理内存信息可以通过 /proc/meminfo 文件获取
// - ZFS ARC缓存信息可以通过 /proc/spl/kstat/zfs/arcstats 文件获取
// - 交换空间信息可以通过 /proc/swaps 文件获取
//
// 内存概念说明：
// - 可用内存（Available）：包括空闲内存、缓存和缓冲区中可回收的部分
// - 缓存（Cached）：用于缓存文件系统数据的内存
// - 缓冲区（Buffers）：用于缓存磁盘I/O操作的内存
// - ZFS ARC：ZFS文件系统的自适应替换缓存，用于加速文件系统访问
// - 交换空间（Swap）：当物理内存不足时，用于临时存储内存数据的磁盘空间
//
// 内存监控的重要性：
// 内存是系统性能的关键资源之一，内存不足会导致系统性能下降甚至崩溃
// 通过监控内存使用情况，可以及时发现内存泄漏、内存过度使用等问题
// 有助于提前采取措施，避免系统性能问题和故障
type MemoryStats struct {
	Total       uint64  `json:"total"`
	Used        uint64  `json:"used"`
	Free        uint64  `json:"free"`
	Available   uint64  `json:"available"`
	UsedPercent float64 `json:"used_percent"`
	Cached      uint64  `json:"cached"`
	Buffers     uint64  `json:"buffers"`
	ZFSArc      uint64  `json:"zfs_arc"`
	SwapTotal   uint64  `json:"swap_total"`
	SwapUsed    uint64  `json:"swap_used"`
	SwapPercent float64 `json:"swap_percent"`
}

// DiskStats 表示磁盘使用统计信息
// 这个结构体用于存储和传输单个磁盘或分区的使用情况和统计数据
// 主要包含磁盘路径、设备名称、文件系统类型、容量、使用率等信息
// 数据以 JSON 格式传输和存储，便于前端界面展示和数据处理
//
// 字段说明：
//
//	Path: 磁盘挂载路径（如 /, /home）
//	Device: 磁盘设备名称（如 /dev/sda1, /dev/nvme0n1p1）
//	Fstype: 文件系统类型（如 ext4, xfs, btrfs）
//	Total: 磁盘总容量（字节）
//	Used: 已使用的磁盘容量（字节）
//	Free: 可用的磁盘容量（字节）
//	UsedPercent: 磁盘使用率百分比
//	Model: 磁盘型号名称（可选，来自 SMART 数据）
//	Serial: 磁盘序列号（可选，来自 SMART 数据）
//
// 这个结构体是磁盘监控的核心数据结构之一
// 它提供了单个磁盘或分区的详细状态和使用情况，有助于了解系统的存储资源分配
// 非常适合用于构建系统存储监控面板和磁盘使用分析系统
//
// 使用场景：
// 1. 磁盘使用率监控和预警
// 2. 存储容量规划和管理
// 3. 文件系统类型和配置监控
// 4. 磁盘硬件信息管理
// 5. 存储资源使用趋势分析
//
// 数据更新机制：
// 通常由监控代理定期采集磁盘统计数据
// 更新频率可以根据实际需求进行调整（如每分钟或每5分钟更新一次）
// 更新后的数据会通过 SSE 或其他实时通信方式推送给客户端
//
// 与其他结构体的关系：
// DiskStats 结构体是 HardwareStats 结构体中的一个子结构体列表
// 它提供了系统硬件监控中存储组件的详细信息
//
// JSON 序列化：
// 所有字段都使用了 JSON 标签，确保数据可以正确地序列化为 JSON 格式
// Model 和 Serial 字段使用了 omitempty 标签，表示如果该字段为空则不会包含在 JSON 输出中
// 这使得数据可以方便地在不同组件之间传输和共享
//
// 磁盘数据采集：
// 磁盘统计数据通常通过操作系统提供的接口或工具采集
// 例如，在Linux系统中：
// - 磁盘使用情况可以通过 statfs 系统调用获取
// - 磁盘挂载信息可以通过 /proc/mounts 文件获取
// - 磁盘硬件信息（型号、序列号）可以通过 SMART 工具（如 smartctl）获取
//
// 磁盘监控的重要性：
// 磁盘空间不足会导致系统无法正常工作，应用程序崩溃
// 通过监控磁盘使用情况，可以及时发现磁盘空间不足的问题
// 有助于提前采取措施，避免系统故障和数据丢失
// 对于使用 ZFS 等高级文件系统的系统，磁盘健康监控尤为重要
type DiskStats struct {
	Path        string  `json:"path"`
	Device      string  `json:"device"`
	Fstype      string  `json:"fstype"`
	Total       uint64  `json:"total"`
	Used        uint64  `json:"used"`
	Free        uint64  `json:"free"`
	UsedPercent float64 `json:"used_percent"`
	Model       string  `json:"model,omitempty"`  // 来自 SMART 数据的磁盘型号名称
	Serial      string  `json:"serial,omitempty"` // 来自 SMART 数据的磁盘序列号
}

// TemperatureStats 表示温度传感器数据
// 这个结构体用于存储和传输单个温度传感器的测量数据
// 主要包含传感器标识、温度值、标签和临界温度等信息
// 数据以 JSON 格式传输和存储，便于前端界面展示和数据处理
//
// 字段说明：
//
//	SensorKey: 传感器唯一标识（如 cpu_thermal_zone0, nvme_temp1）
//	Temperature: 温度值（摄氏度）
//	Label: 传感器标签（可选的友好名称，如 "CPU 温度", "NVMe 温度"）
//	Critical: 临界温度值（可选，超过此温度可能导致硬件损坏）
//
// 这个结构体是温度监控的核心数据结构之一
// 它提供了单个温度传感器的详细测量数据，有助于了解系统硬件的温度状态
// 非常适合用于构建系统温度监控面板和硬件健康分析系统
//
// 使用场景：
// 1. CPU、GPU、内存等硬件温度监控
// 2. 硬件温度预警和保护
// 3. 系统散热性能分析和优化
// 4. 硬件故障预防和诊断
// 5. 环境温度监控（如服务器机房温度）
//
// 数据更新机制：
// 通常由监控代理定期采集温度传感器数据
// 更新频率可以根据实际需求进行调整（如每秒或每5秒更新一次）
// 更新后的数据会通过 SSE 或其他实时通信方式推送给客户端
//
// 与其他结构体的关系：
// TemperatureStats 结构体是 HardwareStats 结构体中的一个子结构体列表
// 它提供了系统硬件监控中温度传感器的详细信息
//
// JSON 序列化：
// 所有字段都使用了 JSON 标签，确保数据可以正确地序列化为 JSON 格式
// Label 和 Critical 字段使用了 omitempty 标签，表示如果该字段为空则不会包含在 JSON 输出中
// 这使得数据可以方便地在不同组件之间传输和共享
//
// 温度数据采集：
// 温度传感器数据通常通过硬件监控工具或操作系统接口采集
// 例如，在Linux系统中：
// - CPU温度可以通过 /sys/class/thermal/ 目录下的文件获取
// - GPU温度可以通过 NVIDIA 或 AMD 提供的驱动接口获取
// - 磁盘温度可以通过 SMART 数据获取
// - 系统温度传感器数据可以通过 lm-sensors 工具获取
//
// 温度监控的重要性：
// 硬件温度过高会导致系统性能下降、硬件损坏甚至火灾风险
// 通过监控硬件温度，可以及时发现散热问题并采取措施（如增加风扇转速、清理灰尘）
// 有助于延长硬件寿命，确保系统稳定运行
// 对于高性能计算系统、服务器和嵌入式设备尤为重要
type TemperatureStats struct {
	SensorKey   string  `json:"sensor_key"`
	Temperature float64 `json:"temperature"` // 摄氏度
	Label       string  `json:"label,omitempty"`
	Critical    float64 `json:"critical,omitempty"`
}
