// Copyright (c) 2024-2025, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later
//
// 路由配置模块
// 负责配置和管理所有HTTP路由，包括公共接口和受保护接口
// 使用Gin框架实现路由管理
//
// Go语言特定概念解释：
// - gin.New(): 创建新的Gin路由器实例
// - gin.ReleaseMode: Gin框架的发布模式，提高性能
// - gin.Recovery(): 恢复中间件，处理panic错误
// - router.Group(): 创建路由组，用于统一管理一组路由
// - router.Use(): 添加中间件到路由链
// - c.JSON(): 返回JSON格式响应
// - os.Hostname(): 获取当前主机名
//
// 关键术语定义：
// - 端点（Endpoint）: API的访问地址
// - 公共接口: 无需认证即可访问的API
// - 受保护接口: 需要API密钥认证才能访问的API
// - SSE (Server-Sent Events): 服务器发送事件，一种单向实时通信技术
package agent

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"

	"github.com/autobrr/netronome/internal/version"
)

// setupRoutes 配置agent的所有HTTP路由
// @Description 配置Gin路由器，设置中间件和所有API端点
// @Return *gin.Engine Gin引擎实例
func (a *Agent) setupRoutes() *gin.Engine {
	// 设置Gin路由器
	gin.SetMode(gin.ReleaseMode) // 设置为发布模式，提高性能
	router := gin.New()          // 创建新的路由器实例
	router.Use(gin.Recovery())   // 添加恢复中间件，处理panic错误

	// CORS中间件 - 简化版，确保跨域请求正常工作
	router.Use(corsMiddleware())

	// 根端点（公共，无需认证）
	router.GET("/", a.handleRoot)

	// Agent标识端点（公共，用于发现）
	router.GET("/netronome/info", a.handleInfo)

	// 创建受保护的路由组
	protected := router.Group("/")
	if a.config.APIKey != "" {
		// 如果配置了API密钥，则添加认证中间件
		protected.Use(a.authMiddleware())
	}

	// SSE端点（受保护）
	protected.GET("/events", a.handleSSE)

	// 历史数据导出端点（受保护）
	protected.GET("/export/historical", a.handleHistoricalExport)

	// 系统信息端点（受保护）
	protected.GET("/system/info", a.handleSystemInfo)

	// 峰值统计端点（受保护）
	protected.GET("/stats/peaks", a.handlePeakStats)

	// 硬件统计端点（受保护）
	protected.GET("/system/hardware", a.handleHardwareStats)

	// Tailscale状态端点（受保护）
	protected.GET("/tailscale/status", a.handleTailscaleStatus)

	return router
}

// handleRoot 处理根端点请求
// @Summary 根端点
// @Description 返回服务信息和可用端点列表
// @Tags 公共接口
// @Produce json
// @Success 200 {object} map[string]any 服务信息和端点列表
func (a *Agent) handleRoot(c *gin.Context) {
	response := gin.H{
		"service": "监控SSE代理",
		"host":    a.config.Host,
		"port":    a.config.Port,
		"endpoints": gin.H{
			"live":       "/events?stream=live-data", // 实时数据SSE端点
			"historical": "/export/historical",       // 历史数据导出端点
			"system":     "/system/info",             // 系统信息端点
			"hardware":   "/system/hardware",         // 硬件信息端点
			"peaks":      "/stats/peaks",             // 峰值统计端点
			"tailscale":  "/tailscale/status",        // Tailscale状态端点
		},
	}

	// 指示是否需要认证
	if a.config.APIKey != "" {
		response["authentication"] = "required"
		response["auth_methods"] = []string{"X-API-Key header", "apikey query parameter"}
	} else {
		response["authentication"] = "none"
	}

	c.JSON(http.StatusOK, response)
}

// handleInfo 处理agent标识端点（用于发现）
// @Summary Agent标识端点
// @Description 返回agent的标识信息，用于服务发现
// @Tags 公共接口
// @Produce json
// @Success 200 {object} map[string]any Agent标识信息
func (a *Agent) handleInfo(c *gin.Context) {
	hostname, _ := os.Hostname()

	// 检查是否使用Tailscale
	usingTailscale := false
	if a.tailscaleConfig != nil && a.tailscaleConfig.Enabled {
		usingTailscale = true
	}

	c.JSON(http.StatusOK, gin.H{
		"type":            "netronome-agent", // agent类型
		"version":         version.Version,   // agent版本
		"hostname":        hostname,          // 主机名
		"listening_host":  a.config.Host,     // 监听主机
		"listening_port":  a.config.Port,     // 监听端口
		"using_tailscale": usingTailscale,    // 是否使用Tailscale
	})
}

// handleTailscaleStatus 处理Tailscale状态端点
// @Summary Tailscale状态端点
// @Description 返回Tailscale的状态信息
// @Tags Tailscale
// @Produce json
// @Success 200 {object} map[string]any Tailscale状态信息
// @Failure 500 {object} map[string]string 错误信息
func (a *Agent) handleTailscaleStatus(c *gin.Context) {
	status, err := a.GetTailscaleStatus()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, status)
}
