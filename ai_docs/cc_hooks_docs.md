# 钩子（Hooks）

> 通过注册 shell 命令来自定义和扩展 Claude Code 的行为

# 介绍

Claude Code 钩子是用户定义的 shell 命令，在 Claude Code 生命周期的各个点执行。钩子提供了对 Claude Code 行为的确定性控制，确保某些操作总是发生，而不是依赖于 LLM 选择执行它们。

示例用例包括：

- **通知**：自定义当 Claude Code 等待您的输入或执行权限时如何收到通知。
- **自动格式化**：在每次文件编辑后，对 .ts 文件运行 `prettier`，对 .go 文件运行 `gofmt` 等。
- **日志记录**：跟踪和计数所有执行的命令，用于合规或调试。
- **反馈**：当 Claude Code 生成的代码不符合您的代码库约定时，提供自动反馈。
- **自定义权限**：阻止对生产文件或敏感目录的修改。

通过将这些规则编码为钩子而不是提示指令，您可以将建议转变为应用级代码，每次预期运行时都会执行。

<Warning>
  钩子会使用您的完整用户权限执行 shell 命令，无需确认。您有责任确保您的钩子是安全的。Anthropic 不对钩子使用导致的数据丢失或系统损坏承担任何责任。请查看[安全考虑](#安全考虑)。
</Warning>

## 快速入门

在这个快速入门中，您将添加一个钩子，用于记录 Claude Code 运行的 shell 命令。

快速入门先决条件：安装 `jq` 用于命令行中的 JSON 处理。

### 步骤 1：打开钩子配置

运行 `/hooks` [斜杠命令](/en/docs/claude-code/slash-commands)并选择 `PreToolUse` 钩子事件。

`PreToolUse` 钩子在工具调用前运行，可以阻止它们，同时向 Claude 提供关于如何以不同方式执行的反馈。

### 步骤 2：添加匹配器

选择 `+ Add new matcher…`（添加新匹配器）仅在 Bash 工具调用时运行您的钩子。

为匹配器输入 `Bash`。

### 步骤 3：添加钩子

选择 `+ Add new hook…`（添加新钩子）并输入以下命令：

```bash
jq -r '"\(.tool_input.command) - \(.tool_input.description // "No description")"' >> ~/.claude/bash-command-log.txt  # 记录 Bash 命令到日志文件
```

### 步骤 4：保存配置

对于存储位置，选择 `User settings`（用户设置），因为您要记录到主目录。然后此钩子将应用于所有项目，而不仅仅是当前项目。

然后按 Esc 直到返回 REPL。您的钩子现已注册！

### 步骤 5：验证钩子

再次运行 `/hooks` 或检查 `~/.claude/settings.json` 以查看您的配置：

```json
"hooks": {
  "PreToolUse": [
    {
      "matcher": "Bash",  // 仅匹配 Bash 工具
      "hooks": [
        {
          "type": "command",  // 命令类型
          "command": "jq -r '\"\\(.tool_input.command) - \\(.tool_input.description // \"No description\")\"' >> ~/.claude/bash-command-log.txt"  // 要执行的命令
        }
      ]
    }
  ]
}
```

## 配置

Claude Code 钩子在您的[设置文件](/en/docs/claude-code/settings)中配置：

- `~/.claude/settings.json` - 用户设置
- `.claude/settings.json` - 项目设置
- `.claude/settings.local.json` - 本地项目设置（不提交）
- 企业管理的策略设置

### 结构

钩子按匹配器组织，每个匹配器可以有多个钩子：

```json
{
  "hooks": {
    "EventName": [  // 钩子事件名称
      {
        "matcher": "ToolPattern",  // 工具匹配模式
        "hooks": [  // 匹配时要执行的命令数组
          {
            "type": "command",  // 命令类型
            "command": "your-command-here"  // 要执行的 bash 命令
          }
        ]
      }
    ]
  }
}
```

- **matcher**：匹配工具名称的模式（仅适用于 `PreToolUse` 和 `PostToolUse`）
  - 简单字符串精确匹配：`Write` 仅匹配 Write 工具
  - 支持正则表达式：`Edit|Write` 或 `Notebook.*`
  - 如果省略或为空字符串，钩子将为所有匹配事件运行
- **hooks**：匹配模式时要执行的命令数组
  - `type`：目前仅支持 `"command"`
  - `command`：要执行的 bash 命令
  - `timeout`：（可选）命令应运行多长时间（以秒为单位），然后取消所有正在进行的钩子。

## 钩子事件

### PreToolUse

在 Claude 创建工具参数后、处理工具调用前运行。

**常见匹配器：**

- `Task` - 代理任务
- `Bash` - Shell 命令
- `Glob` - 文件模式匹配
- `Grep` - 内容搜索
- `Read` - 文件读取
- `Edit`, `MultiEdit` - 文件编辑
- `Write` - 文件写入
- `WebFetch`, `WebSearch` - Web 操作

### PostToolUse

在工具成功完成后立即运行。

识别与 PreToolUse 相同的匹配器值。

### Notification

当 Claude Code 发送通知时运行。

### Stop

当主 Claude Code 代理完成响应时运行。

### SubagentStop

当 Claude Code 子代理（Task 工具调用）完成响应时运行。

## 钩子输入

钩子通过 stdin 接收包含会话信息和事件特定数据的 JSON 数据：

```typescript
{
  // 公共字段
  session_id: string  // 会话 ID
  transcript_path: string  // 对话 JSON 的路径

  // 事件特定字段
  ...
}
```

### PreToolUse 输入

`tool_input` 的精确模式取决于工具。

```json
{
  "session_id": "abc123",  // 会话 ID
  "transcript_path": "~/.claude/projects/.../00893aaf-19fa-41d2-8238-13269b9b3ca0.jsonl",  // 对话记录路径
  "tool_name": "Write",  // 工具名称
  "tool_input": {
    "file_path": "/path/to/file.txt",  // 文件路径
    "content": "file content"  // 文件内容
  }
}
```

### PostToolUse 输入

`tool_input` 和 `tool_response` 的精确模式取决于工具。

```json
{
  "session_id": "abc123",  // 会话 ID
  "transcript_path": "~/.claude/projects/.../00893aaf-19fa-41d2-8238-13269b9b3ca0.jsonl",  // 对话记录路径
  "tool_name": "Write",  // 工具名称
  "tool_input": {
    "file_path": "/path/to/file.txt",  // 输入的文件路径
    "content": "file content"  // 输入的文件内容
  },
  "tool_response": {
    "filePath": "/path/to/file.txt",  // 响应的文件路径
    "success": true  // 是否成功
  }
}
```

### Notification 输入

```json
{
  "session_id": "abc123",  // 会话 ID
  "transcript_path": "~/.claude/projects/.../00893aaf-19fa-41d2-8238-13269b9b3ca0.jsonl",  // 对话记录路径
  "message": "Task completed successfully",  // 通知消息
  "title": "Claude Code"  // 通知标题
}
```

### Stop 和 SubagentStop 输入

当 Claude Code 已经因 stop 钩子而继续运行时，`stop_hook_active` 为 true。检查此值或处理对话记录以防止 Claude Code 无限运行。

```json
{
  "session_id": "abc123",  // 会话 ID
  "transcript_path": "~/.claude/projects/.../00893aaf-19fa-41d2-8238-13269b9b3ca0.jsonl",  // 对话记录路径
  "stop_hook_active": true  // 是否已激活 stop 钩子
}
```

## 钩子输出

钩子有两种方式将输出返回给 Claude Code。输出传达是否要阻止以及应向 Claude 和用户显示的任何反馈。

### 简单方式：退出代码

钩子通过退出代码、stdout 和 stderr 传达状态：

- **退出代码 0**：成功。`stdout` 在 transcript 模式（CTRL-R）中显示给用户。
- **退出代码 2**：阻塞错误。`stderr` 被反馈给 Claude 自动处理。请参阅下面的每个钩子事件行为。
- **其他退出代码**：非阻塞错误。`stderr` 显示给用户，执行继续。

<Warning>
  提醒：如果退出代码为 0，Claude Code 不会看到 stdout。
</Warning>

#### 退出代码 2 行为

| 钩子事件     | 行为                                        |
| -------------- | ----------------------------------------------- |
| `PreToolUse`   | 阻止工具调用，向 Claude 显示错误     |
| `PostToolUse`  | 向 Claude 显示错误（工具已运行）        |
| `Notification` | N/A，仅向用户显示 stderr                  |
| `Stop`         | 阻止停止，向 Claude 显示错误          |
| `SubagentStop` | 阻止停止，向 Claude 子代理显示错误 |

### 高级方式：JSON 输出

钩子可以在 `stdout` 中返回结构化 JSON 以实现更复杂的控制：

#### 公共 JSON 字段

所有钩子类型都可以包含这些可选字段：

```json
{
  "continue": true,  // Claude 是否应该在钩子执行后继续（默认：true）
  "stopReason": "string"  // 当 continue 为 false 时显示的消息
  "suppressOutput": true,  // 隐藏 transcript 模式中的 stdout（默认：false）
}
```

如果 `continue` 为 false，Claude 在钩子运行后停止处理。

- 对于 `PreToolUse`，这与 `"decision": "block"` 不同，后者仅阻止特定工具调用并向 Claude 提供自动反馈。
- 对于 `PostToolUse`，这与 `"decision": "block"` 不同，后者向 Claude 提供自动反馈。
- 对于 `Stop` 和 `SubagentStop`，这优先于任何 `"decision": "block"` 输出。
- 在所有情况下，`"continue" = false` 优先于任何 `"decision": "block"` 输出。

`stopReason` 与 `continue` 一起提供向用户显示的原因，不显示给 Claude。

#### `PreToolUse` 决策控制

`PreToolUse` 钩子可以控制工具调用是否继续。

- "approve" 绕过权限系统。`reason` 显示给用户，但不显示给 Claude。
- "block" 阻止工具调用执行。`reason` 显示给 Claude。
- `undefined` 遵循现有的权限流程。`reason` 被忽略。

```json
{
  "decision": "approve" | "block" | undefined,  // 决策类型
  "reason": "Explanation for decision"  // 决策原因
}
```

#### `PostToolUse` 决策控制

`PostToolUse` 钩子可以控制工具调用是否继续。

- "block" 自动向 Claude 提示 `reason`。
- `undefined` 不执行任何操作。`reason` 被忽略。

```json
{
  "decision": "block" | undefined,  // 决策类型
  "reason": "Explanation for decision"  // 决策原因
}
```

#### `Stop`/`SubagentStop` 决策控制

`Stop` 和 `SubagentStop` 钩子可以控制 Claude 是否必须继续。

- "block" 阻止 Claude 停止。您必须提供 `reason` 以便 Claude 知道如何继续。
- `undefined` 允许 Claude 停止。`reason` 被忽略。

```json
{
  "decision": "block" | undefined,  // 决策类型
  "reason": "Must be provided when Claude is blocked from stopping"  // 必须提供原因
}
```

#### JSON 输出示例：Bash 命令编辑

```python
#!/usr/bin/env python3  # 脚本解释器
import json  # 导入 json 模块
import re  # 导入正则表达式模块
import sys  # 导入系统模块

# 定义验证规则，作为 (正则表达式模式, 消息) 元组的列表
VALIDATION_RULES = [
    (
        r"\\bgrep\\b(?!.*\\|)",  # 匹配不包含管道的 grep
        "Use 'rg' (ripgrep) instead of 'grep' for better performance and features",  # 使用 rg 替代 grep
    ),
    (
        r"\\bfind\\s+\\S+\\s+-name\\b",  # 匹配 find 命令
        "Use 'rg --files | rg pattern' or 'rg --files -g pattern' instead of 'find -name' for better performance",  # 使用 rg 替代 find -name
    ),
]


def validate_command(command: str) -> list[str]:  # 验证命令的函数
    issues = []  # 问题列表
    for pattern, message in VALIDATION_RULES:  # 遍历所有验证规则
        if re.search(pattern, command):  # 如果命令匹配模式
            issues.append(message)  # 添加问题消息
    return issues  # 返回问题列表


try:
    input_data = json.load(sys.stdin)  # 从标准输入加载 JSON 数据
except json.JSONDecodeError as e:  # 捕获 JSON 解码错误
    print(f"Error: Invalid JSON input: {e}", file=sys.stderr)  # 打印错误信息
    sys.exit(1)  # 退出并返回错误代码

tool_name = input_data.get("tool_name", "")  # 获取工具名称
tool_input = input_data.get("tool_input", {})  # 获取工具输入
command = tool_input.get("command", "")  # 获取命令

if tool_name != "Bash" or not command:  # 如果不是 Bash 工具或命令为空
    sys.exit(1)  # 退出并返回错误代码

# 验证命令
issues = validate_command(command)  # 验证命令

if issues:  # 如果有问题
    for message in issues:  # 遍历所有问题
        print(f"• {message}", file=sys.stderr)  # 打印问题消息
    # 退出代码 2 阻止工具调用并向 Claude 显示 stderr
    sys.exit(2)  # 退出并返回错误代码 2
```

## 与 MCP 工具一起使用

Claude Code 钩子与[模型上下文协议 (MCP) 工具](/en/docs/claude-code/mcp)无缝协作。当 MCP 服务器提供工具时，它们会以特殊的命名模式出现，您可以在钩子中匹配这些模式。

### MCP 工具命名

MCP 工具遵循 `mcp__<server>__<tool>` 模式，例如：

- `mcp__memory__create_entities` - 内存服务器的创建实体工具
- `mcp__filesystem__read_file` - 文件系统服务器的读取文件工具
- `mcp__github__search_repositories` - GitHub 服务器的搜索工具

### 为 MCP 工具配置钩子

您可以针对特定的 MCP 工具或整个 MCP 服务器：

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "mcp__memory__.*",  // 匹配内存服务器的所有工具
        "hooks": [
          {
            "type": "command",  // 命令类型
            "command": "echo 'Memory operation initiated' >> ~/mcp-operations.log"  // 记录内存操作
          }
        ]
      },
      {
        "matcher": "mcp__.*__write.*",  // 匹配所有服务器的写操作工具
        "hooks": [
          {
            "type": "command",  // 命令类型
            "command": "/home/user/scripts/validate-mcp-write.py"  // 验证写操作
          }
        ]
      }
    ]
  }
}
```

## 示例

### 代码格式化

文件修改后自动格式化代码：

```json
{
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "Write|Edit|MultiEdit",  // 匹配文件写入和编辑工具
        "hooks": [
          {
            "type": "command",  // 命令类型
            "command": "/home/user/scripts/format-code.sh"  // 执行格式化脚本
          }
        ]
      }
    ]
  }
}
```

### 通知

自定义 Claude Code 请求权限或提示输入空闲时发送的通知。

```json
{
  "hooks": {
    "Notification": [
      {
        "matcher": "",  // 匹配所有通知
        "hooks": [
          {
            "type": "command",  // 命令类型
            "command": "python3 ~/my_custom_notifier.py"  // 执行自定义通知脚本
          }
        ]
      }
    ]
  }
}
```

## 安全考虑

### 免责声明

**风险自担**：Claude Code 钩子会在您的系统上自动执行任意 shell 命令。使用钩子即表示您确认：

- 您对配置的命令负有全部责任
- 钩子可以修改、删除或访问您的用户账户可以访问的任何文件
- 恶意或编写不当的钩子可能导致数据丢失或系统损坏
- Anthropic 不对钩子使用导致的任何损害提供任何保证或承担任何责任
- 您应该在安全环境中彻底测试钩子，然后再在生产环境中使用

在将任何钩子命令添加到您的配置之前，请始终查看并理解它们。

### 安全最佳实践

以下是编写更安全钩子的一些关键实践：

1. **验证和清理输入** - 永远不要盲目信任输入数据
2. **始终引用 shell 变量** - 使用 `"$VAR"` 而不是 `$VAR`
3. **阻止路径遍历** - 检查文件路径中的 `..`
4. **使用绝对路径** - 为脚本指定完整路径
5. **跳过敏感文件** - 避免 `.env`、`.git/`、密钥等

### 配置安全

对设置文件中的钩子进行直接编辑不会立即生效。Claude Code：

1. 在启动时捕获钩子的快照
2. 在整个会话中使用此快照
3. 如果钩子被外部修改，会发出警告
4. 需要在 `/hooks` 菜单中审核才能使更改生效

这可以防止恶意钩子修改影响您当前的会话。

## 钩子执行详细信息

- **超时**：默认 60 秒执行限制，可按命令配置。
  - 如果任何单个命令超时，所有正在进行的钩子都会被取消。
- **并行化**：所有匹配的钩子并行运行
- **环境**：在当前目录中使用 Claude Code 的环境运行
- **输入**：通过 stdin 输入 JSON
- **输出**：
  - PreToolUse/PostToolUse/Stop：进度显示在 transcript（Ctrl-R）中
  - Notification：仅记录到调试日志（`--debug`）

## 调试

要排查钩子问题：

1. 检查 `/hooks` 菜单是否显示您的配置
2. 验证您的[设置文件](/en/docs/claude-code/settings)是有效的 JSON
3. 手动测试命令
4. 检查退出代码
5. 查看 stdout 和 stderr 格式期望
6. 确保正确引用转义
7. 使用 `claude --debug` 调试您的钩子。成功钩子的输出如下所示。

```
[DEBUG] Executing hooks for PostToolUse:Write  # 执行 PostToolUse:Write 钩子
[DEBUG] Getting matching hook commands for PostToolUse with query: Write  # 获取匹配的钩子命令
[DEBUG] Found 1 hook matchers in settings  # 在设置中找到 1 个钩子匹配器
[DEBUG] Matched 1 hooks for query "Write"  # 为查询 "Write" 匹配了 1 个钩子
[DEBUG] Found 1 hook commands to execute  # 找到 1 个要执行的钩子命令
[DEBUG] Executing hook command: <Your command> with timeout 60000ms  # 执行钩子命令，超时 60000ms
[DEBUG] Hook command completed with status 0: <Your stdout>  # 钩子命令完成，状态为 0
```

进度消息显示在 transcript 模式（Ctrl-R）中，显示：

- 正在运行哪个钩子
- 正在执行的命令
- 成功/失败状态
- 输出或错误消息