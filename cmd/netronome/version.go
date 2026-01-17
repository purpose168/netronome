// Copyright (c) 2024-2025, s0up 和 autobrr 贡献者.
// SPDX-License-Identifier: GPL-2.0-or-later
//
// Netronome 版本模块
// 包名: main
// 功能: 提供版本信息查询功能，显示程序版本、构建信息和运行环境
// 作者: s0up 和 autobrr 贡献者
// 创建日期: 2024
// 许可证: GPL-2.0-or-later

package main

import (
	// Go 标准库包
	"fmt"           // 格式化输入输出
	"runtime"       // 运行时相关功能，如 Go 版本、操作系统、架构信息
	"runtime/debug" // 调试相关功能，如读取构建信息

	// 第三方库
	"github.com/spf13/cobra" // 命令行界面框架

	// 内部包
	appversion "github.com/autobrr/netronome/internal/version" // 版本信息管理
)

// versionCmd 版本命令，用于显示程序版本和构建信息
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "打印版本信息",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("netronome 版本: %s\n", appversion.Version)
		if appversion.Commit != "unknown" {
			fmt.Printf("Git 提交:        %s\n", appversion.Commit)
		}
		if appversion.BuildTime != "unknown" {
			fmt.Printf("构建时间:        %s\n", appversion.BuildTime)
		}
		fmt.Printf("Go 版本:        %s\n", runtime.Version())
		fmt.Printf("操作系统/架构:   %s/%s\n", runtime.GOOS, runtime.GOARCH)
	},
	DisableFlagsInUseLine: true, // 禁用命令行中显示已使用的标志
}

// SetVersion 设置版本信息
// 参数:
//
//	v: 版本号
//	bt: 构建时间
//	c: Git 提交哈希值
func SetVersion(v, bt, c string) {
	// 如果版本为 "dev"（开发版本），则尝试从构建信息中获取实际版本
	if v == "dev" {
		if info, ok := debug.ReadBuildInfo(); ok {
			v = info.Main.Version
		}
	}
	// 将版本信息设置到内部版本管理模块
	appversion.Set(v, bt, c)
}

// init 函数在程序启动时自动执行，用于初始化版本命令的使用模板
func init() {
	versionCmd.SetUsageTemplate(`用法:
  {{.CommandPath}}

打印 netronome 的版本和构建时间信息。
`)
}
