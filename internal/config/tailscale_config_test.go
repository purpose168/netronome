// 版权所有 (c) 2024-2025, s0up 和 autobrr 贡献者。
// SPDX-License-Identifier: GPL-2.0-or-later

// Tailscale 配置测试文件
// 该文件包含对 Tailscale 配置功能的单元测试
// 主要测试 Tailscale 配置的自动检测、验证、环境变量覆盖、向后兼容性等功能
package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTailscaleConfig_AutoDetection 测试 Tailscale 配置的自动检测功能
// 该测试验证 GetEffectiveMethod() 函数能否根据配置正确确定使用的 Tailscale 方法
func TestTailscaleConfig_AutoDetection(t *testing.T) {
	tests := []struct {
		name           string          // 测试用例名称
		config         TailscaleConfig // 测试用的 Tailscale 配置
		expectedMethod string          // 预期的 Tailscale 方法
		expectError    bool            // 是否预期出现错误
		errorContains  string          // 预期错误消息中包含的字符串
	}{
		{
			name: "自动模式下提供认证密钥时使用 tsnet",
			config: TailscaleConfig{
				Enabled: true,
				Method:  "auto",
				AuthKey: "tskey-auth-test",
			},
			expectedMethod: "tsnet",
		},
		{
			name: "自动模式下未提供认证密钥时使用 host",
			config: TailscaleConfig{
				Enabled: true,
				Method:  "auto",
				AuthKey: "",
			},
			expectedMethod: "host",
		},
		{
			name: "显式 tsnet 模式需要认证密钥",
			config: TailscaleConfig{
				Enabled: true,
				Method:  "tsnet",
				AuthKey: "",
			},
			expectError:   true,
			errorContains: "当 method 为 'tsnet' 时，auth_key 是必需的",
		},
		{
			name: "带认证密钥的显式 tsnet 模式",
			config: TailscaleConfig{
				Enabled: true,
				Method:  "tsnet",
				AuthKey: "tskey-auth-test",
			},
			expectedMethod: "tsnet",
		},
		{
			name: "显式 host 模式忽略认证密钥",
			config: TailscaleConfig{
				Enabled: true,
				Method:  "host",
				AuthKey: "tskey-auth-test", // 应该被忽略
			},
			expectedMethod: "host",
		},
		{
			name: "禁用的 Tailscale 忽略所有设置",
			config: TailscaleConfig{
				Enabled: false,
				Method:  "invalid",
				AuthKey: "test",
			},
			expectedMethod: "",
		},
	}

	// 遍历所有测试用例
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 获取有效的 Tailscale 方法
			method, err := tt.config.GetEffectiveMethod()

			// 如果预期出现错误
			if tt.expectError {
				require.Error(t, err, "预期出现错误但未得到")
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains, "错误消息不匹配")
				}
			} else {
				// 如果预期不出现错误
				require.NoError(t, err, "未预期的错误")
				assert.Equal(t, tt.expectedMethod, method, "方法不匹配")
			}
		})
	}
}

// TestTailscaleConfig_Validation 测试 Tailscale 配置的验证功能
// 该测试验证 Validate() 函数能否正确验证 Tailscale 配置的有效性
func TestTailscaleConfig_Validation(t *testing.T) {
	tests := []struct {
		name          string          // 测试用例名称
		config        TailscaleConfig // 测试用的 Tailscale 配置
		expectError   bool            // 是否预期出现错误
		errorContains string          // 预期错误消息中包含的字符串
	}{
		{
			name: "有效的 tsnet 配置",
			config: TailscaleConfig{
				Enabled:           true,
				Method:            "tsnet",
				AuthKey:           "tskey-auth-test",
				Hostname:          "test-node",
				Ephemeral:         true,
				AgentPort:         8200,
				AutoDiscover:      true,
				DiscoveryInterval: "5m",
			},
			expectError: false,
		},
		{
			name: "有效的 host 配置",
			config: TailscaleConfig{
				Enabled:           true,
				Method:            "host",
				AgentPort:         8200,
				AutoDiscover:      true,
				DiscoveryInterval: "5m",
			},
			expectError: false,
		},
		{
			name: "无效的方法",
			config: TailscaleConfig{
				Enabled: true,
				Method:  "invalid",
			},
			expectError:   true,
			errorContains: "无效的 method",
		},
		{
			name: "无效的发现间隔时间",
			config: TailscaleConfig{
				Enabled:           true,
				Method:            "host",
				AutoDiscover:      true,
				DiscoveryInterval: "invalid",
			},
			expectError:   true,
			errorContains: "无效的发现间隔",
		},
		{
			name: "负数的 Agent 端口",
			config: TailscaleConfig{
				Enabled:   true,
				Method:    "host",
				AgentPort: -1,
			},
			expectError:   true,
			errorContains: "无效的 agent 端口",
		},
		{
			name: "过高的 Agent 端口",
			config: TailscaleConfig{
				Enabled:   true,
				Method:    "host",
				AgentPort: 70000,
			},
			expectError:   true,
			errorContains: "无效的 agent 端口",
		},
	}

	// 遍历所有测试用例
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 验证配置的有效性
			err := tt.config.Validate()

			// 如果预期出现错误
			if tt.expectError {
				require.Error(t, err, "预期出现错误但未得到")
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains, "错误消息不匹配")
				}
			} else {
				// 如果预期不出现错误
				require.NoError(t, err, "未预期的错误")
			}
		})
	}
}

// TestTailscaleConfig_EnvironmentOverrides 测试环境变量对 Tailscale 配置的覆盖
// 该测试验证环境变量能否正确覆盖 Tailscale 配置的默认值
func TestTailscaleConfig_EnvironmentOverrides(t *testing.T) {
	// 保存原始环境变量，测试结束后恢复
	originalEnv := os.Environ()
	defer func() {
		os.Clearenv()
		for _, env := range originalEnv {
			pair := splitEnvPair(env)
			os.Setenv(pair[0], pair[1])
		}
	}()

	tests := []struct {
		name     string            // 测试用例名称
		envVars  map[string]string // 测试用的环境变量
		initial  TailscaleConfig   // 初始的 Tailscale 配置
		expected TailscaleConfig   // 预期的 Tailscale 配置（应用环境变量后）
	}{
		{
			name: "环境变量覆盖所有设置",
			envVars: map[string]string{
				"NETRONOME__TAILSCALE_ENABLED":            "true",
				"NETRONOME__TAILSCALE_METHOD":             "tsnet",
				"NETRONOME__TAILSCALE_AUTH_KEY":           "tskey-env-test",
				"NETRONOME__TAILSCALE_HOSTNAME":           "env-hostname",
				"NETRONOME__TAILSCALE_EPHEMERAL":          "true",
				"NETRONOME__TAILSCALE_STATE_DIR":          "/custom/state",
				"NETRONOME__TAILSCALE_CONTROL_URL":        "https://headscale.example.com",
				"NETRONOME__TAILSCALE_AGENT_PORT":         "8300",
				"NETRONOME__TAILSCALE_AUTO_DISCOVER":      "false",
				"NETRONOME__TAILSCALE_DISCOVERY_INTERVAL": "10m",
				"NETRONOME__TAILSCALE_DISCOVERY_PORT":     "8400",
				"NETRONOME__TAILSCALE_DISCOVERY_PREFIX":   "prod-",
			},
			initial: TailscaleConfig{
				Enabled: false,
				Method:  "host",
				AuthKey: "original-key",
			},
			expected: TailscaleConfig{
				Enabled:           true,
				Method:            "tsnet",
				AuthKey:           "tskey-env-test",
				Hostname:          "env-hostname",
				Ephemeral:         true,
				StateDir:          "/custom/state",
				ControlURL:        "https://headscale.example.com",
				AgentPort:         8300,
				AutoDiscover:      false,
				DiscoveryInterval: "10m",
				DiscoveryPort:     8400,
				DiscoveryPrefix:   "prod-",
			},
		},
		{
			name: "环境变量部分覆盖设置",
			envVars: map[string]string{
				"NETRONOME__TAILSCALE_METHOD":   "host",
				"NETRONOME__TAILSCALE_AUTH_KEY": "", // 应该清除认证密钥
			},
			initial: TailscaleConfig{
				Enabled:      true,
				Method:       "tsnet",
				AuthKey:      "original-key",
				AgentPort:    8200,
				AutoDiscover: true,
			},
			expected: TailscaleConfig{
				Enabled:      true,
				Method:       "host",
				AuthKey:      "",
				AgentPort:    8200,
				AutoDiscover: true,
			},
		},
	}

	// 遍历所有测试用例
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 清除环境变量
			os.Clearenv()

			// 设置测试用的环境变量
			for k, v := range tt.envVars {
				os.Setenv(k, v)
			}

			// 应用环境变量覆盖
			cfg := tt.initial
			cfg.loadFromEnv()

			// 比较结果
			assert.Equal(t, tt.expected.Enabled, cfg.Enabled, "Enabled 字段不匹配")
			assert.Equal(t, tt.expected.Method, cfg.Method, "Method 字段不匹配")
			assert.Equal(t, tt.expected.AuthKey, cfg.AuthKey, "AuthKey 字段不匹配")
			assert.Equal(t, tt.expected.Hostname, cfg.Hostname, "Hostname 字段不匹配")
			assert.Equal(t, tt.expected.Ephemeral, cfg.Ephemeral, "Ephemeral 字段不匹配")
			assert.Equal(t, tt.expected.StateDir, cfg.StateDir, "StateDir 字段不匹配")
			assert.Equal(t, tt.expected.ControlURL, cfg.ControlURL, "ControlURL 字段不匹配")
			assert.Equal(t, tt.expected.AgentPort, cfg.AgentPort, "AgentPort 字段不匹配")
			assert.Equal(t, tt.expected.AutoDiscover, cfg.AutoDiscover, "AutoDiscover 字段不匹配")
			assert.Equal(t, tt.expected.DiscoveryInterval, cfg.DiscoveryInterval, "DiscoveryInterval 字段不匹配")
			assert.Equal(t, tt.expected.DiscoveryPort, cfg.DiscoveryPort, "DiscoveryPort 字段不匹配")
			assert.Equal(t, tt.expected.DiscoveryPrefix, cfg.DiscoveryPrefix, "DiscoveryPrefix 字段不匹配")
		})
	}
}

// TestTailscaleConfig_BackwardCompatibility 测试 Tailscale 配置的向后兼容性
// 该测试验证 MigrateFromOldFormat() 函数能否正确将旧格式的配置迁移到新格式
func TestTailscaleConfig_BackwardCompatibility(t *testing.T) {
	tests := []struct {
		name     string          // 测试用例名称
		oldStyle Config          // 旧格式的配置
		expected TailscaleConfig // 预期的新格式配置
	}{
		{
			name: "prefer_host 为 true 的旧格式配置",
			oldStyle: Config{
				Tailscale: TailscaleConfig{
					Enabled:    true,
					AuthKey:    "",
					PreferHost: true,
					Agent: TailscaleAgentConfig{
						Enabled: true,
						Port:    8200,
					},
					Monitor: TailscaleMonitorConfig{
						AutoDiscover:      true,
						DiscoveryInterval: "5m",
						DiscoveryPort:     8200,
					},
				},
			},
			expected: TailscaleConfig{
				Enabled:           true,
				Method:            "host",
				AuthKey:           "",
				AgentPort:         8200,
				AutoDiscover:      true,
				DiscoveryInterval: "5m",
				DiscoveryPort:     8200,
			},
		},
		{
			name: "带认证密钥的旧格式配置",
			oldStyle: Config{
				Tailscale: TailscaleConfig{
					Enabled:    true,
					AuthKey:    "tskey-auth-old",
					Hostname:   "old-node",
					PreferHost: false,
					Agent: TailscaleAgentConfig{
						Enabled: true,
						Port:    8300,
					},
					Monitor: TailscaleMonitorConfig{
						AutoDiscover:    false,
						DiscoveryPrefix: "netronome-agent-",
					},
				},
			},
			expected: TailscaleConfig{
				Enabled:         true,
				Method:          "tsnet",
				AuthKey:         "tskey-auth-old",
				Hostname:        "old-node",
				AgentPort:       8300,
				AutoDiscover:    false,
				DiscoveryPrefix: "netronome-agent-",
			},
		},
		{
			name: "仅包含 Monitor 的旧格式配置",
			oldStyle: Config{
				Tailscale: TailscaleConfig{
					Enabled: false,
					Monitor: TailscaleMonitorConfig{
						AutoDiscover:      true,
						DiscoveryInterval: "2m",
						DiscoveryPort:     8200,
					},
				},
			},
			expected: TailscaleConfig{
				Enabled:           false,
				Method:            "host",
				AutoDiscover:      true,
				DiscoveryInterval: "2m",
				DiscoveryPort:     8200,
			},
		},
	}

	// 遍历所有测试用例
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 将旧格式配置迁移到新格式
			migrated := tt.oldStyle.Tailscale.MigrateFromOldFormat()

			// 比较迁移结果
			assert.Equal(t, tt.expected.Enabled, migrated.Enabled, "Enabled 字段不匹配")
			assert.Equal(t, tt.expected.Method, migrated.Method, "Method 字段不匹配")
			assert.Equal(t, tt.expected.AuthKey, migrated.AuthKey, "AuthKey 字段不匹配")
			assert.Equal(t, tt.expected.Hostname, migrated.Hostname, "Hostname 字段不匹配")
			assert.Equal(t, tt.expected.AgentPort, migrated.AgentPort, "AgentPort 字段不匹配")
			assert.Equal(t, tt.expected.AutoDiscover, migrated.AutoDiscover, "AutoDiscover 字段不匹配")
			assert.Equal(t, tt.expected.DiscoveryInterval, migrated.DiscoveryInterval, "DiscoveryInterval 字段不匹配")
			assert.Equal(t, tt.expected.DiscoveryPort, migrated.DiscoveryPort, "DiscoveryPort 字段不匹配")
			assert.Equal(t, tt.expected.DiscoveryPrefix, migrated.DiscoveryPrefix, "DiscoveryPrefix 字段不匹配")
		})
	}
}

// TestTailscaleConfig_IsAgentMode 测试 Tailscale 配置的 Agent 模式检测
// 该测试验证 IsAgentMode() 函数能否正确检测是否处于 Agent 模式
func TestTailscaleConfig_IsAgentMode(t *testing.T) {
	tests := []struct {
		name     string          // 测试用例名称
		config   TailscaleConfig // 测试用的 Tailscale 配置
		expected bool            // 预期的 Agent 模式状态
	}{
		{
			name: "启用且方法有效时处于 Agent 模式",
			config: TailscaleConfig{
				Enabled: true,
				Method:  "host",
			},
			expected: true,
		},
		{
			name: "禁用时不处于 Agent 模式",
			config: TailscaleConfig{
				Enabled: false,
				Method:  "host",
			},
			expected: false,
		},
		{
			name: "启用自动模式且无认证密钥时处于 Agent 模式",
			config: TailscaleConfig{
				Enabled: true,
				Method:  "auto",
				AuthKey: "",
			},
			expected: true,
		},
	}

	// 遍历所有测试用例
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 检测是否处于 Agent 模式
			result := tt.config.IsAgentMode()
			assert.Equal(t, tt.expected, result, "IsAgentMode() 结果不匹配")
		})
	}
}

// TestTailscaleConfig_IsServerDiscoveryMode 测试 Tailscale 配置的服务器发现模式检测
// 该测试验证 IsServerDiscoveryMode() 函数能否正确检测是否处于服务器发现模式
func TestTailscaleConfig_IsServerDiscoveryMode(t *testing.T) {
	tests := []struct {
		name     string          // 测试用例名称
		config   TailscaleConfig // 测试用的 Tailscale 配置
		expected bool            // 预期的服务器发现模式状态
	}{
		{
			name: "启用自动发现时处于服务器发现模式",
			config: TailscaleConfig{
				Enabled:      true,
				AutoDiscover: true,
			},
			expected: true,
		},
		{
			name: "禁用自动发现时不处于服务器发现模式",
			config: TailscaleConfig{
				Enabled:      true,
				AutoDiscover: false,
			},
			expected: false,
		},
		{
			name: "禁用 Tailscale 时即使启用自动发现也不处于服务器发现模式",
			config: TailscaleConfig{
				Enabled:      false,
				AutoDiscover: true,
			},
			expected: false,
		},
	}

	// 遍历所有测试用例
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 检测是否处于服务器发现模式
			result := tt.config.IsServerDiscoveryMode()
			assert.Equal(t, tt.expected, result, "IsServerDiscoveryMode() 结果不匹配")
		})
	}
}

// Helper functions
// splitEnvPair 分割环境变量键值对
// 参数: env - 环境变量字符串，格式为 "KEY=VALUE"
// 返回值: 包含键和值的字符串数组，格式为 ["KEY", "VALUE"]
func splitEnvPair(env string) []string {
	for i := 0; i < len(env); i++ {
		if env[i] == '=' {
			return []string{env[:i], env[i+1:]}
		}
	}
	return []string{env, ""}
}
