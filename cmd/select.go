package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"exq/expr"
	"exq/sink"

	"github.com/clipperhouse/displaywidth"
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/renderer"
	"github.com/olekukonko/tablewriter/tw"
	"github.com/spf13/cobra"
)

var (
	flagType         string
	flagOutFile      string
	flagCols         []string
	flagLimit        int
	flagMaxCellWidth int
)

var selectCmd = &cobra.Command{
	Use:   "select <CEL表达式>",
	Short: "按 CEL 表达式筛选行",
	Long: `按 CEL 表达式筛选行。

列变量为 A、B、C ... ZZ（全部字符串），支持以下函数：
  date(s)      解析日期，支持 ISO、RFC3339、中文日期、Excel 序列号
  todouble(s)  字符串转 double
  toint(s)     字符串转 int

输出列可用 -c 选择（可多次指定，也可一次逗号分隔）：
  -c '编号=A' -c 'SKU=C' -c '变体=B, 变体价格=E'
  -c A -c B -c C

输出类型由 -t 指定，默认 excel：
  excel  不指定 -o 时输出对齐表格到 stdout，指定 -o 时写入 .xlsx
  json   输出 JSON 到 stdout 或文件

示例：
  exq -i data.xlsx select 'C.startsWith("2")'
  exq -i data.xlsx select 'C.startsWith("2")' -t json
  exq -i data.xlsx select 'C.startsWith("2")' -c 'SKU=C' -c '标题=D' -t json -o out.json
  exq -i data.xlsx select 'C.startsWith("2")' -c 'SKU=C' -o out.xlsx`,
	Args: cobra.ExactArgs(1),
	RunE: runSelect,
}

func init() {
	selectCmd.Flags().StringVarP(&flagType, "type", "t", "excel", "输出类型：excel | json")
	selectCmd.Flags().StringVarP(&flagOutFile, "out", "o", "", "输出文件路径；为空则输出到 stdout")
	selectCmd.Flags().StringSliceVarP(&flagCols, "col", "c", nil, "输出列，格式 '别名=列字母' 或 '列字母'，可多次指定")
	selectCmd.Flags().IntVarP(&flagLimit, "limit", "n", 0, "最多输出前 N 行匹配结果（0 表示不限制）")
	selectCmd.Flags().IntVar(&flagMaxCellWidth, "max-cell-width", 100, "单元格最大显示宽度，超出则截断为 '..'")
	rootCmd.AddCommand(selectCmd)
}

func runSelect(cmd *cobra.Command, args []string) error {
	prg, err := expr.Compile(args[0])
	if err != nil {
		return fmt.Errorf("表达式编译失败: %w", err)
	}

	colSpecs, err := expr.ParseColSpecs(flagCols)
	if err != nil {
		return err
	}

	f, err := openExcel(flagInput)
	if err != nil {
		return err
	}
	defer f.Close()

	sheet := resolveSheet(f)
	rows, err := f.GetRows(sheet)
	if err != nil {
		return fmt.Errorf("读取工作表失败: %w", err)
	}
	if len(rows) == 0 {
		return fmt.Errorf("工作表为空")
	}

	var outHeaders []string
	if len(colSpecs) > 0 {
		outHeaders = make([]string, len(colSpecs))
		for i, s := range colSpecs {
			outHeaders[i] = s.Key
		}
	} else {
		outHeaders = rows[0]
	}

	project := func(row []string) []string {
		if len(colSpecs) == 0 {
			return row
		}
		res := make([]string, len(colSpecs))
		for i, s := range colSpecs {
			if s.Col < len(row) {
				res[i] = row[s.Col]
			} else {
				res[i] = ""
			}
		}
		return res
	}

	var matchedRows [][]string
	for i, row := range rows {
		if i == 0 {
			continue
		}
		if flagLimit > 0 && len(matchedRows) >= flagLimit {
			break
		}
		res, _, err := prg.Eval(buildRowEnv(row))
		if err != nil {
			continue
		}
		ok, _ := res.Value().(bool)
		if ok {
			matchedRows = append(matchedRows, project(row))
		}
	}

	typ := strings.ToLower(strings.TrimSpace(flagType))
	if typ == "" {
		typ = "excel"
	}

	switch typ {
	case "excel":
		if flagOutFile == "" {
			writeTable(os.Stdout, outHeaders, matchedRows, flagMaxCellWidth)
			return nil
		}
		out, err := sink.New("excel", flagOutFile, outHeaders)
		if err != nil {
			return fmt.Errorf("创建输出失败: %w", err)
		}
		if err := out.Open(); err != nil {
			return fmt.Errorf("初始化输出失败: %w", err)
		}
		for _, r := range matchedRows {
			if err := out.Write(r); err != nil {
				return fmt.Errorf("写入失败: %w", err)
			}
		}
		if err := out.Close(); err != nil {
			return fmt.Errorf("关闭输出失败: %w", err)
		}
		fmt.Fprintf(os.Stderr, "匹配 %d 行\n", len(matchedRows))
		return nil

	case "json":
		if flagOutFile == "" {
			return writeJSON(os.Stdout, outHeaders, matchedRows)
		}
		return writeJSONFile(flagOutFile, outHeaders, matchedRows)

	default:
		return fmt.Errorf("未知输出类型: %s（可选 excel | json）", typ)
	}
}

// ---------- JSON 输出 ----------

func rowsToMaps(headers []string, rows [][]string) []map[string]string {
	result := make([]map[string]string, 0, len(rows))
	for _, row := range rows {
		m := make(map[string]string, len(headers))
		for i, h := range headers {
			if i < len(row) {
				m[h] = row[i]
			} else {
				m[h] = ""
			}
		}
		result = append(result, m)
	}
	return result
}

func writeJSON(w io.Writer, headers []string, rows [][]string) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(rowsToMaps(headers, rows))
}

func writeJSONFile(path string, headers []string, rows [][]string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("创建 %s 失败: %w", path, err)
	}
	defer f.Close()

	if err := writeJSON(f, headers, rows); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "匹配 %d 行，已写入 %s\n", len(rows), path)
	return nil
}

// ---------- 表格输出 ----------

func writeTable(w io.Writer, headers []string, rows [][]string, maxWidth int) {
	truncHeaders := make([]string, len(headers))
	for i, h := range headers {
		truncHeaders[i] = truncate(h, maxWidth)
	}

	table := tablewriter.NewTable(w,
		tablewriter.WithRenderer(renderer.NewBlueprint(tw.Rendition{
			Borders: tw.BorderNone,
			Settings: tw.Settings{
				Separators: tw.Separators{BetweenColumns: tw.Off, BetweenRows: tw.Off},
				Lines:      tw.LinesNone,
			},
		})),
		tablewriter.WithConfig(tablewriter.Config{
			Header: tw.CellConfig{
				Formatting: tw.CellFormatting{AutoFormat: tw.Off},
				Alignment:  tw.CellAlignment{Global: tw.AlignLeft},
				Padding:    tw.CellPadding{Global: tw.Padding{Left: "", Right: "   "}},
			},
			Row: tw.CellConfig{
				Formatting: tw.CellFormatting{AutoWrap: tw.WrapNone},
				Alignment:  tw.CellAlignment{Global: tw.AlignLeft},
				Padding:    tw.CellPadding{Global: tw.Padding{Left: "", Right: "   "}},
			},
		}),
	)

	headerAny := make([]any, len(truncHeaders))
	for i, h := range truncHeaders {
		headerAny[i] = h
	}
	table.Header(headerAny...)

	for _, row := range rows {
		trimmed := make([]string, len(row))
		for i, cell := range row {
			trimmed[i] = truncate(cell, maxWidth)
		}
		table.Append(trimmed)
	}

	table.Render()
}

func truncate(s string, max int) string {
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

func buildRowEnv(row []string) map[string]interface{} {
	env := make(map[string]interface{}, expr.MaxColumns)
	for i := 0; i < expr.MaxColumns; i++ {
		name := expr.ColumnName(i)
		if i < len(row) {
			env[name] = strings.TrimSpace(row[i])
		} else {
			env[name] = ""
		}
	}
	return env
}
