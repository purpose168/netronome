// Copyright (c) 2024-2025, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later
//
// 中间件模块
// 提供HTTP请求处理的中间件，包括CORS处理和API密钥验证
//
// Go语言特定概念解释：
// - gin.HandlerFunc: Gin框架的HTTP处理器函数类型
// - gin.Context: Gin框架的HTTP请求上下文对象
// - c.Writer.Header().Set: 设置HTTP响应头
// - c.Request.Method: 获取HTTP请求方法
// - c.AbortWithStatus: 终止请求处理并返回指定状态码
// - c.Next(): 继续执行下一个中间件或路由处理器
// - c.GetHeader: 获取HTTP请求头
// - c.Query: 获取HTTP查询参数
// - c.JSON: 返回JSON格式响应
// - c.Abort(): 终止请求处理链
package agent

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// corsMiddleware 处理跨域资源共享(CORS)头部
// @Summary CORS中间件
// @Description 配置跨域资源共享策略，允许所有来源的请求
// @Tags 中间件
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 设置允许所有来源的请求
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		// 允许所有HTTP方法
		c.Writer.Header().Set("Access-Control-Allow-Methods", "*")
		// 允许所有HTTP头部
		c.Writer.Header().Set("Access-Control-Allow-Headers", "*")

		// 处理OPTIONS预检请求，直接返回204 No Content
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		// 继续处理请求
		c.Next()
	}
}

// authMiddleware 验证API密钥（从请求头或查询参数）
// @Summary API密钥验证中间件
// @Description 验证请求中的API密钥，支持从请求头或查询参数获取
// @Tags 中间件
// @Param X-API-Key header string false "API密钥"
// @Param apikey query string false "API密钥（查询参数）"
// @Failure 401 {object} map[string]string 无效或缺失API密钥
func (a *Agent) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 首先检查请求头中的API密钥
		apiKey := c.GetHeader("X-API-Key")
		// 如果请求头中没有，检查查询参数中的API密钥
		if apiKey == "" {
			apiKey = c.Query("apikey")
		}
		// 验证API密钥是否有效
		if apiKey != a.config.APIKey {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "无效或缺失API密钥"})
			// 终止请求处理
			c.Abort()
			return
		}
		// 继续处理请求
		c.Next()
	}
}
