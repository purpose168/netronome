// Copyright (c) 2024-2025, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later
//
// 系统信息收集模块
// 负责收集和返回系统及网络接口信息，支持跨平台（Linux/macOS）
//
// Go语言特定概念解释：
// - runtime.GOOS: 获取当前操作系统类型（linux/darwin等）
// - os.ReadFile: 读取文件内容，Go 1.16+推荐使用的文件读取方式
// - os.Stat: 检查文件/目录是否存在
// - exec.Command: 执行外部系统命令
// - strings包: 提供字符串处理函数（TrimSpace、Fields、HasPrefix等）
// - strconv包: 提供字符串与基本数据类型的转换（ParseFloat、ParseInt等）
// - time包: 提供时间相关功能（Now、Unix等）
// - net包: 提供网络相关功能（Interfaces、IPNet等）
// - gin.Context: Gin框架的HTTP请求上下文对象
// - zerolog: 高性能结构化日志库（Debug、Error、Info等方法）
package agent

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

// handleSystemInfo 返回系统和网络接口信息的HTTP处理器
// @Summary 获取系统信息
// @Description 返回包含主机名、内核版本、网络接口等信息的系统详情
// @Tags 系统管理
// @Produce json
// @Success 200 {object} SystemInfo 系统信息对象
// @Failure 500 {object} map[string]string 错误信息
// @Router /api/system [get]
func (a *Agent) handleSystemInfo(c *gin.Context) {
	info, err := a.getSystemInfo()
	if err != nil {
		log.Error().Err(err).Msg("获取系统信息失败")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "获取系统信息失败",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, info)
}

// getSystemInfo 收集系统和网络接口信息
// @Description 跨平台收集系统信息，包括主机名、内核版本、运行时间、vnstat版本和网络接口详情
// @Return *SystemInfo 系统信息对象
// @Return error 错误信息
func (a *Agent) getSystemInfo() (*SystemInfo, error) {
	log.Debug().Msg("开始收集系统信息")

	info := &SystemInfo{
		Interfaces: make(map[string]InterfaceInfo),
		UpdatedAt:  time.Now(),
	}

	// 获取主机名
	hostname, err := os.Hostname()
	if err == nil {
		info.Hostname = hostname
		log.Debug().Str("hostname", hostname).Msg("获取到主机名")
	} else {
		log.Error().Err(err).Msg("获取主机名失败")
	}

	// 获取内核版本
	info.Kernel = runtime.GOOS + " " + runtime.GOARCH
	log.Debug().Str("default_kernel", info.Kernel).Msg("默认内核信息")

	switch runtime.GOOS {
	case "linux":
		cmd := exec.Command("uname", "-r")
		output, err := cmd.Output()
		if err == nil {
			kernel := strings.TrimSpace(string(output))
			info.Kernel = "Linux " + kernel
			log.Debug().Str("kernel", info.Kernel).Msg("获取到Linux内核版本")
		} else {
			log.Error().Err(err).Msg("执行uname -r命令失败")
		}
	case "darwin":
		if uname, err := exec.Command("uname", "-r").Output(); err == nil {
			info.Kernel = "Darwin " + strings.TrimSpace(string(uname))
			log.Debug().Str("kernel", info.Kernel).Msg("获取到Darwin内核版本")
		}
	}

	// 获取系统运行时间
	// 跨平台实现：
	// - Linux: 从/proc/uptime文件读取，该文件包含系统运行的秒数
	// - macOS: 使用sysctl命令获取系统启动时间，然后计算与当前时间的差值
	switch runtime.GOOS {
	case "linux":
		log.Debug().Msg("从/proc/uptime获取Linux系统运行时间")
		// /proc/uptime文件格式：[运行时间(秒)] [空闲时间(秒)]
		data, err := os.ReadFile("/proc/uptime")
		if err == nil {
			content := string(data)
			log.Debug().Str("proc_uptime_content", content).Msg("读取/proc/uptime文件")
			fields := strings.Fields(content) // 分割为两个字段
			if len(fields) > 0 {
				// 解析第一个字段为浮点数，表示系统运行时间（秒）
				uptime, parseErr := strconv.ParseFloat(fields[0], 64)
				if parseErr == nil {
					info.Uptime = int64(uptime) // 转换为整数秒
					log.Debug().Int64("uptime_seconds", info.Uptime).Msg("解析系统运行时间")
				} else {
					log.Error().Err(parseErr).Str("field", fields[0]).Msg("解析系统运行时间失败")
				}
			} else {
				log.Error().Str("content", content).Msg("/proc/uptime文件中没有字段")
			}
		} else {
			log.Error().Err(err).Msg("读取/proc/uptime文件失败")
		}
	case "darwin":
		// macOS: 使用sysctl命令获取系统启动时间
		if output, err := exec.Command("sysctl", "-n", "kern.boottime").Output(); err == nil {
			// sysctl命令输出格式: { sec = 1234567890, usec = 123456 }
			// sec表示系统启动的Unix时间戳（秒）
			str := strings.TrimSpace(string(output))
			log.Debug().Str("sysctl_output", str).Msg("获取到macOS系统启动时间")
			// 解析sec字段
			if idx := strings.Index(str, "sec = "); idx != -1 {
				str = str[idx+6:] // 截取sec = 之后的字符串
				if idx := strings.Index(str, ","); idx != -1 {
					str = str[:idx] // 截取到逗号之前的字符串，得到纯数字
					if sec, err := strconv.ParseInt(str, 10, 64); err == nil {
						// 系统运行时间 = 当前时间 - 启动时间
						info.Uptime = time.Now().Unix() - sec
						log.Debug().Int64("uptime_seconds", info.Uptime).Msg("计算macOS系统运行时间")
					}
				}
			}
		}
	}

	// 获取vnstat版本
	cmd := exec.Command("vnstat", "--version")
	output, err := cmd.Output()
	if err == nil {
		versionOutput := string(output)
		log.Debug().Str("vnstat_version_output", versionOutput).Msg("vnstat版本输出")
		lines := strings.Split(versionOutput, "\n")
		if len(lines) > 0 {
			info.VnstatVersion = strings.TrimSpace(lines[0])
			log.Debug().Str("vnstat_version", info.VnstatVersion).Msg("解析vnstat版本")
		}
	} else {
		log.Error().Err(err).Msg("获取vnstat版本失败")
	}

	// 获取网络接口信息
	interfaces, err := net.Interfaces()
	if err == nil {
		log.Debug().Msg("获取网络接口列表")
		for _, iface := range interfaces {
			// 跳过回环接口
			if iface.Flags&net.FlagLoopback != 0 {
				log.Debug().Str("interface", iface.Name).Msg("跳过回环接口")
				continue
			}

			log.Debug().Str("interface", iface.Name).Msg("处理网络接口")

			ifaceInfo := InterfaceInfo{
				Name: iface.Name,
				IsUp: iface.Flags&net.FlagUp != 0,
			}

			// 获取IP地址
			addrs, err := iface.Addrs()
			if err == nil && len(addrs) > 0 {
				log.Debug().Str("interface", iface.Name).Int("addr_count", len(addrs)).Msg("获取到IP地址列表")
				for _, addr := range addrs {
					if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.To4() != nil {
						ifaceInfo.IPAddress = ipnet.IP.String()
						log.Debug().Str("interface", iface.Name).Str("ip", ifaceInfo.IPAddress).Msg("获取到IPv4地址")
						break
					}
				}
			} else if err != nil {
				log.Error().Err(err).Str("interface", iface.Name).Msg("获取IP地址失败")
			}

			// 获取网络接口链接速度（仅Linux系统支持）
			if runtime.GOOS == "linux" {
				// 虚拟接口检测：
				// 方法1：检查是否为桥接接口
				// 如果/sys/class/net/{interface}/bridge目录存在，则为桥接接口
				bridgePath := fmt.Sprintf("/sys/class/net/%s/bridge", iface.Name)
				_, isBridge := os.Stat(bridgePath)

				// 方法2：检查接口名称是否匹配常见的虚拟接口命名模式
				// 虚拟接口没有实际的物理链接速度，因此需要特殊处理
				isVirtual := isBridge == nil || // 桥接接口
					strings.HasPrefix(iface.Name, "vmbr") || // 虚拟机桥接接口（如Proxmox）
					strings.HasPrefix(iface.Name, "br") || // 通用桥接接口
					strings.HasPrefix(iface.Name, "virbr") || // 虚拟桥接接口（如libvirt/KVM）
					strings.HasPrefix(iface.Name, "docker") || // Docker容器接口
					strings.HasPrefix(iface.Name, "veth") || // 虚拟以太网接口（用于容器网络）
					strings.HasPrefix(iface.Name, "tap") || // TAP接口（二层虚拟网络设备）
					strings.HasPrefix(iface.Name, "tun") || // TUN接口（三层虚拟网络设备）
					strings.Contains(iface.Name, "bond") // 绑定接口（将多个物理接口绑定为一个）

				if isVirtual {
					// 对于虚拟接口，设置LinkSpeed为-1表示它是虚拟的
					// 这样前端可以根据这个值区分物理接口和虚拟接口
					ifaceInfo.LinkSpeed = -1
					log.Debug().Str("interface", iface.Name).Msg("检测到虚拟/桥接接口")
				} else {
					// 对于物理接口，从/sys/class/net/{interface}/speed文件读取实际速度
					// 速度单位为Mbps（兆比特每秒）
					speedFile := fmt.Sprintf("/sys/class/net/%s/speed", iface.Name)
					data, err := os.ReadFile(speedFile)
					if err == nil {
						speedStr := strings.TrimSpace(string(data))
						log.Debug().Str("interface", iface.Name).Str("speed_raw", speedStr).Msg("读取链接速度")
						if speed, err := strconv.Atoi(speedStr); err == nil && speed > 0 {
							ifaceInfo.LinkSpeed = speed
							log.Debug().Str("interface", iface.Name).Int("speed", speed).Msg("获取到链接速度")
						}
					} else {
						log.Debug().Err(err).Str("interface", iface.Name).Str("file", speedFile).Msg("链接速度不可用")
					}
				}
			}

			// 获取vnstat接口别名（如果已配置）
			if output, err := exec.Command("vnstat", "--json", "-i", iface.Name).Output(); err == nil {
				var interfaceData map[string]any
				if json.Unmarshal(output, &interfaceData) == nil {
					if interfaces, ok := interfaceData["interfaces"].([]any); ok && len(interfaces) > 0 {
						if ifaceData, ok := interfaces[0].(map[string]any); ok {
							// 获取接口别名
							if alias, ok := ifaceData["alias"].(string); ok {
								ifaceInfo.Alias = alias
							}
							// 获取流量统计
							if traffic, ok := ifaceData["traffic"].(map[string]any); ok {
								if total, ok := traffic["total"].(map[string]any); ok {
									if rx, ok := total["rx"].(float64); ok {
										if tx, ok := total["tx"].(float64); ok {
											ifaceInfo.BytesTotal = int64(rx + tx)
										}
									}
								}
							}
						}
					}
				}
			}

			// 将接口信息添加到系统信息中
			info.Interfaces[iface.Name] = ifaceInfo
			log.Debug().
				Str("interface", iface.Name).
				Bool("is_up", ifaceInfo.IsUp).
				Str("ip", ifaceInfo.IPAddress).
				Int("speed", ifaceInfo.LinkSpeed).
				Msg("将接口添加到系统信息中")
		}
	} else {
		log.Error().Err(err).Msg("获取网络接口失败")
	}

	// 记录收集到的系统信息摘要
	log.Info().
		Str("hostname", info.Hostname).
		Str("kernel", info.Kernel).
		Int64("uptime", info.Uptime).
		Str("vnstat_version", info.VnstatVersion).
		Int("interface_count", len(info.Interfaces)).
		Msg("系统信息收集完成")

	return info, nil
}
