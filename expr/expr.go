package expr

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
)

// MaxColumns 注册 A~ZZ 共 702 个列变量
const MaxColumns = 26 + 26*26 // 702

// ColumnName 返回第 n 列（0 基）的字母名：0->A, 25->Z, 26->AA, 701->ZZ
func ColumnName(n int) string {
	name := ""
	for n >= 0 {
		name = string(rune('A'+n%26)) + name
		n = n/26 - 1
	}
	return name
}

// Compile 构建 CEL 环境并编译表达式，同时校验返回类型为 bool。
func Compile(expr string) (cel.Program, error) {
	opts := make([]cel.EnvOption, 0, MaxColumns+3)
	for i := 0; i < MaxColumns; i++ {
		opts = append(opts, cel.Variable(ColumnName(i), cel.StringType))
	}
	opts = append(opts, dateFunc(), toDoubleFunc(), toIntFunc())

	env, err := cel.NewEnv(opts...)
	if err != nil {
		return nil, err
	}

	ast, issues := env.Compile(expr)
	if issues != nil && issues.Err() != nil {
		return nil, issues.Err()
	}

	if ast.OutputType() != cel.BoolType {
		return nil, fmt.Errorf("表达式必须返回 bool，实际返回 %s", ast.OutputType())
	}

	return env.Program(ast)
}

// ---------- date() ----------

func dateFunc() cel.EnvOption {
	return cel.Function("date",
		cel.Overload("date_string",
			[]*cel.Type{cel.StringType},
			cel.TimestampType,
			cel.UnaryBinding(func(v ref.Val) ref.Val {
				s, ok := v.Value().(string)
				if !ok {
					return types.NewErr("date: 参数必须是字符串")
				}
				t, err := parseDate(s)
				if err != nil {
					return types.NewErr("date: %v", err)
				}
				return types.Timestamp{Time: t}
			}),
		),
	)
}

func parseDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, fmt.Errorf("空字符串")
	}

	// 纯数字：Excel 序列号（合理范围 1 ~ 2958465，对应 1900-01-01 ~ 9999-12-31）
	if isAllDigits(s) {
		days, err := strconv.ParseFloat(s, 64)
		if err == nil && days >= 1 && days <= 2958465 {
			unixSec := (days - 25569) * 86400
			return time.Unix(int64(unixSec), 0).In(time.Local), nil
		}
		// 超出范围，往下尝试其它格式
	}

	// 带时区
	withTZ := []string{
		time.RFC3339,
		time.RFC3339Nano,
	}
	for _, layout := range withTZ {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}

	// 无时区（本地时区）
	noTZ := []string{
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
		"2006/01/02",
		"2006-1-2",
		"2006/1/2",
		"2006年1月2日",
		"2006年01月02日",
		"20060102",
	}
	for _, layout := range noTZ {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("无法解析日期: %q", s)
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// ---------- todouble() ----------

func toDoubleFunc() cel.EnvOption {
	return cel.Function("todouble",
		cel.Overload("todouble_string",
			[]*cel.Type{cel.StringType},
			cel.DoubleType,
			cel.UnaryBinding(func(v ref.Val) ref.Val {
				s, ok := v.Value().(string)
				if !ok {
					return types.NewErr("todouble: 参数必须是字符串")
				}
				n, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
				if err != nil {
					return types.NewErr("todouble: 无法解析 %q", s)
				}
				return types.Double(n)
			}),
		),
	)
}

// ---------- toint() ----------

func toIntFunc() cel.EnvOption {
	return cel.Function("toint",
		cel.Overload("toint_string",
			[]*cel.Type{cel.StringType},
			cel.IntType,
			cel.UnaryBinding(func(v ref.Val) ref.Val {
				s, ok := v.Value().(string)
				if !ok {
					return types.NewErr("toint: 参数必须是字符串")
				}
				n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
				if err != nil {
					return types.NewErr("toint: 无法解析 %q", s)
				}
				return types.Int(n)
			}),
		),
	)
}
