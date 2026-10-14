package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/xuri/excelize/v2"
)

var (
	flagInput string
	flagSheet string
)

// Version 由 -ldflags 注入，默认 dev
var Version = "dev"

var rootCmd = &cobra.Command{
	Use:   "exq",
	Short: "exq — Excel 查询工具",
	Long: `exq 是一个命令行 Excel 查询工具。

支持三种模式：
  exq get    <坐标列表>   按单元格坐标取值
  exq select <CEL表达式>  按表达式筛选行
  exq mcp                作为 MCP server 运行（stdio）`,
	SilenceUsage: true,
	Version:      Version,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&flagInput, "input", "i", "", "输入 Excel 文件（必填）")
	rootCmd.PersistentFlags().StringVarP(&flagSheet, "sheet", "s", "", "工作表名（默认第一个）")
}

func openExcel(path string) (*excelize.File, error) {
	if path == "" {
		return nil, fmt.Errorf("缺少输入文件，请用 -i 指定")
	}
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, fmt.Errorf("打开 %s 失败: %w", path, err)
	}
	return f, nil
}

func resolveSheet(f *excelize.File) string {
	if flagSheet != "" {
		return flagSheet
	}
	return f.GetSheetName(0)
}
