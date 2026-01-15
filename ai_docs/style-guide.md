# 前端样式指南

本文档概述了 Netronome 项目前端开发的既定模式和约定。它是维护代码库一致性的权威参考。

## 目录

1. [组件架构](#组件架构)
2. [样式约定](#样式约定)
3. [动画模式](#动画模式)
4. [响应式设计](#响应式设计)
5. [颜色系统](#颜色系统)
6. [排版](#排版)
7. [状态管理](#状态管理)
8. [可访问性](#可访问性)
9. [性能优化](#性能优化)
10. [文件组织](#文件组织)
11. [快速参考](#快速参考)

---

## 组件架构

### React 模式

**使用带有 TypeScript 接口的函数组件：**

```typescript
interface ComponentProps {
  title: string; // 标题
  isActive?: boolean; // 是否激活（可选）
  onAction?: () => void; // 动作回调（可选）
}

export const Component: React.FC<ComponentProps> = ({
  title,
  isActive = false,
  onAction,
}) => {
  // 组件实现
};
```

**创建 UI 组件时扩展原生 HTML 属性：**

```typescript
interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  isLoading?: boolean; // 是否加载中（可选）
  variant?: "primary" | "secondary"; // 按钮变体（可选）
}

export const Button: React.FC<ButtonProps> = ({
  isLoading = false,
  variant = "primary",
  children,
  className,
  ...props
}) => {
  // 过滤与 motion 冲突的属性
  const { whileHover, whileTap, ...buttonProps } = props;

  return (
    <motion.button
      whileHover={{ scale: 1.02 }} // 悬停时缩放
      whileTap={{ scale: 0.98 }} // 点击时缩放
      className={cn(baseStyles, variantStyles[variant], className)}
      {...buttonProps}
    >
      {isLoading ? <LoadingSpinner /> : children} {/* 加载状态或子内容 */}
    </motion.button>
  );
};
```

### 组件变体架构 (CVA)

**使用 class-variance-authority 处理复杂组件变体：**

```typescript
import { cva, type VariantProps } from "class-variance-authority";
import { cn } from "@/lib/utils";

const buttonVariants = cva(
  "inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-md text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500/50 disabled:pointer-events-none disabled:opacity-50",
  {
    variants: {
      variant: {
        default: "bg-blue-500 text-white hover:bg-blue-600 dark:bg-blue-600 dark:hover:bg-blue-700 shadow-lg", // 默认变体
        destructive: "bg-red-500 text-white hover:bg-red-600 dark:bg-red-600 dark:hover:bg-red-700 shadow-lg", // 危险变体
        outline: "border border-gray-300 bg-white text-gray-900 hover:bg-gray-50 hover:text-gray-900 dark:border-gray-700 dark:bg-gray-900 dark:text-gray-100 dark:hover:bg-gray-800 dark:hover:text-gray-100 shadow-lg", // 轮廓变体
        secondary: "bg-gray-200/50 text-gray-900 hover:bg-gray-300/50 dark:bg-gray-800/50 dark:text-gray-100 dark:hover:bg-gray-800 border border-gray-300 dark:border-gray-800 shadow-lg", // 次要变体
        ghost: "text-gray-900 hover:bg-gray-100 hover:text-gray-900 dark:text-gray-100 dark:hover:bg-gray-800 dark:hover:text-gray-100", // 幽灵变体
        link: "text-blue-500 underline-offset-4 hover:underline dark:text-blue-400", // 链接变体
      },
      size: {
        default: "h-9 px-4 py-2", // 默认尺寸
        sm: "h-8 rounded-md px-3 text-xs", // 小尺寸
        lg: "h-10 rounded-md px-8", // 大尺寸
        icon: "h-9 w-9", // 图标尺寸
      },
    },
    defaultVariants: {
      variant: "default", // 默认变体
      size: "default", // 默认尺寸
    },
  }
);

export interface ButtonProps
  extends React.ButtonHTMLAttributes<HTMLButtonElement>,
    VariantProps<typeof buttonVariants> {
  asChild?: boolean; // 是否使用子元素作为根组件
  isLoading?: boolean; // 是否加载中
}

const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  ({ className, variant, size, asChild = false, isLoading = false, children, ...props }, ref) => {
    const Comp = asChild ? Slot : "button"; // 确定根组件类型
    return (
      <Comp
        className={cn(buttonVariants({ variant, size, className }))}
        ref={ref}
        {...props}
      >
        {isLoading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />} {/* 加载指示器 */}
        {children} {/* 按钮内容 */}
      </Comp>
    );
  }
);
Button.displayName = "Button"; // 设置组件显示名称
```

### Radix UI 集成模式

**使用 Radix UI 原语进行适当的组合：**

```typescript
import * as DialogPrimitive from "@radix-ui/react-dialog";
import { Slot } from "@radix-ui/react-slot";

// 复合组件模式
function Dialog(props: React.ComponentProps<typeof DialogPrimitive.Root>) {
  return <DialogPrimitive.Root data-slot="dialog" {...props} />;
}

function DialogContent({
  className,
  children,
  showCloseButton = true,
  ...props
}: React.ComponentProps<typeof DialogPrimitive.Content> & {
  showCloseButton?: boolean; // 是否显示关闭按钮
}) {
  return (
    <DialogPortal>
      <DialogOverlay />
      <DialogPrimitive.Content
        data-slot="dialog-content"
        className={cn(
          "bg-gray-50/95 dark:bg-gray-850/95 fixed top-[50%] left-[50%] z-50 grid w-full max-w-lg translate-x-[-50%] translate-y-[-50%] gap-4 rounded-xl border p-6 shadow-xl",
          className
        )}
        {...props}
      >
        {children} {/* 对话框内容 */}
        {showCloseButton && (
          <DialogPrimitive.Close className="absolute top-4 right-4">
            <XMarkIcon /> {/* 关闭图标 */}
            <span className="sr-only">关闭</span> {/* 屏幕阅读器文本 */}
          </DialogPrimitive.Close>
        )}
      </DialogPrimitive.Content>
    </DialogPortal>
  );
}

// 导出所有子组件
export { Dialog, DialogContent, DialogHeader, DialogFooter, DialogTitle };
```

### 组件组合

**创建带有基于状态样式的可复用子组件：**

```typescript
const MetricCard: React.FC<{
  icon: React.ReactNode; // 图标
  title: string; // 标题
  value: string; // 数值
  unit: string; // 单位
  average?: string; // 平均值（可选）
  status?: "normal" | "warning" | "error" | "success"; // 状态（可选）
}> = ({ icon, title, value, unit, average, status = "normal" }) => {
  const statusColors = {
    normal: "", // 正常状态
    success: "ring-1 ring-emerald-500/20 bg-emerald-500/5", // 成功状态样式
    warning: "ring-1 ring-amber-500/20 bg-amber-500/5", // 警告状态样式
    error: "ring-1 ring-red-500/20 bg-red-500/5", // 错误状态样式
  };

  const valueColors = {
    normal: "text-gray-900 dark:text-white", // 正常状态文本颜色
    success: "text-emerald-600 dark:text-emerald-400", // 成功状态文本颜色
    warning: "text-amber-600 dark:text-amber-400", // 警告状态文本颜色
    error: "text-red-600 dark:text-red-400", // 错误状态文本颜色
  };

  return (
    <div className={`bg-gray-50/95 dark:bg-gray-850/95 p-3 sm:p-4 rounded-xl border border-gray-200 dark:border-gray-800 shadow-lg ${statusColors[status]}`}>
      <div className="flex items-center gap-2 sm:gap-3 mb-2">
        <div className="text-gray-600 dark:text-gray-400 flex-shrink-0">{icon}</div> {/* 图标 */}
        <h3 className="text-gray-700 dark:text-gray-300 font-medium text-sm sm:text-base truncate">
          {title} {/* 标题 */}
        </h3>
      </div>
      <div className="flex items-baseline gap-1 sm:gap-2">
        <span className={`text-xl sm:text-2xl font-bold ${valueColors[status]}`}>
          {value} {/* 数值 */}
        </span>
        <span className="text-gray-600 dark:text-gray-400 text-sm sm:text-base">{unit}</span> {/* 单位 */}
      </div>
      {average && (
        <div className="mt-1 text-xs sm:text-sm text-gray-600 dark:text-gray-400 truncate">
          <span className="hidden sm:inline">Average: </span> {/* 大屏显示完整文本 */}
          <span className="sm:hidden">Avg: </span> {/* 小屏显示缩写 */}
          {average} {unit}
        </div>
      )}
    </div>
  );
};
```

---

## 样式约定

### Tailwind CSS 使用

**使用具有一致模式的工具类：**

```typescript
// 背景模式
className = "bg-gray-50/95 dark:bg-gray-850/95";

// 边框模式
className = "border border-gray-200 dark:border-gray-900";

// 文本模式
className = "text-gray-700 dark:text-gray-300";

// 交互状态
className = "hover:bg-gray-300/50 dark:hover:bg-gray-800 transition-colors";
```

**使用 `cn()` 工具函数处理条件类：**

```typescript
import { cn } from "@/lib/utils";

const Component = ({ isActive, className }) => (
  <div className={cn("base-styles", isActive && "active-styles", className)}>
    Content
  </div>
);
```

### 深色模式支持

**始终提供带有明确文本颜色的深色模式替代方案：**

```typescript
// 标准模式 - 始终包含文本颜色以确保可访问性
className = "bg-white dark:bg-gray-800 text-gray-900 dark:text-white";

// 按钮变体必须明确指定文本颜色
className = "bg-white text-gray-900 hover:bg-gray-50 dark:bg-gray-900 dark:text-gray-100 dark:hover:bg-gray-800";

// 带透明度的背景
className = "bg-gray-50/95 dark:bg-gray-850/95";

// 边框样式
className = "border-gray-200 dark:border-gray-800";
```

**重要提示**：始终指定明确的文本颜色，特别是对于按钮等交互元素。依赖继承的文本颜色可能会在深色模式下导致可访问性问题。

### 阴影和背景效果

**使用一致的阴影模式：**

```typescript
// 标准阴影
className = "shadow-lg";

// 带背景模糊的阴影
className = "backdrop-blur-sm bg-blue-500/10 border border-blue-500/30";
```

---

## 动画模式

### Motion/React 集成

**为了性能，在组件外部定义动画配置：**

```typescript
// 动画配置移到组件外部，防止重复创建
const SPRING_TRANSITION = {
  type: "spring" as const,
  stiffness: 500, // 刚度
  damping: 30, // 阻尼
} as const;

const Component = () => (
  <motion.div
    initial={{ opacity: 0, y: 20 }} // 初始状态：透明度为0，向下偏移20px
    animate={{ opacity: 1, y: 0 }} // 动画状态：透明度为1，位置归零
    transition={SPRING_TRANSITION} // 应用弹簧过渡效果
  >
    Content
  </motion.div>
);
```

**标准动画模式：**

```typescript
// 页面/区块进入动画
<motion.div
  initial={{ opacity: 0, y: 20 }} // 初始状态：透明度为0，向下偏移20px
  animate={{ opacity: 1, y: 0 }} // 动画状态：透明度为1，位置归零
  exit={{ opacity: 0, y: 20 }} // 退出状态：透明度为0，向下偏移20px
  transition={{ duration: 0.5 }} // 动画持续时间：0.5秒
>

// 列表项进入动画
<motion.tr
  initial={{ opacity: 0, y: 20 }}
  animate={{ opacity: 1, y: 0 }}
  transition={{ duration: 0.3 }} // 列表项动画稍快：0.3秒
>

// 按钮交互动画
<motion.button
  whileHover={{ scale: 1.02 }} // 悬停时轻微放大（1.02倍）
  whileTap={{ scale: 0.98 }} // 点击时轻微缩小（0.98倍）
  transition={{ duration: 0.2 }} // 交互动画最快：0.2秒
>

// 布局过渡动画
<motion.div
  layoutId="activeTab" // 共享布局ID，用于平滑过渡
  transition={{ type: "spring", stiffness: 500, damping: 30 }} // 弹簧过渡效果
>
```

### 动画时间安排

**使用一致的时间值：**

- **快速交互**：`0.2s`（按钮悬停/点击等）
- **组件过渡**：`0.3s`（列表项、小部件等）
- **页面过渡**：`0.5s`（页面加载、大区块切换等）
- **反馈延迟**：`2s`（操作成功/失败提示等）

---

## 响应式设计

### 移动优先方法

**始终使用响应式前缀：**

```typescript
// 移动优先断点
className = "px-2 sm:px-4 md:px-6 lg:px-8"; // 从移动端到桌面端的内边距递增

// 组件可见性控制
className = "hidden md:block"; // 仅桌面端显示
className = "md:hidden"; // 仅移动端显示

// 响应式网格布局
className = "grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4"; // 移动端单列，桌面端四列
```

### 响应式数据显示

**使用表格/卡片响应式模式：**

```typescript
// 桌面端表格视图
<div className="hidden md:block"> {/* 仅在中等及以上屏幕显示 */}
  <table className="w-full text-sm">
    {/* 表格内容 */}
  </table>
</div>

// 移动端卡片视图
<div className="md:hidden space-y-3"> {/* 仅在小屏幕显示 */}
  {items.map(item => (
    <div className="bg-gray-200/50 dark:bg-gray-800/50 rounded-lg p-4">
      {/* 卡片内容 */}
    </div>
  ))}
</div>
```

### 文本响应式设计

**使用响应式文本大小：**

```typescript
// 响应式文本大小
className = "text-sm sm:text-base"; // 小屏幕使用sm，中等屏幕使用base

// 响应式间距
className = "space-x-1 sm:space-x-2"; // 小屏幕间距1，中等屏幕间距2

// 响应式内边距
className = "px-2 sm:px-6 py-2 sm:py-3"; // 小屏幕边距小，中等屏幕边距大
```

---

## 颜色系统

### 主要颜色调色板

**核心 -400 变体（图表、数据可视化）：**

```typescript
// Blue-400: #60a5fa - 主要操作、下载指标
className = "text-blue-400"; // 文本颜色
className = "stroke-blue-400"; // 描边颜色

// Emerald-400: #34d399 - 成功状态、上传指标
className = "text-emerald-400";
className = "stroke-emerald-400";

// Amber-400: #fbbf24 - 警告状态、延迟指标
className = "text-amber-400";
className = "stroke-amber-400";

// Purple-400: #c084fc - 特殊操作、抖动指标
className = "text-purple-400";
className = "stroke-purple-400";
```

### 语义颜色使用

**主要操作（蓝色）：**

```typescript
className = "text-blue-600 dark:text-blue-400"; // 主要文本颜色
className = "bg-blue-500/10 border border-blue-500/30"; // 半透明背景带边框
```

**成功/积极（翠绿色/绿色）：**

```typescript
className = "text-emerald-600 dark:text-emerald-400"; // 翠绿色用于状态
className = "text-green-600 dark:text-green-400"; // 用于数据显示
```

**警告/注意（琥珀色/黄色）：**

```typescript
className = "text-amber-600 dark:text-amber-400"; // 琥珀色用于状态
className = "text-yellow-600 dark:text-yellow-400"; // 用于数据显示
```

**错误/危险（红色）：**

```typescript
className = "text-red-600 dark:text-red-400"; // 红色文本
className = "bg-red-500/10 border border-red-500/30"; // 红色半透明背景带边框
```

**特殊/强调（紫色）：**

```typescript
className = "text-purple-600 dark:text-purple-400"; // 紫色文本
className = "bg-purple-500/10 text-purple-600 dark:text-purple-400"; // 紫色半透明背景
```

### 服务类型颜色

**速度测试类型：**

```typescript
// iperf3
className = "bg-purple-500/10 text-purple-600 dark:text-purple-400"; // 紫色背景

// LibreSpeed
className = "bg-blue-500/10 text-blue-600 dark:text-blue-400"; // 蓝色背景

// Speedtest.net
className =
  "bg-emerald-200/50 dark:bg-emerald-500/10 text-emerald-700 dark:text-emerald-400"; // 翠绿色背景
```

---

## 排版

### 字体模式

**标准文本：**

```typescript
className = "font-normal"; // 默认字体权重
className = "font-medium"; // 标题、标签
className = "font-semibold"; // 章节标题
className = "font-bold"; // 强调、数值
```

**等宽字体用于数据：**

```typescript
className = "font-mono"; // 数字、技术数据
```

### 文本大小

**标准层级：**

```typescript
className = "text-xs"; // 12px - 小标签、元数据
className = "text-sm"; // 14px - 正文、描述
className = "text-base"; // 16px - 标准正文
className = "text-lg"; // 18px - 大正文
className = "text-xl"; // 20px - 章节标题
className = "text-2xl"; // 24px - 页面标题、大数值
```

### 文本颜色

**标准文本颜色：**

```typescript
className = "text-gray-900 dark:text-white"; // 主要文本
className = "text-gray-700 dark:text-gray-300"; // 次要文本
className = "text-gray-600 dark:text-gray-400"; // 弱化文本
className = "text-gray-500 dark:text-gray-500"; // 禁用/占位符
```

---

## 状态管理

### 本地状态模式

**使用 useState 管理组件状态：**

```typescript
const [isOpen, setIsOpen] = useState(false); // 布尔状态
const [displayCount, setDisplayCount] = useState(5); // 数字状态
```

**使用 localStorage 存储用户偏好：**

```typescript
const [isRecentTestsOpen] = useState(() => {
  const saved = localStorage.getItem("recent-tests-open"); // 从 localStorage 获取
  return saved === null ? true : saved === "true"; // 默认值为 true
});

useEffect(() => {
  localStorage.setItem("recent-tests-open", open.toString()); // 保存到 localStorage
}, [open]); // 依赖于 open 状态
```

### TanStack Query 模式

**带有缓存策略的高级查询配置：**

```typescript
const statusQuery = useQuery<MonitorStatus>({
  queryKey: ["monitor-agent-status", agent.id], // 查询键，用于缓存和失效
  queryFn: () => getMonitorAgentStatus(agent.id), // 数据获取函数
  refetchInterval: agent.enabled ? MONITOR_REFRESH_INTERVALS.STATUS : false, // 自动刷新间隔
  staleTime: MONITOR_REFRESH_INTERVALS.STATUS / 2, // 数据新鲜时间（刷新间隔的一半）
  gcTime: 5 * 60 * 1000, // 缓存保留时间（5分钟）
  enabled: agent.enabled && includeData, // 条件执行
});
```

**基于多个条件的条件查询：**

```typescript
const hardwareStatsQuery = useQuery<HardwareStats>({
  queryKey: ["monitor-agent-hardware", agent.id], // 查询键
  queryFn: () => getMonitorAgentHardwareStats(agent.id), // 数据获取函数
  refetchInterval: agent.enabled ? MONITOR_REFRESH_INTERVALS.HARDWARE_STATS : false, // 自动刷新间隔
  staleTime: MONITOR_REFRESH_INTERVALS.HARDWARE_STATS, // 数据新鲜时间
  enabled: agent.enabled && includeHardwareStats && statusQuery.data?.connected, // 多个条件
});
```

**带有失效处理的直接缓存操作：**

```typescript
// 直接更新缓存，然后进行失效处理
queryClient.setQueryData(["packetloss", "history", monitorId], freshHistory);

// 强制 React Query 通知所有订阅者
queryClient.invalidateQueries({
  queryKey: ["packetloss", "history", monitorId], // 精确的查询键
  exact: true, // 精确匹配
});
```

**带有 toast 通知的 Mutation：**

```typescript
const startMutation = useMutation({
  mutationFn: () => startMonitorAgent(agent.id), // 执行的操作
  onSuccess: () => {
    // 成功后失效相关查询
    queryClient.invalidateQueries({ queryKey: ["monitor-agents"] });
    queryClient.invalidateQueries({ queryKey: ["monitor-agent-status", agent.id] });
    // 显示成功通知
    showToast("Agent started", "success", {
      description: `${agent.name} is now active`,
    });
  },
  onError: (error: Error) => {
    // 显示错误通知
    showToast("Failed to start agent", "error", {
      description: error.message || "Unable to start the monitoring agent",
    });
  },
});
```

### 自定义钩子模式

**带轮询和清理的钩子：**

```typescript
export const usePacketLossMonitorStatus = (
  monitors: PacketLossMonitor[],
  selectedMonitorId?: number,
) => {
  const queryClient = useQueryClient();
  // 使用 Map 存储监控器状态
  const [monitorStatuses, setMonitorStatuses] = useState<Map<number, MonitorStatus>>(new Map());

  useEffect(() => {
    // 过滤出已启用的监控器
    const enabledMonitors = monitors.filter((m) => m.enabled);
    if (enabledMonitors.length === 0) return;

    // 设置轮询定时器
    const pollInterval = setInterval(async () => {
      // 并行获取所有监控器状态
      const statusPromises = enabledMonitors.map(async (monitor) => {
        try {
          const status = await getPacketLossMonitorStatus(monitor.id);
          return { monitorId: monitor.id, status };
        } catch (error) {
          console.error(`Failed to get status for monitor ${monitor.id}:`, error);
          return null;
        }
      });

      const results = await Promise.all(statusPromises);
      
      // 更新状态并处理完成逻辑
      setMonitorStatuses((prev) => {
        const newStatuses = new Map(prev);
        results.forEach((result) => {
          if (result) newStatuses.set(result.monitorId, result.status);
        });
        return newStatuses;
      });
    }, 2000); // 每 2 秒轮询一次

    // 清理函数
    return () => clearInterval(pollInterval);
  }, [monitors, queryClient, selectedMonitorId]);

  return monitorStatuses;
};
```

**带有复杂选项和多个数据源的钩子：**

```typescript
interface UseMonitorAgentOptions {
  agent: MonitorAgent; // 监控代理
  includeNativeData?: boolean; // 是否包含原生数据
  includeSystemInfo?: boolean; // 是否包含系统信息
  includePeakStats?: boolean; // 是否包含峰值统计
  includeHardwareStats?: boolean; // 是否包含硬件统计
}

export const useMonitorAgent = ({
  agent,
  includeNativeData = false,
  includeSystemInfo = false,
  includePeakStats = false,
  includeHardwareStats = false,
}: UseMonitorAgentOptions) => {
  // 具有不同刷新速率和条件的多个查询
  // 返回带有加载状态的复杂数据结构
  return {
    status: statusQuery.data, // 状态数据
    nativeData: nativeDataQuery.data, // 原生数据
    systemInfo: systemInfoQuery.data, // 系统信息
    peakStats: peakStatsQuery.data, // 峰值统计
    hardwareStats: hardwareStatsQuery.data, // 硬件统计
    isLoadingStatus: statusQuery.isLoading, // 状态加载中
    isLoadingNativeData: nativeDataQuery.isLoading, // 原生数据加载中
    // ... 其他加载状态
    startMutation, // 启动监控的 mutation
    stopMutation, // 停止监控的 mutation
  };
};
```

---

## 可访问性

### ARIA 属性

**使用正确的 ARIA 标签：**

```typescript
<button
  aria-label="Share public speed test page" // 分享公开速度测试页面
  aria-pressed={isActive} // 是否按下状态
  role="tab" // 角色：标签页
  aria-selected={isActive} // 是否选中
>
```

**导航模式：**

```typescript
<nav role="tablist"> // 标签页列表导航
  <button role="tab" aria-selected={isActive} aria-pressed={isActive}>
    Tab Content // 标签页内容
  </button>
</nav>
```

### 键盘导航

**支持键盘交互：**

```typescript
<input
  onKeyDown={(e) => {
    if (e.key === "Enter" && !isLoading) { // 按下 Enter 键且不在加载中时执行操作
      handleAction();
    }
  }}
/>
```

### 焦点管理

**提供焦点指示器：**

```typescript
className =
  "focus:outline-none focus:ring-1 focus:ring-inset focus:ring-blue-500/50"; // 自定义焦点样式
```

---

## 性能优化

### 动画性能

**将配置移到组件外部：**

```typescript
// 好的做法 - 在组件外部定义
const SPRING_TRANSITION = {
  type: "spring" as const,
  stiffness: 500, // 刚度
  damping: 30, // 阻尼
} as const;

const Component = () => (
  <motion.div transition={SPRING_TRANSITION}>Content</motion.div>
);
```

### 记忆化模式

**使用 useMemo 处理昂贵的计算：**

```typescript
const filteredServers = useMemo(() => {
  return allServers.filter((server) => {
    const matchesSearch = 
      searchTerm === "" ||
      server.name.toLowerCase().includes(searchTerm.toLowerCase());
    return matchesSearch;
  });
}, [allServers, searchTerm]); // 依赖项数组
```

### 组件优化

**使用 React.FC 保持一致性：**

```typescript
export const Component: React.FC<Props> = ({ prop1, prop2 }) => {
  // 组件实现
};
```

---

## API 层模式

### 一致的错误处理

**在所有 API 函数中使用标准化的错误处理：**

```typescript
export async function getServers(testType: string) {
  try {
    const response = await fetch(getApiUrl(`/servers?testType=${testType}`));
    if (!response.ok) {
      const errorData = await response.json().catch(() => ({}));
      throw new Error(errorData.message || "Failed to fetch servers");
    }
    return await response.json();
  } catch (error) {
    console.error("Error fetching servers:", error);
    throw error;
  }
}
```

**带有正确头信息的 POST 请求：**

```typescript
export async function runSpeedTest(options: SpeedTestOptions) {
  try {
    const response = await fetch(getApiUrl("/speedtest"), {
      method: "POST",
      headers: {
        "Content-Type": "application/json", // JSON 内容类型
      },
      body: JSON.stringify(options), // 请求体
    });
    if (!response.ok) {
      const errorData = await response.json().catch(() => ({}));
      throw new Error(errorData.message || "Failed to run speed test");
    }
    return await response.json();
  } catch (error) {
    console.error("Error running speed test:", error);
    throw error;
  }
}
```

**实时数据的缓存控制：**

```typescript
export async function getSpeedTestStatus() {
  try {
    const response = await fetch(getApiUrl("/speedtest/status"), {
      headers: {
        "Cache-Control": "no-cache", // 禁用缓存
        Pragma: "no-cache", // HTTP/1.0 兼容
      },
    });
    // ... 错误处理
  } catch (error) {
    console.error("Error getting speed test status:", error);
    throw error;
  }
}
```

## 常量和配置模式

### 类型安全常量

**使用 `as const` 创建不可变配置：**

```typescript
export const MONITOR_REFRESH_INTERVALS = {
  STATUS: 5000, // 5 秒
  HARDWARE_STATS: 30000, // 30 秒
  NATIVE_DATA: 60000, // 1 分钟
  SYSTEM_INFO: 300000, // 5 分钟
} as const; // 使其不可变

// 从常量中提取类型
export type MonitorRefreshInterval = typeof MONITOR_REFRESH_INTERVALS[keyof typeof MONITOR_REFRESH_INTERVALS];
```

**表单选项的接口+数据模式：**

```typescript
export interface IntervalOption {
  value: string; // 值
  label: string; // 显示标签
}

export const intervalOptions: IntervalOption[] = [
  { value: "10s", label: "Every 10 seconds" },
  { value: "30s", label: "Every 30 seconds" },
  { value: "1m", label: "Every 1 minute" },
  { value: "5m", label: "Every 5 minutes" },
  // ... 更多选项
];

export const defaultFormData: MonitorFormData = {
  host: "", // 主机
  name: "", // 名称
  interval: "30m", // 间隔
  scheduleType: "interval", // 调度类型
  exactTimes: [], // 精确时间
  packetCount: 10, // 数据包数量
  threshold: 5.0, // 阈值
  enabled: true, // 是否启用
};
```

## 路由模式

### TanStack Router 配置

**带有身份验证的路由组合：**

```typescript
import { createRouter, createRoute, createRootRoute, Outlet } from "@tanstack/react-router";

// 受保护路由包装器
function ProtectedRoute() {
  const { isAuthenticated, isLoading } = useAuth();
  const navigate = router.navigate;

  useEffect(() => {
    if (!isLoading && !isAuthenticated) {
      navigate({ to: "/login" }); // 未认证时跳转到登录页
    }
  }, [isLoading, isAuthenticated, navigate]);

  if (isLoading) {
    return <LoadingSpinner />; // 加载中状态
  }

  return isAuthenticated ? <Outlet /> : null; // 认证后显示子路由内容
}

// 路由树结构
const rootRoute = createRootRoute({
  component: () => <App />, // 根组件
});

const protectedRoute = createRoute({
  getParentRoute: () => rootRoute,
  id: "protected",
  component: ProtectedRoute, // 受保护路由组件
});

const indexRoute = createRoute({
  getParentRoute: () => protectedRoute,
  path: "/", // 根路径
  component: Main, // 主页面组件
});

// 构建路由树
const routeTree = rootRoute.addChildren([
  protectedRoute.addChildren([indexRoute]), // 受保护路由的子路由
  authRoute.addChildren([loginRoute, registerRoute]), // 身份验证相关路由
]);

// 创建路由器
export const router = createRouter({
  routeTree,
  defaultPreload: "intent", // 预加载策略
  basepath: window.__BASE_URL__ || "/", // 基础路径
});
```

## 类型定义模式

### 复杂接口层次结构

**定义全面的类型结构：**

```typescript
export interface PacketLossMonitor {
  id: number; // 监控器 ID
  host: string; // 目标主机
  name?: string; // 名称（可选）
  interval: string; // 间隔时间字符串，非数字
  packetCount: number; // 数据包数量
  enabled: boolean; // 是否启用
  threshold: number; // 阈值
  lastRun?: string; // 上次运行时间（可选）
  nextRun?: string; // 下次运行时间（可选）
  createdAt: string; // 创建时间
  updatedAt: string; // 更新时间
}

export interface PacketLossResult {
  id: number; // 结果 ID
  monitorId: number; // 所属监控器 ID
  packetLoss: number; // 数据包丢失率
  minRtt: number; // 最小往返时间
  maxRtt: number; // 最大往返时间
  avgRtt: number; // 平均往返时间
  stdDevRtt: number; // 往返时间标准差
  packetsSent: number; // 发送的数据包数
  packetsRecv: number; // 接收的数据包数
  usedMtr?: boolean; // 是否使用 MTR
  hopCount?: number; // 跳数
  mtrData?: string; // 包含 MTRData 的 JSON 字符串
  privilegedMode?: boolean; // 是否使用特权模式
  createdAt: string;
}
```

**特定值的联合类型：**

```typescript
export type TimeRange = "1d" | "3d" | "1w" | "1m" | "all"; // 时间范围类型
export type TestType = "speedtest" | "iperf" | "librespeed"; // 测试类型
```

**API 响应的通用接口：**

```typescript
export interface PaginatedResponse<T> {
  data: T[]; // 数据数组
  page: number; // 当前页码
  limit: number; // 每页数量
  total?: number; // 总数（可选）
}
```

---

## 文件组织

### 目录结构

```
src/
├── components/      # 组件目录
│   ├── auth/           # 认证组件
│   ├── common/         # 共享组件
│   ├── monitor/        # 监控功能组件
│   ├── settings/       # 设置和配置组件
│   ├── speedtest/      # 网速测试功能（深度嵌套）：
│   │   ├── packetloss/ # 功能特定组织：
│   │   │   ├── components/    # 子功能组件
│   │   │   ├── hooks/         # 功能特定钩子
│   │   │   ├── types/         # 功能特定类型
│   │   │   ├── utils/         # 功能特定工具
│   │   │   └── constants/     # 功能特定常量
│   │   └── traceroute/ # 类似的深度结构
│   ├── icons/          # 自定义图标组件
│   └── ui/             # 基础 UI 组件 (shadcn/ui)
├── api/                # API 层函数
├── constants/          # 全局常量
├── context/            # React 上下文
├── hooks/              # 全局自定义钩子
├── lib/                # 库工具函数 (utils.ts)
├── types/              # 全局类型定义
├── utils/              # 全局工具函数
├── routes.tsx          # TanStack Router 配置
└── main.tsx            # 应用程序入口点
```

### 命名约定

**文件和组件：**

- 组件: `PascalCase.tsx`
- 工具函数: `camelCase.ts`
- 类型: `types.ts`
- 常量: `SCREAMING_SNAKE_CASE`（全大写蛇形命名）

**导入：**

```typescript
// 标准导入顺序
import React from "react";
import { motion } from "motion/react";
import { useQuery } from "@tanstack/react-query";

// 本地导入
import { Component } from "@/components/ui/Component";
import { utility } from "@/utils/utility";
import { Type } from "@/types/types";
```

### 许可证头部

**所有源文件必须包含：**

```typescript
/*
 * Copyright (c) 2024-2025, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */
```

---

## 快速参考

### 组件架构快捷方式

**CVA 按钮变体：**

```typescript
const buttonVariants = cva(
  "base-classes",
  {
    variants: {
      variant: { default: "classes", destructive: "classes" },
      size: { default: "classes", sm: "classes", lg: "classes" },
    },
    defaultVariants: { variant: "default", size: "default" },
  }
);

interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement>, VariantProps<typeof buttonVariants> {
  asChild?: boolean; // 是否使用子元素渲染
}

const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  ({ className, variant, size, asChild = false, ...props }, ref) => {
    const Comp = asChild ? Slot : "button"; // 如果是子元素则使用 Slot，否则使用 button
    return <Comp className={cn(buttonVariants({ variant, size, className }))} ref={ref} {...props} />;
  }
);
```

**基于状态的组件：**

```typescript
const statusColors = {
  normal: "", // 正常状态
  success: "ring-1 ring-emerald-500/20 bg-emerald-500/5", // 成功状态
  warning: "ring-1 ring-amber-500/20 bg-amber-500/5", // 警告状态
  error: "ring-1 ring-red-500/20 bg-red-500/5", // 错误状态
};
```

### TanStack Query 快捷方式

**复杂查询配置：**

```typescript
const query = useQuery({
  queryKey: ["key", id], // 查询键，用于缓存
  queryFn: () => fetchData(id), // 查询函数
  refetchInterval: enabled ? INTERVALS.STATUS : false, // 刷新间隔
  staleTime: INTERVALS.STATUS / 2, // 数据过期时间
  gcTime: 5 * 60 * 1000, // 垃圾回收时间
  enabled: enabled && hasData, // 是否启用查询
});
```

**缓存操作：**

```typescript
// 直接更新 + 失效模式
queryClient.setQueryData(["key", id], newData); // 设置缓存数据
queryClient.invalidateQueries({ queryKey: ["key", id], exact: true }); // 使查询失效
```

### API 模式快捷方式

**标准 API 函数：**

```typescript
export async function apiFunction(param: string) {
  try {
    const response = await fetch(getApiUrl(`/endpoint?param=${param}`)); // 发起请求
    if (!response.ok) {
      const errorData = await response.json().catch(() => ({})); // 解析错误数据
      throw new Error(errorData.message || "获取数据失败"); // 抛出错误
    }
    return await response.json(); // 返回成功数据
  } catch (error) {
    console.error("错误:", error); // 打印错误
    throw error; // 重新抛出错误
  }
}
```

### 常量模式快捷方式

**类型安全常量：**

```typescript
export const CONFIG = {
  VALUE1: 1000, // 配置值1
  VALUE2: 2000, // 配置值2
} as const; // 确保类型安全

export type ConfigValue = typeof CONFIG[keyof typeof CONFIG]; // 从常量中提取类型
```

### 常用类组合

**卡片/面板：**

```typescript
className =
  "bg-gray-50/95 dark:bg-gray-850/95 rounded-xl p-6 shadow-lg border border-gray-200 dark:border-gray-800";
```

**按钮（主要）：**

```typescript
className =
  "bg-blue-500 text-white hover:bg-blue-600 dark:bg-blue-600 dark:hover:bg-blue-700 shadow-lg";
```

**按钮（轮廓）：**

```typescript
className =
  "border border-gray-300 bg-white text-gray-900 hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-900 dark:text-gray-100 dark:hover:bg-gray-800 shadow-lg";
```

**按钮（次要）：**

```typescript
className =
  "bg-gray-200/50 text-gray-900 hover:bg-gray-300/50 dark:bg-gray-800/50 dark:text-gray-100 dark:hover:bg-gray-800 border border-gray-300 dark:border-gray-800 shadow-lg";
```

**输入框：**

```typescript
className =
  "px-4 py-2 bg-gray-200/50 dark:bg-gray-800/50 border border-gray-300 dark:border-gray-900 text-gray-700 dark:text-gray-300 rounded-lg shadow-md focus:outline-none focus:ring-1 focus:ring-inset focus:ring-blue-500/50";
```

**状态徽章：**

```typescript
className =
  "inline-flex items-center px-2 py-1 rounded-full text-xs font-medium bg-blue-500/10 text-blue-600 dark:text-blue-400";
```

### 动画快捷方式

**标准入场：**

```typescript
initial={{ opacity: 0, y: 20 }} // 初始状态：透明，向下偏移20px
animate={{ opacity: 1, y: 0 }} // 动画后状态：不透明，位置还原
```

**按钮交互：**

```typescript
whileHover={{ scale: 1.02 }} // 悬停时缩放1.02倍
whileTap={{ scale: 0.98 }} // 点击时缩放0.98倍
```

**布局过渡：**

```typescript
layoutId="uniqueId" // 唯一布局ID，用于平滑过渡
transition={{ type: "spring", stiffness: 500, damping: 30 }} // 过渡参数
```

### 颜色快速参考

#### 主要 -400 颜色调色板

| 颜色         | 十六进制代码 | Tailwind 类名 | 使用场景                 |
|------------|----------|------------|----------------------|
| Blue-400   | `#60a5fa` | `blue-400` | 下载指标，主要颜色            |
| Emerald-400 | `#34d399` | `emerald-400` | 上传指标，成功状态            |
| Amber-400  | `#fbbf24` | `amber-400` | 延迟指标，警告状态            |
| Purple-400 | `#c084fc` | `purple-400` | 抖动指标，特殊状态            |

#### 语义颜色使用

| 用途          | 浅色模式              | 深色模式              |
|-------------|-------------------|-------------------|
| Primary Text | `text-gray-900`    | `text-white`       |
| Secondary Text | `text-gray-700`    | `text-gray-300`    |
| Muted Text  | `text-gray-600`    | `text-gray-400`    |
| Primary Action | `text-blue-600`    | `text-blue-400`    |
| Success     | `text-emerald-600` | `text-emerald-400` |
| Warning     | `text-amber-600`   | `text-amber-400`   |
| Error       | `text-red-600`     | `text-red-400`     |
| Special     | `text-purple-600`  | `text-purple-400`  |

---

## 最近更新

本样式指南已根据当前代码库模式的分析进行了全面更新。主要新增了以下内容：

### 新增章节：
- **组件变体架构 (CVA)**: 使用 `class-variance-authority` 的现代变体管理
- **Radix UI 集成模式**: 复合组件和 `asChild` 属性模式
- **高级 TanStack Query 模式**: 复杂缓存策略、条件查询和缓存操作
- **自定义钩子模式**: 轮询、清理和复杂数据管理
- **API 层模式**: 一致的错误处理和请求模式
- **常量和配置模式**: 使用 `as const` 的类型安全配置
- **路由模式**: TanStack Router 组合和认证
- **类型定义模式**: 复杂接口和类型提取

### 更新章节：
- **目录结构**: 添加了代码库中使用的实际深度嵌套模式
- **组件组合**: 添加了基于状态的样式模式
- **快速参考**: 添加了新模式的快捷方式

### 最新更新：按钮组件模式（2025年1月）

- **更新了 CVA 按钮示例**: 现在展示完整的变体定义，包含适当的深色模式支持
- **添加了按钮（轮廓）模式**: 记录了带有显式文本颜色的完整轮廓变体
- **增强了深色模式指南**: 增加了对可访问性的显式文本颜色的强调
- **修复了快速参考**: 更新了按钮快捷方式以匹配实际实现模式

**关键见解**: 实际的 Button 组件实现比样式指南示例更全面，这表明保持文档与生产代码一致的重要性。

随着新模式的出现和代码库的发展，应更新本样式指南。添加新组件或模式时，请确保它们遵循这些已建立的约定，以保持应用程序的一致性。
