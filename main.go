// qkdoc: QuarkLang API 文档生成器——工具链成员
//
// 用法: qkdoc [-o outdir] [-title T] files...
//
// 为每个 .qk 生成一份 Markdown（类型/接口/实现/函数/FFI 库/类型别名/导入），
// 文档注释取声明前紧邻的 // 或 /* */ 注释；再生成 index.md 汇总。
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"quarklang/internal/lang"
)

type docComment struct {
	start, end int // 行号区间
	text       string
}

// scanComments 扫描注释（词法器会丢注释，这里自带扫描）。
func scanComments(src string) []docComment {
	var out []docComment
	lines := strings.Split(src, "\n")
	inBlock := false
	var buf []string
	start := 0
	for i, ln := range lines {
		trimmed := strings.TrimSpace(ln)
		if inBlock {
			buf = append(buf, strings.TrimSpace(strings.TrimSuffix(trimmed, "*/")))
			if strings.Contains(trimmed, "*/") {
				out = append(out, docComment{start, i + 1, strings.Join(buf, "\n")})
				inBlock = false
				buf = nil
			}
			continue
		}
		if strings.HasPrefix(trimmed, "/*") {
			body := strings.TrimPrefix(trimmed, "/*")
			if strings.Contains(body, "*/") {
				body = strings.TrimSuffix(body, "*/")
				out = append(out, docComment{i + 1, i + 1, strings.TrimSpace(body)})
				continue
			}
			inBlock = true
			start = i + 1
			buf = []string{strings.TrimSpace(body)}
			continue
		}
		if strings.HasPrefix(trimmed, "//") {
			if len(buf) > 0 && start == i { // 与上一行连续的 // 块
				buf = append(buf, strings.TrimSpace(strings.TrimPrefix(trimmed, "//")))
			} else {
				buf = []string{strings.TrimSpace(strings.TrimPrefix(trimmed, "//"))}
				start = i + 1
			}
			// 向后看是否还是注释行：这里直接记录，下一次循环会覆盖
			out = append(out, docComment{start, i + 1, strings.Join(buf, "\n")})
			continue
		}
		if trimmed != "" {
			buf = nil
			start = 0
		}
	}
	// 合并连续的 // 块（后一条覆盖前一条时以行号连续为准）
	var merged []docComment
	for _, c := range out {
		if len(merged) > 0 {
			last := &merged[len(merged)-1]
			if c.start == last.start && c.end > last.end {
				last.end = c.end
				last.text = c.text
				continue
			}
		}
		merged = append(merged, c)
	}
	return merged
}

// docFor 取声明行上方紧邻的注释块。
func docFor(cs []docComment, line int) string {
	best := ""
	for _, c := range cs {
		if c.end < line && line-c.end <= 1 {
			if strings.Contains(c.text, "\n") {
				best = c.text
			} else {
				// 单行注释可能属于连续块：向前找同 start 的更完整版本
				best = c.text
			}
		}
	}
	return strings.TrimSpace(best)
}

func mdEscape(s string) string { return strings.ReplaceAll(s, "|", "\\|") }

func sig(f *lang.FuncDecl) string {
	var ps []string
	for _, p := range f.Params {
		t := p.Type
		if p.Decor != "" {
			t = p.Decor + " " + t
		}
		ps = append(ps, t+" "+p.Name)
	}
	tp := ""
	if len(f.TypeParams) > 0 {
		tp = "<" + strings.Join(f.TypeParams, ", ") + ">"
	}
	ret := f.Ret
	if ret == "" {
		ret = "void"
	}
	return fmt.Sprintf("fn%s %s(%s) %s", tp, f.Name, strings.Join(ps, ", "), ret)
}

func gen(path, outDir string) (string, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	prog, cerr := lang.Compile(string(src))
	cs := scanComments(string(src))
	var b strings.Builder
	name := filepath.Base(path)
	fmt.Fprintf(&b, "# %s\n\n", name)
	kind := prog.Kind
	if kind == "" {
		kind = "main"
	}
	fmt.Fprintf(&b, "> 来源：`%s` · program %s", path, kind)
	if cerr != nil {
		fmt.Fprintf(&b, " · **解析/类型检查未通过：%v**", cerr)
	}
	b.WriteString("\n\n")
	if len(prog.Imports) > 0 {
		fmt.Fprintf(&b, "## 导入\n\n")
		for _, im := range prog.Imports {
			fmt.Fprintf(&b, "- `%s`\n", im)
		}
		b.WriteString("\n")
	}
	if len(prog.Structs) > 0 {
		fmt.Fprintf(&b, "## 类型\n\n")
		for _, s := range prog.Structs {
			d := docFor(cs, s.Pos.Line)
			if d != "" {
				fmt.Fprintf(&b, "### %s\n\n%s\n\n", s.Name, d)
			} else {
				fmt.Fprintf(&b, "### %s\n\n", s.Name)
			}
			if len(s.Members) > 0 {
				b.WriteString("| 字段 | 类型 |\n|---|---|\n")
				for _, f := range s.Members {
					fmt.Fprintf(&b, "| `%s` | `%s` |\n", f.Name, mdEscape(f.Type))
				}
				b.WriteString("\n")
			}
		}
	}
	if len(prog.Interfaces) > 0 {
		fmt.Fprintf(&b, "## 接口\n\n")
		for _, it := range prog.Interfaces {
			d := docFor(cs, it.Pos.Line)
			fmt.Fprintf(&b, "### %s\n\n", it.Name)
			if d != "" {
				fmt.Fprintf(&b, "%s\n\n", d)
			}
			for _, m := range it.Methods {
				ps := []string{}
				for _, p := range m.Params {
					ps = append(ps, p.Type+" "+p.Name)
				}
				ret := m.Ret
				if ret == "" {
					ret = "void"
				}
				fmt.Fprintf(&b, "- `fn %s(%s) %s`\n", m.Name, strings.Join(ps, ", "), ret)
			}
			if len(it.Expands) > 0 {
				fmt.Fprintf(&b, "- expand: %s\n", strings.Join(it.Expands, ", "))
			}
			b.WriteString("\n")
		}
	}
	if len(prog.Impls) > 0 {
		fmt.Fprintf(&b, "## 实现\n\n")
		for _, im := range prog.Impls {
			tp := ""
			if len(im.TypeParams) > 0 {
				tp = "<" + strings.Join(im.TypeParams, ", ") + ">"
			}
			fmt.Fprintf(&b, "### impl%s for %s\n\n", tp, im.Type)
			for _, m := range im.Methods {
				fmt.Fprintf(&b, "- `%s`\n", sig(m)[3:])
			}
			b.WriteString("\n")
		}
	}
	if len(prog.Funcs) > 0 {
		fmt.Fprintf(&b, "## 函数\n\n")
		for _, f := range prog.Funcs {
			if strings.HasPrefix(f.Name, "@") {
				continue
			}
			d := docFor(cs, f.Pos.Line)
			fmt.Fprintf(&b, "### %s\n\n```qk\n%s\n```\n\n", f.Name, sig(f))
			if d != "" {
				fmt.Fprintf(&b, "%s\n\n", d)
			}
		}
	}
	if len(prog.Libraries) > 0 {
		fmt.Fprintf(&b, "## FFI 库\n\n")
		for _, l := range prog.Libraries {
			fmt.Fprintf(&b, "### %s\n\n", l.Name)
			for _, s := range l.Methods {
				ps := []string{}
				for _, p := range s.Params {
					ps = append(ps, p.Type+" "+p.Name)
				}
				ret := s.Ret
				if ret == "" {
					ret = "void"
				}
				fmt.Fprintf(&b, "- `fn %s(%s) %s`\n", s.Name, strings.Join(ps, ", "), ret)
			}
			b.WriteString("\n")
		}
	}
	if len(prog.TypeAliases) > 0 {
		fmt.Fprintf(&b, "## 类型别名\n\n")
		for _, t := range prog.TypeAliases {
			fmt.Fprintf(&b, "- `%s = %s`\n", t.Name, t.Type)
		}
		b.WriteString("\n")
	}
	if len(prog.Pub) > 0 {
		fmt.Fprintf(&b, "## 导出（pub）\n\n")
		for _, p := range prog.Pub {
			fmt.Fprintf(&b, "- `%s`\n", p)
		}
		b.WriteString("\n")
	}
	out := filepath.Join(outDir, strings.TrimSuffix(name, ".qk")+".md")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(out, []byte(b.String()), 0o644); err != nil {
		return "", err
	}
	return out, nil
}

func main() {
	args := os.Args[1:]
	outDir := "docs"
	title := "QuarkLang API"
	var files []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-o":
			if i+1 < len(args) {
				i++
				outDir = args[i]
			}
		case "-title":
			if i+1 < len(args) {
				i++
				title = args[i]
			}
		case "-h", "--help":
			fmt.Println("usage: qkdoc [-o outdir] [-title T] files...")
			return
		default:
			files = append(files, args[i])
		}
	}
	if len(files) == 0 {
		fmt.Fprintln(os.Stderr, "usage: qkdoc [-o outdir] [-title T] files...")
		os.Exit(2)
	}
	sort.Strings(files)
	var made []string
	for _, f := range files {
		out, err := gen(f, outDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "qkdoc: %s: %v\n", f, err)
			continue
		}
		made = append(made, out)
		fmt.Println("已生成", out)
	}
	// index.md
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", title)
	fmt.Fprintf(&b, "共 %d 个源文件。\n\n", len(made))
	for _, m := range made {
		rel := filepath.Base(m)
		fmt.Fprintf(&b, "- [%s](%s)\n", rel, rel)
	}
	idx := filepath.Join(outDir, "index.md")
	_ = os.WriteFile(idx, []byte(b.String()), 0o644)
	fmt.Println("已生成", idx)
}
