// Copyright (c) 2024-2025, s0up 和 autobrr 贡献者.
// SPDX-License-Identifier: GPL-2.0-or-later
//
// Netronome 磁盘工具模块
// 包名: agent
// 功能: 实现磁盘监控相关的工具函数，包括路径匹配、磁盘过滤和设备路径获取
// 作者: s0up 和 autobrr 贡献者
// 创建日期: 2024
// 许可证: GPL-2.0-or-later
//
// 主要功能:
// 1. 路径匹配：支持通配符和前缀匹配的路径比较功能
// 2. 磁盘过滤：根据配置规则和系统默认规则决定是否监控特定磁盘
// 3. 设备路径获取：跨平台获取磁盘设备路径，用于 SMART 数据采集
//
// 技术特点:
// - 支持跨平台（Linux 和 macOS）
// - 支持通配符模式匹配（使用 filepath.Match）
// - 支持前缀匹配模式（使用 * 通配符）
// - 内置默认规则过滤特殊文件系统（如 /proc、/sys 等）
// - 自动检测操作系统类型并使用相应的设备命名规则
// - 使用 zerolog 进行结构化日志记录

package agent

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/rs/zerolog/log"
)

// matchPath 检查路径是否与模式匹配，支持通配符和前缀匹配
// 功能：判断给定的路径是否符合指定的匹配模式，支持多种匹配规则
// 参数：
//
//	pattern string: 匹配模式，可以包含通配符
//	path string: 要检查的路径
//
// 返回值：
//
//	bool: 如果路径匹配模式返回 true，否则返回 false
//
// 匹配规则优先级：
// 1. 前缀匹配：如果模式以 * 结尾（如 "/home/*"），则检查路径是否以此前缀开头
// 2. 通配符匹配：如果模式包含通配符（*、?、[]），则使用 filepath.Match 进行匹配
// 3. 精确匹配：否则进行精确字符串比较
//
// 通配符说明：
// - *：匹配任意长度的字符串（包括空字符串）
// - ?：匹配任意单个字符
// - []：匹配方括号内的任意单个字符
//
// 实现逻辑：
// 1. 检查模式是否以 * 结尾，若是则进行前缀匹配
// 2. 检查模式是否包含通配符，若是则使用 filepath.Match 进行匹配：
//   - 先尝试匹配完整路径
//   - 如果失败，再尝试仅匹配路径的基名（文件名部分）
//
// 3. 否则进行精确字符串比较
//
// 技术说明：
// - 使用 strings 包的 HasSuffix、TrimSuffix、HasPrefix 和 ContainsAny 函数进行字符串操作
// - 使用 filepath 包的 Match 函数进行通配符匹配
// - 使用 filepath 包的 Base 函数获取路径的基名部分
// - 这是一个纯函数，没有副作用，适合在并发环境中使用
func matchPath(pattern, path string) bool {
	// 检查模式是否以 * 结尾，用于前缀匹配
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(path, prefix)
	}

	// 检查模式是否包含任何通配符字符
	if strings.ContainsAny(pattern, "*?[]") {
		// 对单组件模式使用 filepath.Match
		matched, err := filepath.Match(pattern, path)
		if err == nil && matched {
			return true
		}
		// 也尝试仅匹配基名
		matched, err = filepath.Match(pattern, filepath.Base(path))
		return err == nil && matched
	}

	// 否则，进行精确匹配
	return pattern == path
}

// shouldIncludeDisk 根据过滤规则决定是否在监控中包含特定磁盘
// 功能：根据配置规则和系统默认规则，判断是否应该监控指定的磁盘
// 参数：
//
//	mountpoint string: 磁盘挂载点路径
//	device string: 磁盘设备名称
//	fstype string: 文件系统类型
//
// 返回值：
//
//	bool: 如果应该包含该磁盘返回 true，否则返回 false
//
// 过滤规则优先级（从高到低）：
// 1. 显式包含：如果磁盘匹配 DiskIncludes 中的任何模式，则包含（最高优先级）
// 2. 显式排除：如果磁盘匹配 DiskExcludes 中的任何模式，则排除
// 3. 默认排除：如果磁盘是特殊文件系统或设备类型，则默认排除
// 4. 默认包含：否则默认包含
//
// 特殊文件系统和设备类型说明：
// - 挂载点：/snap、/run、/dev、/proc、/sys 等系统特殊目录
// - Docker/容器相关：/var/lib/docker/overlay、/var/lib/containers/storage/overlay
// - 临时文件系统：tmpfs、devfs、udev、overlay
// - 特殊文件系统类型：squashfs、devtmpfs、overlay、proc、sysfs、cgroup、cgroup2
//
// 实现逻辑：
// 1. 遍历 DiskIncludes 配置，检查磁盘是否被显式包含
// 2. 如果未被显式包含，遍历 DiskExcludes 配置，检查磁盘是否被显式排除
// 3. 如果未被显式排除，检查磁盘是否是特殊文件系统或设备类型
// 4. 如果以上都不是，默认包含该磁盘
//
// 技术说明：
// - 使用 matchPath 函数进行路径模式匹配
// - 使用 strings.HasPrefix 函数检查挂载点和设备名称前缀
// - 使用字符串比较检查文件系统类型
// - 使用 zerolog 记录调试日志，记录匹配的模式
// - 此函数是 Agent 结构体的方法，使用配置中的 DiskIncludes 和 DiskExcludes 规则
func (a *Agent) shouldIncludeDisk(mountpoint, device, fstype string) bool {
	// 首先检查是否被显式包含 - 这具有最高优先级
	for _, pattern := range a.config.DiskIncludes {
		if matchPath(pattern, mountpoint) {
			log.Debug().Str("mount", mountpoint).Str("pattern", pattern).Msg("磁盘被模式匹配包含")
			return true
		}
	}

	// 然后检查是否被显式排除
	for _, pattern := range a.config.DiskExcludes {
		if matchPath(pattern, mountpoint) {
			log.Debug().Str("mount", mountpoint).Str("pattern", pattern).Msg("磁盘被模式匹配排除")
			return false
		}
	}

	// 默认跳过特殊文件系统（除非上面已显式包含）
	if strings.HasPrefix(mountpoint, "/snap") ||
		strings.HasPrefix(mountpoint, "/run") ||
		strings.HasPrefix(mountpoint, "/dev") ||
		strings.HasPrefix(mountpoint, "/proc") ||
		strings.HasPrefix(mountpoint, "/sys") ||
		strings.HasPrefix(mountpoint, "/var/lib/docker/overlay") ||
		strings.HasPrefix(mountpoint, "/var/lib/containers/storage/overlay") ||
		strings.HasPrefix(device, "tmpfs") ||
		strings.HasPrefix(device, "devfs") ||
		strings.HasPrefix(device, "udev") ||
		strings.HasPrefix(device, "overlay") ||
		fstype == "squashfs" ||
		fstype == "devtmpfs" ||
		fstype == "overlay" ||
		fstype == "proc" ||
		fstype == "sysfs" ||
		fstype == "cgroup" ||
		fstype == "cgroup2" {
		return false
	}

	// 默认包含其他所有磁盘
	return true
}

// getDevicePaths 返回用于检查 SMART 数据的设备路径列表
// 功能：跨平台获取磁盘设备路径，用于后续的 SMART 数据采集
// 参数：无
// 返回值：
//
//	[]string: 设备路径列表，如 ["/dev/sda", "/dev/nvme0n1", "/dev/rdisk0"]
//
// 跨平台支持：
// - macOS (darwin): 查找 /dev 目录下的原始磁盘设备（rdisk* 和 nvme*）
// - Linux: 查找 /dev 目录下的传统磁盘设备（sd*, nvme*, hd*）
//
// 设备命名规则：
// macOS:
// - disk0, disk1: 标准磁盘设备
// - rdisk0, rdisk1: 原始磁盘设备（用于 SMART 访问）
// - rdisk0s1, rdisk1s2: 磁盘分区（需要跳过）
// - nvme0, nvme1: NVMe 磁盘设备
//
// Linux:
// - sda, sdb: 标准 SATA/SCSI 磁盘设备
// - nvme0n1, nvme1n1: NVMe 磁盘设备
// - hda, hdb: IDE 磁盘设备（旧系统）
// - sda1, sdb2, nvme0n1p1: 磁盘分区（需要跳过）
//
// 实现逻辑：
// 1. 根据当前操作系统类型（runtime.GOOS）选择不同的设备查找策略
// 2. 读取 /dev 目录下的所有条目
// 3. 根据设备命名规则筛选出磁盘设备（排除分区）
// 4. 构建完整的设备路径并添加到结果列表
// 5. 如果读取 /dev 目录失败，返回空列表并记录调试日志
//
// 技术说明：
// - 使用 runtime.GOOS 检测当前操作系统类型
// - 使用 os.ReadDir 函数读取目录内容
// - 使用 filepath.Join 函数构建完整的设备路径
// - 使用 strings.HasPrefix 和 strings.Contains 函数进行设备名称匹配
// - 使用 zerolog 记录调试日志
// - 跳过分区设备，只返回完整的磁盘设备路径
func (a *Agent) getDevicePaths() []string {
	var devicePaths []string

	if runtime.GOOS == "darwin" {
		// macOS: 在 /dev 目录中查找磁盘设备
		entries, err := os.ReadDir("/dev")
		if err != nil {
			log.Debug().Err(err).Msg("读取 /dev 目录失败")
			return devicePaths
		}

		for _, entry := range entries {
			name := entry.Name()
			// macOS 使用 disk0, disk1 等和 rdisk0, rdisk1 等
			// 我们需要原始磁盘设备 (rdisk) 来访问 SMART 数据
			if strings.HasPrefix(name, "rdisk") && !strings.Contains(name, "s") {
				// 跳过分区 (rdisk0s1, rdisk1s2 等)
				devicePaths = append(devicePaths, filepath.Join("/dev", name))
			}
			// 也检查可能存在的 nvme 设备
			if strings.HasPrefix(name, "nvme") && !strings.Contains(name, "p") {
				devicePaths = append(devicePaths, filepath.Join("/dev", name))
			}
		}
	} else {
		// Linux: 查找传统设备名称
		entries, err := os.ReadDir("/dev")
		if err != nil {
			log.Debug().Err(err).Msg("读取 /dev 目录失败")
			return devicePaths
		}

		for _, entry := range entries {
			name := entry.Name()
			// 查找磁盘设备 (sda, sdb, nvme0n1 等)
			if !strings.HasPrefix(name, "sd") && !strings.HasPrefix(name, "nvme") && !strings.HasPrefix(name, "hd") {
				continue
			}

			// 跳过分区 (sda1, sdb2, nvme0n1p1 等)
			if strings.Contains(name, "p") && len(name) > 4 {
				continue
			}
			if len(name) > 3 && name[len(name)-1] >= '0' && name[len(name)-1] <= '9' {
				continue
			}

			devicePaths = append(devicePaths, filepath.Join("/dev", name))
		}
	}

	return devicePaths
}
