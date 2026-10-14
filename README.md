# exq

`exq` 是一个命令行 Excel 查询工具，把 Excel 当数据库用：**列 = 字段（A、B、C...ZZ），行 = 记录**。支持 CEL 表达式筛选、单元格取值、工作表列举，以及 MCP server 模式供 AI 调用。

## 安装

```bash
go build -o exq .
```

需要 Go 1.23+。

## 快速开始

```bash
# 看有哪些工作表
exq -i data.xlsx sheets

# 取单元格
exq -i data.xlsx get 'A2, B9'
exq -i data.xlsx get 'product=A2, sku=B9'

# 按表达式筛选
exq -i data.xlsx select 'C.startsWith("2")'
exq -i data.xlsx select 'C.startsWith("2")' -c 'SKU=C, 标题=D' -t json
```

## 全局参数

| 参数          | 说明                    |
| ------------- | ----------------------- |
| `-i, --input` | 输入 Excel 文件（必填） |
| `-s, --sheet` | 工作表名（默认第一张）  |

---

## `exq sheets`

列出工作表及行列数。

```bash
$ exq -i data.xlsx sheets
#    工作表                            行数      列数
1  Sheet1                            1024         8 (默认)
2  订单                               512         6
```

指定 `-s` 只显示那一张。

---

## `exq get`

按单元格坐标取值。坐标格式 `别名=单元格` 或纯 `单元格`，逗号分隔。

```bash
# key 即坐标
$ exq -i data.xlsx get 'A2, B9'
{"A2":"gid://shopify/Product/...", "B9":"SKU-001"}

# 别名
$ exq -i data.xlsx get 'product=A2, sku=B9'
{"product":"gid://shopify/Product/...", "sku":"SKU-001"}

# kv 输出（Tab 分隔）
$ exq -i data.xlsx get 'product=A2, sku=B9' -f kv
product	gid://shopify/Product/...
sku	SKU-001
```

| 参数           | 默认   | 说明                     |
| -------------- | ------ | ------------------------ |
| `-f, --format` | `json` | 输出格式：`json` \| `kv` |

---

## `exq select`

按 CEL 表达式筛选行。**第 1 行视为表头，数据从第 2 行开始。**

### 列变量

所有列以 Excel 列字母命名：`A`、`B`、`C` ... `ZZ`（共 702 个），**全部是字符串类型**。未使用的列注入空字符串。

### 内置函数

| 函数          | 说明                                                                                                           |
| ------------- | -------------------------------------------------------------------------------------------------------------- |
| `date(s)`     | 解析日期，返回 timestamp。支持 ISO、RFC3339、中文日期、Excel 序列号；解析失败返回 CEL 错误 → 该行当 false 跳过 |
| `todouble(s)` | 字符串转 double，失败 → 该行跳过                                                                               |
| `toint(s)`    | 字符串转 int，失败 → 该行跳过                                                                                  |

`date()` 解析规则：

- 纯数字（1 ~ 2958465）→ 当 Excel 序列号
- 带时区（RFC3339）→ 保留时区
- 无时区（`2026-01-02`、`2026年1月2日`、`20260102` 等）→ 按本地时区

### 表达式示例

```
C.startsWith("2")                                          # 前缀
D.contains("手机")                                          # 包含
date(A) >= date("2026-01-01") && date(A) < date("2027-01-01")  # 日期范围
todouble(E) > 1000.0                                       # 数字范围（注意 .0）
toint(F) == 100                                            # 整数相等
C.startsWith("2") && date(A) >= date("2026-01-01")         # 组合
```

**注意**：CEL 是静态类型，`todouble(E) > 1000` 会报类型错误（double 和 int 不匹配），必须写 `1000.0` 或 `todouble(E) > double(1000)`。

### 行为规则

- **编译期错误**（语法错、类型错、返回类型不是 bool）→ 报告错误并退出
- **求值期错误**（某行数据让函数解析失败）→ 该行当 `false`，静默跳过
- **`||` 组合的坑**：`date(A) > ... || D.contains("x")` 中，如果 A 解析失败，整行失败，即使 D 匹配也不会选中

### 参数

| 参数               | 默认    | 说明                                                |
| ------------------ | ------- | --------------------------------------------------- |
| `-c, --col`        | 空      | 输出列，`别名=列字母` 或 `列字母`，可多次或逗号分隔 |
| `-n, --limit`      | `0`     | 最多输出前 N 行匹配（0 不限制）                     |
| `-t, --type`       | `excel` | 输出类型：`excel` \| `json`                         |
| `-o, --out`        | 空      | 输出文件；为空则输出到 stdout                       |
| `--max-cell-width` | `100`   | 单元格最大显示宽度，超出截断为 `..`                 |

### 输出行为

| `-t`    | `-o` 为空          | `-o` 有值  |
| ------- | ------------------ | ---------- |
| `excel` | 对齐表格到 stdout  | 写 `.xlsx` |
| `json`  | JSON 数组到 stdout | 写 `.json` |

`-c` 控制输出列。不传则输出全部列。列顺序按 `-c` 给定顺序。

```bash
# 表格输出（默认）
$ exq -i data.xlsx select 'C.startsWith("2")'
SKU   标题   价格
2001  手机   1999

# 选列
$ exq -i data.xlsx select 'C.startsWith("2")' -c 'SKU=C, 标题=D'

# JSON
$ exq -i data.xlsx select 'C.startsWith("2")' -t json
[
  {
    "SKU": "2001",
    "标题": "手机"
  }
]

# 限制行数
$ exq -i data.xlsx select 'C.startsWith("2")' -n 10

# 写文件
$ exq -i data.xlsx select 'C.startsWith("2")' -o out.xlsx
$ exq -i data.xlsx select 'C.startsWith("2")' -t json -o out.json
```

---

## `exq mcp`

以 MCP server 模式运行（stdio），提供两个工具供 AI 调用：

| 工具           | 对应命令 |
| -------------- | -------- |
| `excel_get`    | `get`    |
| `excel_select` | `select` |

客户端配置（Claude Desktop / Cline 等）：

```json
{
  "mcpServers": {
    "exq": {
      "command": "D:\\path\\to\\exq.exe",
      "args": ["mcp"]
    }
  }
}
```

---

## 输出约定

- **stdout**：只输出数据（表格、JSON、kv）
- **stderr**：提示、错误、进度信息

管道消费安全：

```bash
exq -i data.xlsx select 'C.startsWith("2")' -t json | jq '.[].SKU'
```

---

## 依赖

- [cel-go](https://github.com/cel-expr/cel-go) — CEL 表达式引擎（`cel.dev/cel-go`）
- [excelize](https://github.com/qax-os/excelize) — Excel 读写
- [cobra](https://github.com/spf13/cobra) — 命令行
- [tablewriter](https://github.com/olekukonko/tablewriter) — 表格渲染
- [displaywidth](https://github.com/clipperhouse/displaywidth) — 字符显示宽度
- [go-sdk](https://github.com/modelcontextprotocol/go-sdk) — MCP

## 许可

MIT
