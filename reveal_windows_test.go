//go:build windows

package main

import (
	"strings"
	"testing"
)

// /select, 的引号位置是这类 bug 的根源：开关留在引号外，只有路径加引号。
// 写成 "/select,path" 会被 explorer 当成单个参数，开关失效并退回"文档"目录。
func TestExplorerCommandLineQuoting(t *testing.T) {
	cases := []struct {
		name string
		path string
	}{
		{"含空格中文", `E:\blog-files\学习类\Markdown 入门到精通 完整版小白教程.html`},
		{"无空格", `E:\blog-files\关于.md`},
		{"含括号与空格", `E:\blog-files\手册\Git 命令大全（小白友好版）.html`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			line := explorerCommandLine(tc.path)

			// 开关必须在第一个引号之前出现
			selectIdx := strings.Index(line, "/select,")
			firstQuote := strings.Index(line, `"`)
			if selectIdx == -1 {
				t.Fatal("命令行缺少 /select,")
			}
			if firstQuote != -1 && selectIdx > firstQuote {
				t.Fatalf("/select, 被包在引号内，explorer 会忽略它: %s", line)
			}

			// /select, 之后紧跟一个引号，引号内是完整路径
			want := `/select,"` + tc.path + `"`
			if !strings.Contains(line, want) {
				t.Fatalf("命令行 = %s，期望包含 %s", line, want)
			}

			// 不能出现整体包住开关的错误形式
			if strings.Contains(line, `"/select,`) {
				t.Fatalf("命令行把开关和路径整体加引号: %s", line)
			}
		})
	}
}

func TestExplorerCommandLineDir(t *testing.T) {
	dir := `E:\blog-files\学习类`
	line := explorerCommandLineDir(dir)
	if !strings.HasSuffix(line, `"`+dir+`"`) {
		t.Fatalf("命令行 = %s", line)
	}
	if strings.Contains(line, "/select") {
		t.Fatal("打开目录时不应带 /select")
	}
}
