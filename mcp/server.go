package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"exq/expr"
	"exq/sink"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/xuri/excelize/v2"
)

// Run 启动 stdio MCP server
func Run(ctx context.Context) error {
	server := mcpsdk.NewServer(&mcpsdk.Implementation{
		Name:    "exq",
		Version: "0.1.0",
	}, nil)

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "excel_get",
		Description: "按单元格坐标从 Excel 取值。坐标可带别名，如 'A2, B9' 或 'product=A2, sku=B9'。",
	}, handleGet)

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "excel_select",
		Description: "按 CEL 表达式筛选 Excel 行。列变量为 A、B、C...ZZ（字符串）。支持 date(s)、todouble(s)、toint(s) 函数。可用 cols 选择输出列，格式 '别名=列字母' 或 '列字母'。可用 limit 限制返回行数。",
	}, handleSelect)

	return server.Run(ctx, &mcpsdk.StdioTransport{})
}

// ---------- get ----------

type getInput struct {
	File  string `json:"file" jsonschema:"Excel 文件路径"`
	Sheet string `json:"sheet,omitempty" jsonschema:"工作表名，默认第一个"`
	Cells string `json:"cells" jsonschema:"坐标列表，如 'A2, B9' 或 'product=A2, sku=B9'"`
}

type getOutput struct {
	Values map[string]string `json:"values"`
}

func handleGet(ctx context.Context, req *mcpsdk.CallToolRequest, in getInput) (*mcpsdk.CallToolResult, getOutput, error) {
	f, err := excelize.OpenFile(in.File)
	if err != nil {
		return nil, getOutput{}, fmt.Errorf("打开 %s 失败: %w", in.File, err)
	}
	defer f.Close()

	sheet := in.Sheet
	if sheet == "" {
		sheet = f.GetSheetName(0)
	}

	refs, err := parseCellRefs(in.Cells)
	if err != nil {
		return nil, getOutput{}, err
	}

	vals := make(map[string]string, len(refs))
	for _, r := range refs {
		v, err := f.GetCellValue(sheet, r.Cell)
		if err != nil {
			return nil, getOutput{}, fmt.Errorf("读取 %s 失败: %w", r.Cell, err)
		}
		vals[r.Key] = v
	}

	return nil, getOutput{Values: vals}, nil
}

type cellRef struct {
	Cell string
	Key  string
}

func parseCellRefs(s string) ([]cellRef, error) {
	var refs []cellRef
	seen := map[string]bool{}
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		var ref cellRef
		if idx := strings.Index(part, "="); idx >= 0 {
			ref.Key = strings.TrimSpace(part[:idx])
			ref.Cell = strings.TrimSpace(part[idx+1:])
		} else {
			ref.Cell = part
			ref.Key = part
		}
		if ref.Cell == "" || ref.Key == "" {
			return nil, fmt.Errorf("无效坐标: %q", part)
		}
		if seen[ref.Key] {
			return nil, fmt.Errorf("重复的 key: %q", ref.Key)
		}
		seen[ref.Key] = true
		refs = append(refs, ref)
	}
	if len(refs) == 0 {
		return nil, fmt.Errorf("未提供有效坐标")
	}
	return refs, nil
}

// ---------- select ----------

type selectInput struct {
	File  string   `json:"file" jsonschema:"Excel 文件路径"`
	Sheet string   `json:"sheet,omitempty" jsonschema:"工作表名，默认第一个"`
	Expr  string   `json:"expr" jsonschema:"CEL 表达式，列变量 A~ZZ，支持 date()/todouble()/toint()"`
	Out   string   `json:"out,omitempty" jsonschema:"输出 xlsx 路径；为空则只返回匹配行，不落盘"`
	Cols  []string `json:"cols,omitempty" jsonschema:"输出列，格式 '别名=列字母' 或 '列字母'，可多个"`
	Limit int      `json:"limit,omitempty" jsonschema:"最多返回前 N 行匹配结果（0 表示不限制）"`
}

type selectOutput struct {
	Matched int        `json:"matched"`
	Headers []string   `json:"headers"`
	Rows    [][]string `json:"rows"`
	OutFile string     `json:"out_file,omitempty"`
}

func handleSelect(ctx context.Context, req *mcpsdk.CallToolRequest, in selectInput) (*mcpsdk.CallToolResult, selectOutput, error) {
	prg, err := expr.Compile(in.Expr)
	if err != nil {
		return nil, selectOutput{}, fmt.Errorf("表达式编译失败: %w", err)
	}

	colSpecs, err := expr.ParseColSpecs(in.Cols)
	if err != nil {
		return nil, selectOutput{}, err
	}

	f, err := excelize.OpenFile(in.File)
	if err != nil {
		return nil, selectOutput{}, fmt.Errorf("打开 %s 失败: %w", in.File, err)
	}
	defer f.Close()

	sheet := in.Sheet
	if sheet == "" {
		sheet = f.GetSheetName(0)
	}
	rows, err := f.GetRows(sheet)
	if err != nil {
		return nil, selectOutput{}, fmt.Errorf("读取工作表失败: %w", err)
	}
	if len(rows) == 0 {
		return nil, selectOutput{}, fmt.Errorf("工作表为空")
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

	// 收集匹配行，达到 limit 就停（与 CLI 语义一致）
	var matchedRows [][]string
	for i, row := range rows {
		if i == 0 {
			continue
		}
		if in.Limit > 0 && len(matchedRows) >= in.Limit {
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

	out := selectOutput{
		Matched: len(matchedRows),
		Headers: outHeaders,
		Rows:    matchedRows,
	}

	if in.Out != "" {
		s, err := sink.New("excel", in.Out, outHeaders)
		if err != nil {
			return nil, selectOutput{}, err
		}
		if err := s.Open(); err != nil {
			return nil, selectOutput{}, err
		}
		for _, r := range matchedRows {
			if err := s.Write(r); err != nil {
				return nil, selectOutput{}, err
			}
		}
		if err := s.Close(); err != nil {
			return nil, selectOutput{}, err
		}
		out.OutFile = in.Out
	}

	text := summarize(out)
	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{
			&mcpsdk.TextContent{Text: text},
		},
	}, out, nil
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

func summarize(o selectOutput) string {
	b, _ := json.Marshal(o)
	return string(b)
}
