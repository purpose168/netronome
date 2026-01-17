// Copyright (c) 2024-2025, s0up 和 autobrr 贡献者.
// SPDX-License-Identifier: GPL-2.0-or-later

// Netronome 广播器模块
// 包名: broadcaster
// 功能: 定义广播器接口，用于在系统内广播速度更新信息
// 作者: s0up 和 autobrr 贡献者
// 创建日期: 2024
// 许可证: GPL-2.0-or-later

// 该模块定义了一个简单但关键的接口，用于在系统的不同组件之间传递速度更新信息。
// 广播器模式允许系统实现松耦合的组件通信，提高系统的可扩展性和可维护性。
//
// 主要功能：
// - 定义广播器接口，规范速度更新的广播行为
// - 支持多种广播实现（如内存广播、WebSocket 广播等）
// - 提供统一的接口，便于不同组件之间的通信
//
// 使用场景：
// - 网络速度监控组件向 UI 或其他服务广播速度更新
// - 实时数据可视化需要获取最新的速度信息
// - 系统日志记录或告警需要基于速度更新触发
//
// 依赖：
// - github.com/autobrr/netronome/internal/types: 包含 SpeedUpdate 类型定义

package broadcaster

import "github.com/autobrr/netronome/internal/types"

// Broadcaster 是一个广播器接口，用于广播速度更新信息
// 该接口定义了广播器必须实现的方法，提供了统一的接口用于组件间通信
//
// 设计原则：
// - 接口隔离原则：只定义必要的方法
// - 依赖倒置原则：组件依赖于接口而不是具体实现
// - 松耦合：允许不同组件之间通过接口通信，不直接依赖具体实现
//
// 使用方式：
// 1. 实现 BroadcastUpdate 方法，定义如何广播速度更新
// 2. 在需要广播速度更新的地方注入 Broadcaster 接口实例
// 3. 调用 BroadcastUpdate 方法发送速度更新

type Broadcaster interface {
	// BroadcastUpdate 广播速度更新信息
	// 参数：
	// - speedUpdate: 速度更新信息，包含上传、下载速度等数据
	//
	// 该方法用于将速度更新信息广播给所有注册的接收者
	// 具体实现可以根据需求选择不同的广播方式（如内存、网络等）
	//
	// 示例：
	// broadcaster.BroadcastUpdate(types.SpeedUpdate{
	//     Upload:   1024 * 1024,  // 1MB/s
	//     Download: 1024 * 1024 * 10, // 10MB/s
	//     Timestamp: time.Now(),
	// })
	BroadcastUpdate(speedUpdate types.SpeedUpdate)
}
