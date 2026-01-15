# 使用 UV 运行脚本

Python 脚本是一种用于独立执行的文件，例如使用 `python <script>.py` 运行。使用 uv 执行脚本可以确保脚本依赖项得到管理，而无需手动管理环境。

## 运行无依赖脚本

如果您的脚本没有依赖项，可以使用 `uv run` 执行它：

```python
# example.py
print("Hello world")  # 打印 "Hello world"
```

```bash
$ uv run example.py
Hello world
```

同样，如果您的脚本依赖于标准库中的模块，也无需额外操作。

可以向脚本提供参数：

```python
# example.py
import sys  # 导入 sys 模块以访问命令行参数
print(" ".join(sys.argv[1:]))  # 打印除脚本名外的所有参数
```

```bash
$ uv run example.py test
test

$ uv run example.py hello world!
hello world!
```

此外，您还可以直接从标准输入读取脚本内容。

请注意，如果您在一个项目（即包含 `pyproject.toml` 的目录）中使用 `uv run`，它会在运行脚本之前安装当前项目。如果您的脚本不依赖于该项目，请使用 `--no-project` 标志跳过此步骤：

```bash
$ # 注意：`--no-project` 标志必须在脚本名称之前提供。
$ uv run --no-project example.py
```

## 运行有依赖脚本

当您的脚本需要其他包时，必须将它们安装到脚本运行的环境中。使用 `--with` 选项来指定依赖项：

```bash
$ uv run --with rich example.py  # 运行 example.py 并安装 rich 依赖
```

如果需要特定版本，可以为请求的依赖项添加约束：

```bash
$ uv run --with 'rich>12,<13' example.py  # 指定 rich 版本在 12 到 13 之间
```

可以通过重复使用 `--with` 选项来请求多个依赖项。

## 创建 Python 脚本

Python 最近添加了一种内联脚本元数据的标准格式。它允许选择 Python 版本并定义依赖项。使用 `uv init --script` 来初始化带有内联元数据的脚本：

```bash
$ uv init --script example.py --python 3.12  # 创建一个使用 Python 3.12 的脚本
```

## 声明脚本依赖

内联元数据格式允许在脚本本身中声明脚本的依赖项。使用 `uv add --script` 来为脚本声明依赖项：

```bash
$ uv add --script example.py 'requests<3' 'rich'  # 为 example.py 添加 requests<3 和 rich 依赖
```

这将在脚本顶部添加一个 `script` 部分，使用 TOML 格式声明依赖项：

```python
# /// script
# dependencies = [\
#   "requests<3",\
#   "rich",\
# ]
# ///

import requests  # 导入 requests 库用于 HTTP 请求
from rich.pretty import pprint  # 从 rich 库导入 pprint 用于美观打印

resp = requests.get("https://peps.python.org/api/peps.json")  # 发送 GET 请求获取 PEPs 数据
data = resp.json()  # 将响应转换为 JSON 格式
pprint([(k, v["title"]) for k, v in data.items()][:10])  # 打印前 10 个 PEP 的编号和标题
```

uv 会自动创建一个包含运行脚本所需依赖项的环境。

## 使用 shebang 创建可执行文件

可以添加 shebang 使脚本无需使用 `uv run` 即可执行：

```python
#!/usr/bin/env -S uv run --script  # 使用 uv run --script 执行此脚本

print("Hello, world!")  # 打印 "Hello, world!"
```

确保您的脚本是可执行的，例如使用 `chmod +x greet`，然后运行脚本。

## 使用替代包索引

如果您希望使用替代包索引来解析依赖项，可以使用 `--index` 选项提供索引：

```bash
$ uv add --index "https://example.com/simple" --script example.py 'requests<3' 'rich'  # 使用自定义包索引
```

## 锁定依赖

uv 支持使用 `uv.lock` 文件格式为 PEP 723 脚本锁定依赖项：

```bash
$ uv lock --script example.py  # 锁定 example.py 的依赖项
```

运行 `uv lock --script` 将在脚本旁边创建一个 `.lock` 文件（例如 `example.py.lock`）。

## 提高可重复性

除了锁定依赖项外，uv 还支持在内联脚本元数据的 `tool.uv` 部分使用 `exclude-newer` 字段，以限制 uv 只考虑在特定日期之前发布的发行版：

```python
# /// script
# dependencies = [\
#   "requests",\
# ]
# [tool.uv]
# exclude-newer = "2023-10-16T00:00:00Z"  # 只使用 2023-10-16 之前发布的依赖版本
# ///
```

## 使用不同的 Python 版本

uv 允许在每次脚本调用时请求任意 Python 版本：

```bash
$ # 使用特定的 Python 版本
$ uv run --python 3.10 example.py  # 使用 Python 3.10 运行脚本
```

## 使用 GUI 脚本

在 Windows 上，`uv` 会使用 `pythonw` 运行扩展名为 `.pyw` 的脚本（无控制台窗口）。