package expr

import (
	"fmt"
	"regexp"
	"strings"
)

// ColSpec 表示输出列的定义
type ColSpec struct {
	Key string // 输出表头
	Col int    // 0 基列索引
}

var colLetterRe = regexp.MustCompile(`^[A-Za-z]{1,3}$`)

// ColNameToIndex 把列字母转 0 基索引：A->0, Z->25, AA->26
func ColNameToIndex(s string) (int, error) {
	s = strings.ToUpper(strings.TrimSpace(s))
	if !colLetterRe.MatchString(s) {
		return 0, fmt.Errorf("无效列字母: %q", s)
	}
	n := 0
	for _, r := range s {
		n = n*26 + int(r-'A'+1)
	}
	return n - 1, nil
}

// ParseColSpecs 解析 -c 参数，支持：
//   - '别名=列字母'，如 'SKU=C'
//   - 纯列字母，如 'C'
//   - 逗号分隔多个，如 'SKU=C, 标题=D'
func ParseColSpecs(items []string) ([]ColSpec, error) {
	var specs []ColSpec
	seen := map[string]bool{}

	for _, item := range items {
		for _, part := range strings.Split(item, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			var spec ColSpec
			if idx := strings.Index(part, "="); idx >= 0 {
				spec.Key = strings.TrimSpace(part[:idx])
				colLetter := strings.TrimSpace(part[idx+1:])
				if spec.Key == "" {
					return nil, fmt.Errorf("别名不能为空: %q", part)
				}
				ci, err := ColNameToIndex(colLetter)
				if err != nil {
					return nil, err
				}
				spec.Col = ci
			} else {
				ci, err := ColNameToIndex(part)
				if err != nil {
					return nil, err
				}
				spec.Col = ci
				spec.Key = strings.ToUpper(part)
			}
			if seen[spec.Key] {
				return nil, fmt.Errorf("重复的输出列名: %q", spec.Key)
			}
			seen[spec.Key] = true
			specs = append(specs, spec)
		}
	}
	return specs, nil
}
