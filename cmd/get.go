package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

var flagGetFormat string

var getCmd = &cobra.Command{
	Use:   "get <坐标列表>",
	Short: "按单元格坐标取值",
	Long: `按单元格坐标取值。

坐标格式（可混用，逗号分隔，空格可选）：
  <单元格>                  key 即坐标本身
  <别名>=<单元格>           使用别名作 key

示例：
  exq -i data.xlsx get 'A2, B9'
  exq -i data.xlsx get 'product=A2, sku=B9'
  exq -i data.xlsx get 'product=A2, B9' -f kv`,
	Args: cobra.ExactArgs(1),
	RunE: runGet,
}

func init() {
	getCmd.Flags().StringVarP(&flagGetFormat, "format", "f", "json", "输出格式：json | kv")
	rootCmd.AddCommand(getCmd)
}

type cellRef struct {
	Cell string
	Key  string
}

var cellCoordRe = regexp.MustCompile(`^\$?[A-Za-z]{1,3}\$?[0-9]+$`)

func parseCellRefs(s string) ([]cellRef, error) {
	var refs []cellRef
	seenKey := map[string]bool{}

	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		var ref cellRef
		if idx := strings.Index(part, "="); idx >= 0 {
			ref.Key = strings.TrimSpace(part[:idx])
			ref.Cell = strings.TrimSpace(part[idx+1:])
			if ref.Key == "" {
				return nil, fmt.Errorf("别名不能为空: %q", part)
			}
			if ref.Cell == "" {
				return nil, fmt.Errorf("坐标不能为空: %q", part)
			}
		} else {
			ref.Cell = part
			ref.Key = part
		}

		if !cellCoordRe.MatchString(ref.Cell) {
			return nil, fmt.Errorf("无效坐标: %q", ref.Cell)
		}
		if seenKey[ref.Key] {
			return nil, fmt.Errorf("重复的 key: %q", ref.Key)
		}
		seenKey[ref.Key] = true

		refs = append(refs, ref)
	}

	if len(refs) == 0 {
		return nil, fmt.Errorf("未提供有效坐标")
	}
	return refs, nil
}

func runGet(cmd *cobra.Command, args []string) error {
	refs, err := parseCellRefs(args[0])
	if err != nil {
		return err
	}

	f, err := openExcel(flagInput)
	if err != nil {
		return err
	}
	defer f.Close()

	sheet := resolveSheet(f)

	keys := make([]string, 0, len(refs))
	vals := make(map[string]string, len(refs))

	for _, r := range refs {
		v, err := f.GetCellValue(sheet, r.Cell)
		if err != nil {
			return fmt.Errorf("读取 %s 失败: %w", r.Cell, err)
		}
		keys = append(keys, r.Key)
		vals[r.Key] = v
	}

	switch strings.ToLower(flagGetFormat) {
	case "json":
		return writeGetJSON(os.Stdout, keys, vals)
	case "kv":
		return writeGetKV(os.Stdout, keys, vals)
	default:
		return fmt.Errorf("未知输出格式: %s（可选 json / kv）", flagGetFormat)
	}
}

func writeGetJSON(w *os.File, keys []string, vals map[string]string) error {
	var b strings.Builder
	b.WriteString("{")
	for i, k := range keys {
		if i > 0 {
			b.WriteString(", ")
		}
		kb, _ := json.Marshal(k)
		vb, _ := json.Marshal(vals[k])
		b.Write(kb)
		b.WriteString(":")
		b.Write(vb)
	}
	b.WriteString("}\n")
	_, err := w.WriteString(b.String())
	return err
}

func writeGetKV(w *os.File, keys []string, vals map[string]string) error {
	for _, k := range keys {
		if _, err := fmt.Fprintf(w, "%s\t%s\n", k, vals[k]); err != nil {
			return err
		}
	}
	return nil
}
