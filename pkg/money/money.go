// Package money 提供精确的金额类型。
//
// 为什么不用 float64：0.1 + 0.2 在浮点里等于 0.30000000000000004，
// 金额计算出现这种误差是不可接受的。这里用 int64 存"分"，
// 在数据库(DECIMAL) 与 JSON(字符串) 之间精确转换。
package money

import (
	"database/sql/driver"
	"fmt"
	"strconv"
	"strings"
)

// Cents 以分为单位的金额。12990 表示 129.90 元。
type Cents int64

// FromYuan 把 "129.90" 这样的字符串转成分。
func FromYuan(s string) (Cents, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("金额不能为空")
	}

	neg := false
	switch s[0] {
	case '-':
		neg = true
		s = s[1:]
	case '+':
		s = s[1:]
	}
	if s == "" {
		return 0, fmt.Errorf("金额格式不正确: %q", s)
	}

	intPart, fracPart, hasDot := strings.Cut(s, ".")
	if intPart == "" {
		intPart = "0"
	}
	if len(fracPart) > 2 {
		return 0, fmt.Errorf("金额最多两位小数: %q", s)
	}
	if hasDot && fracPart == "" {
		return 0, fmt.Errorf("金额格式不正确: %q", s)
	}
	// 补齐到两位："9" -> "90"
	for len(fracPart) < 2 {
		fracPart += "0"
	}

	yuan, err := strconv.ParseInt(intPart, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("金额格式不正确: %q", s)
	}
	cents, err := strconv.ParseInt(fracPart, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("金额格式不正确: %q", s)
	}

	v := Cents(yuan*100 + cents)
	if neg {
		v = -v
	}
	return v, nil
}

// String 输出 "129.90" 这样的两位小数字符串。
func (c Cents) String() string {
	neg := c < 0
	v := int64(c)
	if neg {
		v = -v
	}
	s := fmt.Sprintf("%d.%02d", v/100, v%100)
	if neg {
		return "-" + s
	}
	return s
}

// MarshalJSON 输出字符串而不是数字，避免前端 JS 浮点精度问题。
func (c Cents) MarshalJSON() ([]byte, error) {
	return []byte(`"` + c.String() + `"`), nil
}

// UnmarshalJSON 接受字符串 "129.90"，也兼容数字 129.9（不推荐）。
func (c *Cents) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		*c = 0
		return nil
	}
	v, err := FromYuan(s)
	if err != nil {
		return err
	}
	*c = v
	return nil
}

// Value 实现 driver.Valuer，写入数据库时转成 DECIMAL 字符串。
func (c Cents) Value() (driver.Value, error) {
	return c.String(), nil
}

// Scan 实现 sql.Scanner，从 DECIMAL 读出。
func (c *Cents) Scan(src any) error {
	switch v := src.(type) {
	case nil:
		*c = 0
		return nil
	case []byte:
		parsed, err := FromYuan(string(v))
		if err != nil {
			return err
		}
		*c = parsed
		return nil
	case string:
		parsed, err := FromYuan(v)
		if err != nil {
			return err
		}
		*c = parsed
		return nil
	default:
		return fmt.Errorf("money: 无法解析类型 %T", src)
	}
}

// Mul 按数量放大，用于计算小计。
func (c Cents) Mul(n int) Cents {
	return Cents(int64(c) * int64(n))
}

// Add 相加。
func (c Cents) Add(other Cents) Cents {
	return c + other
}

// IsZero 是否为零。
func (c Cents) IsZero() bool { return c == 0 }

// Positive 是否大于零。
func (c Cents) Positive() bool { return c > 0 }
