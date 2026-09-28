# QuarkLangQkdoc

QuarkLang API 文档生成器——工具链成员。读取 .qk 源码（复用 `internal/lang` 的解析器），
把类型/接口/实现/函数/FFI 库/类型别名/导入导出成 Markdown，并生成 `index.md`。

## 用法

```sh
go build -o qkdoc .            # go.mod 里 replace 指向 ../QuarkLang
qkdoc -o docs src/*.qk
qkdoc -o docs -title "Style API" style.qk
```

## 输出

每个源文件一份 `<名字>.md`：

```markdown
# style.qk

> 来源：`/path/style.qk` · program library

## 类型
### StyleGradient
---------- 值类型（渐变 / 变换 / 矩形） ----------
| 字段 | 类型 |
|---|---|
| `kind` | `int` |
...

## 接口 / ## 实现 / ## 函数 / ## FFI 库 / ## 类型别名 / ## 导出（pub）
```

- 文档注释：声明**上一行紧邻**的 `//` 或 `/* */` 注释块
- 源文件无法通过解析/类型检查时，仍会出文档，但在头部标注具体错误

## 实测

```
$ qkdoc -o /tmp/qkdocout style.qk regex.qk
已生成 /tmp/qkdocout/style.md
已生成 /tmp/qkdocout/regex.md
已生成 /tmp/qkdocout/index.md
$ grep -c '^### ' /tmp/qkdocout/{style,regex}.md
/tmp/qkdocout/style.md:10
/tmp/qkdocout/regex.md:13
```
