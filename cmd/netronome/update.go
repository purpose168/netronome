// Copyright (c) 2024-2025, s0up 和 autobrr 贡献者.
// SPDX-License-Identifier: GPL-2.0-or-later
//
// Netronome 更新模块
// 包名: main
// 功能: 提供程序自动更新功能，从 GitHub 下载最新版本
// 作者: s0up 和 autobrr 贡献者
// 创建日期: 2024
// 许可证: GPL-2.0-or-later

package main

import (
	// Go 标准库包
	"fmt"     // 格式化输入输出
	"runtime" // 运行时相关功能，如操作系统检测
	"strings" // 字符串操作函数

	// 第三方库
	"github.com/creativeprojects/go-selfupdate" // 自更新库，用于从 GitHub 下载更新
	"github.com/rs/zerolog/log"                 // 结构化日志库
	"github.com/spf13/cobra"                    // 命令行界面框架
)

// updateCmd 更新命令，用于检查并安装 Netronome 的最新版本
var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "将 Netronome 更新到最新版本",
	Long: `检查并安装 Netronome 的最新版本。
此命令将从 GitHub 下载最新版本并替换当前二进制文件。`,
	RunE: runUpdate,
}

// runUpdate 执行更新操作，检查并安装最新版本
// 参数:
//
//	cmd: Cobra 命令对象
//	args: 命令行参数列表
//
// 返回值:
//
//	error: 如果发生错误则返回错误信息，否则返回 nil
func runUpdate(cmd *cobra.Command, args []string) error {
	log.Info().Str("current_version", version).Msg("正在检查更新...")

	// 创建 GitHub 仓库源
	repo := selfupdate.ParseSlug("autobrr/netronome")

	// 首先，创建更新器实例（不带验证）以获取最新版本信息
	updater, err := selfupdate.NewUpdater(selfupdate.Config{})
	if err != nil {
		return fmt.Errorf("创建更新器失败: %w", err)
	}

	// 获取最新版本信息
	release, found, err := updater.DetectLatest(cmd.Context(), repo)
	if err != nil {
		return fmt.Errorf("检测最新版本失败: %w", err)
	}

	if !found {
		log.Info().Msg("未找到更新")
		return nil
	}

	// 比较版本
	if release.LessOrEqual(version) {
		log.Info().
			Str("current_version", version).
			Str("latest_version", release.Version()).
			Msg("已经在运行最新版本")
		return nil
	}

	log.Info().
		Str("current_version", version).
		Str("latest_version", release.Version()).
		Msg("发现新版本")

	// 现在创建带有正确校验和文件名的更新器
	versionStr := strings.TrimPrefix(release.Version(), "v") // 移除版本号前缀 "v"

	updaterWithChecksum, err := selfupdate.NewUpdater(selfupdate.Config{
		// 设置校验和验证器，确保下载的文件完整性
		Validator: &selfupdate.ChecksumValidator{
			UniqueFilename: fmt.Sprintf("netronome_%s_checksums.txt", versionStr),
		},
	})
	if err != nil {
		return fmt.Errorf("创建带校验和的更新器失败: %w", err)
	}

	// 执行更新操作
	exe, err := selfupdate.ExecutablePath()
	if err != nil {
		return fmt.Errorf("获取可执行文件路径失败: %w", err)
	}

	log.Info().Str("path", exe).Msg("正在更新二进制文件...")

	// 更新当前可执行文件
	if err := updaterWithChecksum.UpdateTo(cmd.Context(), release, exe); err != nil {
		return fmt.Errorf("更新失败: %w", err)
	}

	log.Info().
		Str("version", release.Version()).
		Msg("成功更新到最新版本")

	// 告知用户可能需要重启服务
	if runtime.GOOS == "linux" {
		log.Info().Msg("如果以 systemd 服务运行，请使用以下命令重启: systemctl restart netronome-agent")
	}

	return nil
}
