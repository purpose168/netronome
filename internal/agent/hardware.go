// Copyright (c) 2024-2025, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later
//
// 硬件信息收集模块
// 负责收集和返回系统硬件统计信息，包括CPU、内存、磁盘和温度等
// 使用gopsutil库实现跨平台硬件信息获取
package agent

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/sensors"
)

// handleHardwareStats 返回硬件统计信息的HTTP处理器
// @Summary 获取硬件统计信息
// @Description 返回包含CPU、内存、磁盘和温度等系统硬件详细信息
// @Tags 硬件监控
// @Produce json
// @Success 200 {object} HardwareStats 硬件统计信息对象
// @Failure 500 {object} map[string]string 错误信息
// @Router /api/hardware [get]
func (a *Agent) handleHardwareStats(c *gin.Context) {
	stats, err := a.getHardwareStats()
	if err != nil {
		log.Error().Err(err).Msg("获取硬件统计信息失败")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "获取硬件统计信息失败",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// getHardwareStats 收集硬件统计信息
// @Description 收集并返回系统硬件信息，包括CPU、内存、磁盘和温度传感器数据
// @Return *HardwareStats 硬件统计信息对象
// @Return error 错误信息
func (a *Agent) getHardwareStats() (*HardwareStats, error) {
	log.Debug().
		Strs("disk_includes", a.config.DiskIncludes).
		Strs("disk_excludes", a.config.DiskExcludes).
		Msg("开始收集硬件统计信息，应用磁盘过滤规则")

	stats := &HardwareStats{
		UpdatedAt: time.Now(),
	}

	// 获取CPU信息（型号和频率）
	cpuInfo, err := cpu.Info()
	if err == nil && len(cpuInfo) > 0 {
		// 使用第一个CPU条目的型号和频率信息
		stats.CPU.Model = cpuInfo[0].ModelName
		stats.CPU.Frequency = cpuInfo[0].Mhz

		log.Debug().
			Str("model", stats.CPU.Model).
			Float64("freq", stats.CPU.Frequency).
			Int("cpu_entries", len(cpuInfo)).
			Msg("获取到CPU信息")
	} else {
		log.Error().Err(err).Msg("获取CPU信息失败")
	}

	// 获取物理CPU核心数（cpu.Counts(false)返回物理核心数）
	physicalCores, err := cpu.Counts(false)
	if err == nil {
		stats.CPU.Cores = physicalCores
		log.Debug().Int("physical_cores", physicalCores).Msg("获取到物理CPU核心数")
	} else {
		log.Error().Err(err).Msg("获取物理CPU核心数失败")
		// 备用方案：如果cpuInfo可用，尝试从中获取核心数
		if len(cpuInfo) > 0 && cpuInfo[0].Cores > 0 {
			stats.CPU.Cores = int(cpuInfo[0].Cores)
		}
	}

	// 获取逻辑CPU线程数（cpu.Counts(true)返回逻辑线程数）
	logicalThreads, err := cpu.Counts(true)
	if err == nil {
		stats.CPU.Threads = logicalThreads
		log.Debug().Int("logical_threads", logicalThreads).Msg("获取到逻辑CPU线程数")
	} else {
		log.Error().Err(err).Msg("获取逻辑CPU线程数失败")
	}

	// 处理容器环境（如LXC）下的特殊情况，线程数可能小于核心数
	// 在容器环境中，由于CPU资源限制，逻辑线程数可能被限制为小于物理核心数
	// 例如，在LXC容器中，cpu.Counts(false)可能返回宿主机的物理核心数
	// 而cpu.Counts(true)返回的是容器实际可用的逻辑线程数
	if stats.CPU.Threads < stats.CPU.Cores && stats.CPU.Threads > 0 {
		log.Debug().
			Int("original_cores", stats.CPU.Cores).
			Int("threads", stats.CPU.Threads).
			Msg("线程数小于核心数（可能在容器中），调整核心数以匹配线程数")
		// 调整核心数以匹配线程数，这样负载平均值等计算会更准确
		stats.CPU.Cores = stats.CPU.Threads
	}

	// 获取CPU使用率百分比
	cpuPercent, err := cpu.Percent(time.Second, false)
	if err == nil && len(cpuPercent) > 0 {
		stats.CPU.UsagePercent = cpuPercent[0]
		log.Debug().Float64("usage", cpuPercent[0]).Msg("获取到CPU使用率")
	} else {
		log.Error().Err(err).Msg("获取CPU使用率失败")
	}

	// 获取负载平均值（仅Unix类系统支持）
	if runtime.GOOS != "windows" {
		loadAvg, err := getLoadAverage()
		if err == nil {
			stats.CPU.LoadAvg = loadAvg
			log.Debug().Floats64("load_avg", loadAvg).Msg("获取到负载平均值")
		} else {
			log.Error().Err(err).Msg("获取负载平均值失败")
		}
	}

	// 获取内存统计信息
	vmStat, err := mem.VirtualMemory()
	if err == nil {
		stats.Memory.Total = vmStat.Total
		stats.Memory.Free = vmStat.Free
		stats.Memory.Available = vmStat.Available
		stats.Memory.Cached = vmStat.Cached
		stats.Memory.Buffers = vmStat.Buffers

		// 获取ZFS ARC缓存大小（如果可用）
		zfsArcSize := getZFSARCSize()
		stats.Memory.ZFSArc = zfsArcSize

		// 计算已使用内存
		// 在Linux系统上，从"已使用"内存中排除缓存/缓冲区，因为它们可以被释放
		// Available已经考虑了可以回收的内存
		if runtime.GOOS == "linux" {
			// Linux计算方式：Total - Available
			// 这样可以得到实际应用程序使用的内存（不包括缓存/缓冲区）
			stats.Memory.Used = vmStat.Total - vmStat.Available
			// UsedPercent应该只反映应用程序内存使用情况
			stats.Memory.UsedPercent = float64(stats.Memory.Used) / float64(vmStat.Total) * 100
		} else {
			// 对于其他系统，使用提供的默认值
			stats.Memory.Used = vmStat.Used
			stats.Memory.UsedPercent = vmStat.UsedPercent
		}

		log.Debug().
			Uint64("total", vmStat.Total).
			Uint64("used", stats.Memory.Used).
			Uint64("free", vmStat.Free).
			Uint64("available", vmStat.Available).
			Uint64("cached", vmStat.Cached).
			Uint64("buffers", vmStat.Buffers).
			Uint64("zfs_arc", zfsArcSize).
			Float64("percent", stats.Memory.UsedPercent).
			Msg("获取到内存统计信息")
	} else {
		log.Error().Err(err).Msg("获取内存统计信息失败")
	}

	// 获取交换内存统计信息
	swapStat, err := mem.SwapMemory()
	if err == nil {
		stats.Memory.SwapTotal = swapStat.Total
		stats.Memory.SwapUsed = swapStat.Used
		stats.Memory.SwapPercent = swapStat.UsedPercent
		log.Debug().
			Uint64("total", swapStat.Total).
			Uint64("used", swapStat.Used).
			Float64("percent", swapStat.UsedPercent).
			Msg("获取到交换内存统计信息")
	} else {
		log.Debug().Err(err).Msg("获取交换内存统计信息失败（如果没有交换分区，这可能是正常的）")
	}

	// 获取磁盘统计信息（包括所有文件系统，包括fuse/虚拟文件系统）
	partitions, err := disk.Partitions(true)
	if err == nil {
		log.Debug().Int("partition_count", len(partitions)).Msg("获取到磁盘分区")

		// 构建设备名称到SMART信息的映射
		deviceInfoMap := make(map[string]struct{ model, serial string })
		devicePaths := a.getDevicePaths()
		for _, devicePath := range devicePaths {
			model, serial := a.getDiskInfo(devicePath)
			if model != "" || serial != "" {
				// 存储原始设备和分区名称的信息
				baseName := filepath.Base(devicePath)
				deviceInfoMap[devicePath] = struct{ model, serial string }{model, serial}
				deviceInfoMap["/dev/"+baseName] = struct{ model, serial string }{model, serial}
				// 还存储不带分区后缀的名称，用于匹配
				if strings.Contains(baseName, "disk") {
					// macOS风格：/dev/diskX -> 存储用于匹配 /dev/diskXsY
					deviceInfoMap["/dev/"+baseName] = struct{ model, serial string }{model, serial}
				}
			}
		}

		for _, partition := range partitions {
			// 根据过滤规则检查是否应包含此磁盘
			if !a.shouldIncludeDisk(partition.Mountpoint, partition.Device, partition.Fstype) {
				log.Debug().
					Str("mount", partition.Mountpoint).
					Str("device", partition.Device).
					Str("fstype", partition.Fstype).
					Msg("根据过滤规则跳过磁盘")
				continue
			}

			usage, err := disk.Usage(partition.Mountpoint)
			if err != nil {
				log.Debug().Err(err).Str("mount", partition.Mountpoint).Msg("获取磁盘使用情况失败")
				continue
			}

			// 如果磁盘太小（小于1GB），则跳过
			if usage.Total < 1024*1024*1024 {
				log.Debug().
					Str("mount", partition.Mountpoint).
					Uint64("total", usage.Total).
					Msg("跳过小磁盘")
				continue
			}

			diskStat := DiskStats{
				Path:        partition.Mountpoint,
				Device:      partition.Device,
				Fstype:      partition.Fstype,
				Total:       usage.Total,
				Used:        usage.Used,
				Free:        usage.Free,
				UsedPercent: usage.UsedPercent,
			}

			// 尝试为该设备查找SMART信息
			// 首先尝试精确匹配
			if info, ok := deviceInfoMap[partition.Device]; ok {
				diskStat.Model = info.model
				diskStat.Serial = info.serial
			} else {
				// 尝试按基础设备名称匹配（移除分区号）
				baseDevice := partition.Device
				// 移除分区后缀（例如，/dev/sda1 -> /dev/sda）
				if idx := strings.LastIndexAny(baseDevice, "0123456789"); idx > 0 && idx == len(baseDevice)-1 {
					// 移除尾部数字
					for idx > 0 && baseDevice[idx-1] >= '0' && baseDevice[idx-1] <= '9' {
						idx--
					}
					baseDevice = baseDevice[:idx]
				}
				// 还处理p1, p2样式的分区（例如，/dev/nvme0n1p1 -> /dev/nvme0n1）
				if strings.Contains(baseDevice, "p") && len(baseDevice) > 2 {
					if idx := strings.LastIndex(baseDevice, "p"); idx > 0 {
						if idx < len(baseDevice)-1 && baseDevice[idx+1] >= '0' && baseDevice[idx+1] <= '9' {
							baseDevice = baseDevice[:idx]
						}
					}
				}

				if info, ok := deviceInfoMap[baseDevice]; ok {
					diskStat.Model = info.model
					diskStat.Serial = info.serial
				}
			}

			stats.Disks = append(stats.Disks, diskStat)

			log.Debug().
				Str("path", partition.Mountpoint).
				Str("device", partition.Device).
				Str("model", diskStat.Model).
				Uint64("total", usage.Total).
				Float64("percent", usage.UsedPercent).
				Msg("将磁盘添加到统计信息中")
		}
		log.Debug().Int("disk_count", len(stats.Disks)).Msg("完成磁盘处理")
	} else {
		log.Error().Err(err).Msg("获取磁盘分区失败")
	}

	// 获取温度传感器数据
	temps, err := sensors.TemperaturesWithContext(context.Background())
	if err == nil {
		log.Debug().Int("sensor_count", len(temps)).Msg("获取到温度传感器")
		for _, temp := range temps {
			// 跳过读数为零或无效的传感器
			if temp.Temperature <= 0 || temp.Temperature > 200 {
				log.Trace().
					Str("sensor", temp.SensorKey).
					Float64("temp", temp.Temperature).
					Msg("跳过无效的温度读数")
				continue
			}

			// 过滤掉冗余的PMU传感器 - 只保留最重要的
			sensorKey := temp.SensorKey
			if strings.Contains(sensorKey, "PMU") {
				// 对于PMU传感器，只保留代表性样本
				if strings.Contains(sensorKey, "tdev") && !strings.HasSuffix(sensorKey, "tdev1") {
					// 跳过大多数tdev传感器，只保留每个PMU的tdev1
					continue
				}
				if strings.Contains(sensorKey, "tdie") && !strings.HasSuffix(sensorKey, "tdie1") {
					// 跳过大多数tdie传感器，只保留每个PMU的tdie1
					continue
				}
			}

			stats.Temperature = append(stats.Temperature, TemperatureStats{
				SensorKey:   temp.SensorKey,
				Temperature: temp.Temperature,
				Label:       "", // gopsutil不提供标签
				Critical:    temp.High,
			})

			log.Debug().
				Str("sensor", temp.SensorKey).
				Float64("temp", temp.Temperature).
				Float64("critical", temp.High).
				Msg("添加温度传感器")
		}
		log.Debug().Int("temp_sensor_count", len(stats.Temperature)).Msg("完成温度传感器处理")
	} else {
		log.Debug().Err(err).Msg("获取温度传感器失败（在某些系统上这可能是正常的）")
	}

	// 通过SMART获取磁盘温度（Linux: SATA+NVMe，macOS: 仅NVMe，需要特权）
	if runtime.GOOS == "linux" || runtime.GOOS == "darwin" {
		hddTemps := a.getHDDTemperatures()
		if len(hddTemps) > 0 {
			stats.Temperature = append(stats.Temperature, hddTemps...)
			log.Debug().Int("hdd_temp_count", len(hddTemps)).Msg("添加磁盘温度传感器")
		}
	}

	// 记录最终统计摘要
	log.Info().
		Float64("cpu_usage", stats.CPU.UsagePercent).
		Int("cpu_threads", stats.CPU.Threads).
		Float64("mem_percent", stats.Memory.UsedPercent).
		Int("disk_count", len(stats.Disks)).
		Int("temp_count", len(stats.Temperature)).
		Msg("硬件统计信息收集完成")

	return stats, nil
}

// getZFSARCSize 获取使用ZFS文件系统的系统上的ZFS ARC缓存大小
// ZFS ARC是ZFS文件系统的自适应替换缓存
func getZFSARCSize() uint64 {
	// 检查ZFS ARC统计文件（Linux系统）
	arcStatsPath := "/proc/spl/kstat/zfs/arcstats"
	data, err := os.ReadFile(arcStatsPath)
	if err != nil {
		// 不是ZFS系统或没有权限访问
		return 0
	}

	// 解析arcstats文件以找到大小
	lines := strings.SplitSeq(string(data), "\n")
	for line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 3 && fields[0] == "size" {
			size, err := strconv.ParseUint(fields[2], 10, 64)
			if err == nil {
				return size
			}
		}
	}

	return 0
}

// getLoadAverage 获取系统负载平均值（仅Unix类系统）
// 返回1分钟、5分钟和15分钟的负载平均值
func getLoadAverage() ([]float64, error) {
	if runtime.GOOS == "windows" {
		return nil, fmt.Errorf("Windows系统不支持负载平均值")
	}

	loadAvg, err := load.Avg()
	if err != nil {
		return nil, err
	}

	return []float64{loadAvg.Load1, loadAvg.Load5, loadAvg.Load15}, nil
}
