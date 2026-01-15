# Claude 快速入门

在本示例中，我们将让 Claude 编写一个检查字符串是否为回文的 Python 函数。

## 先决条件

您需要：

- 一个 Anthropic [控制台账户](https://console.anthropic.com/)
- 一个 [API 密钥](https://console.anthropic.com/settings/keys)
- Python 3.7+ 或 TypeScript 4.5+

Anthropic 提供 [Python 和 TypeScript SDK](https://docs.anthropic.com/en/api/client-sdks)，您也可以直接向 API 发送 HTTP 请求。

## 从 Workbench 开始

无论您执行什么特定任务，任何 API 调用都会向 Anthropic API 发送一个配置良好的提示。在学习如何充分利用 Claude 时，我们建议您在 Workbench 中开始开发过程，Workbench 是 Claude 的基于 Web 的界面。

登录 [Anthropic 控制台](https://console.anthropic.com/) 并点击 **Write a prompt from scratch**（从头开始编写提示）。

在中间部分的 User（用户）下，让我们向 Claude 提问。

用户

```text
为什么海洋是咸的？
```

点击 **Run**（运行）。在右侧，您将看到类似以下的输出

响应

```text
海洋是咸的有几个因素：

1. 岩石风化：数百万年来，雨水、河流和溪流侵蚀了含有矿物盐的岩石。这些盐通过水流被带入海洋。
2. 火山活动：海底火山和热液喷口将包括盐在内的矿物质释放到海水中。
3. 大气沉降：来自海洋喷雾的盐粒子可以被风吹走并重新沉积到海洋中。
4. 蒸发：当水从海洋表面蒸发时，会留下溶解的盐，增加了剩余水中的盐浓度。
5. 生物过程：一些海洋生物通过释放盐化合物作为新陈代谢的副产物，为海洋的盐度做出贡献。

随着时间的推移，这些过程不断向海洋中添加盐分，而蒸发则去除淡水，导致海洋目前的盐度水平。重要的是要注意，海洋中的盐总量相对稳定，因为盐的输入与通过蒸发岩沉积等过程去除的盐相平衡。
```

这是一个很好的答案，但假设我们想要控制 Claude 给出的确切答案类型。例如，只允许 Claude 用诗歌回答问题。我们可以通过添加系统提示（System Prompt）来控制响应的格式、语气和个性。

系统提示

```text
您是一位世界级诗人。仅用短诗回答。
```

再次点击 **Run**（运行）。

响应

```text
海洋的咸水，
时光与元素的故事交织。
岩石与雨水，缓慢侵蚀，
矿物质溶于水中。
亿万年过去，盐依然存在，
在广阔永恒的水域中。
```

看到 Claude 的响应如何变化了吗？LLM（大型语言模型）对清晰直接的指令反应良好。您可以将角色指令放在系统提示或用户消息中。我们建议测试哪种方式能为您的用例产生最佳结果。

一旦您调整了输入，对输出感到满意，并且对如何使用 Claude 有了良好的理解，就可以将您的 Workbench 转换为集成。

点击 **Get Code**（获取代码）以复制代表您的 Workbench 会话的生成代码。

## 安装 SDK

Anthropic 提供适用于 [Python](https://pypi.org/project/anthropic/) (3.7+)、[TypeScript](https://www.npmjs.com/package/@anthropic-ai/sdk) (4.5+) 和 [Java](https://central.sonatype.com/artifact/com.anthropic/anthropic-java/) (8+) 的 SDK。我们目前还有一个 [Go](https://pkg.go.dev/github.com/anthropics/anthropic-sdk-go) SDK 处于测试阶段。

### Python

在您的项目目录中，创建一个虚拟环境。

```bash
python -m venv claude-env  # 创建虚拟环境
```

激活虚拟环境：

- 在 macOS 或 Linux 上：`source claude-env/bin/activate`
- 在 Windows 上：`claude-env\Scripts\activate`

```bash
pip install anthropic  # 安装 Anthropic SDK
```

### TypeScript

安装 SDK。

```bash
npm install @anthropic-ai/sdk  # 安装 TypeScript SDK
```

### Java

首先在 [Maven Central](https://central.sonatype.com/artifact/com.anthropic/anthropic-java) 上找到 Java SDK 的当前版本。
在您的 Gradle 文件中声明 SDK 作为依赖项：

```gradle
implementation("com.anthropic:anthropic-java:1.0.0")  // 添加 Java SDK 依赖
```

或者在您的 Maven 文件中：

```xml
<dependency>  <!-- Anthropic Java SDK 依赖 -->
  <groupId>com.anthropic</groupId>
  <artifactId>anthropic-java</artifactId>
  <version>1.0.0</version>
</dependency>
```

## 设置您的 API 密钥

每个 API 调用都需要一个有效的 API 密钥。SDK 设计为从环境变量 `ANTHROPIC_API_KEY` 中获取 API 密钥。您也可以在初始化 Anthropic 客户端时提供密钥。

### macOS 和 Linux

```bash
export ANTHROPIC_API_KEY='your-api-key-here'  # 设置 API 密钥环境变量
```

## 调用 API

通过向 [/messages](https://docs.anthropic.com/en/api/messages) 端点传递适当的参数来调用 API。

请注意，Workbench 提供的代码在构造函数中设置了 API 密钥。如果您将 API 密钥设置为环境变量，可以省略该行，如下所示。

### Python

```python
import anthropic  # 导入 anthropic 库

client = anthropic.Anthropic()  # 创建 Anthropic 客户端

message = client.messages.create(  # 创建消息请求
    model="claude-opus-4-20250514",  # 使用的模型名称
    max_tokens=1000,  # 生成的最大令牌数
    temperature=1,  # 温度参数，控制输出的随机性
    system="You are a world-class poet. Respond only with short poems.",  # 系统提示
    messages=[  # 消息列表
        {
            "role": "user",  # 角色：用户
            "content": [  # 内容列表
                {
                    "type": "text",  # 内容类型：文本
                    "text": "Why is the ocean salty?"  # 文本内容
                }
            ]
        }
    ]
)
print(message.content)  # 打印响应内容
```

使用 `python3 claude_quickstart.py` 或 `node claude_quickstart.js` 运行代码。

输出（Python）

```python
[TextBlock(text="The ocean's salty brine,
A tale of time and design.
Rocks and rivers, their minerals shed,
Accumulating in the ocean's bed.
Evaporation leaves salt behind,
In the vast waters, forever enshrined.", type='text')]
```

Workbench 和代码示例使用默认的模型设置：model（名称）、temperature（温度）和 max tokens（最大令牌数）。

本快速入门展示了如何使用控制台、Workbench 和 API 开发一个基本但功能完整的 Claude 驱动的应用程序。您可以使用相同的工作流作为更强大用例的基础。

## 下一步

既然您已经发出了第一个 Anthropic API 请求，是时候探索其他可能性了：

- **使用案例指南** - 常见用例的端到端实现指南。
- **Anthropic 食谱** - 通过交互式 Jupyter 笔记本学习，这些笔记本展示了上传 PDF、嵌入等功能。
- **提示库** - 探索数十个示例提示，为各种用例提供灵感。