package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/clipperhouse/displaywidth"
	"github.com/spf13/cobra"
)

var sheetsCmd = &cobra.Command{
	Use:   "sheets",
	Short: "列出 Excel 文件中的所有工作表",
	Long: `列出 Excel 文件中的所有工作表，以及每个表的行列数。

示例：
  exq -i data.xlsx sheets
  exq -i data.xlsx -s 订单 sheets`,
	Args: cobra.NoArgs,
	RunE: runSheets,
}

func init() {
	rootCmd.AddCommand(sheetsCmd)
}

func runSheets(cmd *cobra.Command, args []string) error {
	f, err := openExcel(flagInput)
	if err != nil {
		return err
	}
	defer f.Close()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return fmt.Errorf("文件没有工作表")
	}

	only := flagSheet

	fmt.Fprintf(os.Stdout, "%-4s  %-30s  %8s  %8s\n", "#", "工作表", "行数", "列数")

	idx := 0
	for _, name := range sheets {
		if only != "" && name != only {
			continue
		}
		rows, err := f.GetRows(name)
		if err != nil {
			return fmt.Errorf("读取工作表 %q 失败: %w", name, err)
		}

		rowCount := len(rows)
		colCount := 0
		for _, r := range rows {
			if len(r) > colCount {
				colCount = len(r)
			}
		}

		marker := ""
		if name == f.GetSheetName(0) {
			marker = " (默认)"
		}

		idx++
		fmt.Fprintf(os.Stdout, "%-4d  %-30s  %8d  %8d%s\n",
			idx, truncateName(name, 30), rowCount, colCount, marker)
	}

	if only != "" && idx == 0 {
		return fmt.Errorf("工作表 %q 不存在", only)
	}

	return nil
}

func truncateName(s string, max int) string {
	if max <= 0 || displaywidth.String(s) <= max {
		return s
	}
	limit := max - 2
	if limit < 0 {
		limit = 0
	}
	var b strings.Builder
	w := 0
	for _, r := range s {
		rw := displaywidth.Rune(r)
		if w+rw > limit {
			break
		}
		b.WriteRune(r)
		w += rw
	}
	return b.String() + ".."
}
