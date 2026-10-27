package money_test

import (
	"encoding/json"
	"testing"

	"go-mall/pkg/money"
)

func TestFromYuan(t *testing.T) {
	cases := []struct {
		in   string
		want money.Cents
	}{
		{"0", 0},
		{"0.00", 0},
		{"1", 100},
		{"1.5", 150},
		{"129.90", 12990},
		{"0.01", 1},
		{"99999999.99", 9999999999},
		{"-12.34", -1234},
	}
	for _, c := range cases {
		got, err := money.FromYuan(c.in)
		if err != nil {
			t.Fatalf("%q 解析失败: %v", c.in, err)
		}
		if got != c.want {
			t.Fatalf("%q 期望 %d，实际 %d", c.in, c.want, got)
		}
	}
}

func TestFromYuanRejectsInvalid(t *testing.T) {
	for _, in := range []string{"", "abc", "1.234", "1.", "1..2", "1,5"} {
		if _, err := money.FromYuan(in); err == nil {
			t.Fatalf("%q 应该解析失败", in)
		}
	}
}

func TestString(t *testing.T) {
	cases := []struct {
		in   money.Cents
		want string
	}{
		{0, "0.00"},
		{1, "0.01"},
		{150, "1.50"},
		{12990, "129.90"},
		{-1234, "-12.34"},
	}
	for _, c := range cases {
		if got := c.in.String(); got != c.want {
			t.Fatalf("%d 期望 %q，实际 %q", c.in, c.want, got)
		}
	}
}

func TestJSONRoundTrip(t *testing.T) {
	type payload struct {
		Price money.Cents `json:"price"`
	}

	raw, err := json.Marshal(payload{Price: 12990})
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	if string(raw) != `{"price":"129.90"}` {
		t.Fatalf("金额必须以字符串输出，实际 %s", raw)
	}

	var back payload
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("反序列化失败: %v", err)
	}
	if back.Price != 12990 {
		t.Fatalf("往返后金额不一致: %d", back.Price)
	}
}

func TestSQLRoundTrip(t *testing.T) {
	// 模拟数据库返回 DECIMAL 的两种形态
	for _, src := range []any{[]byte("129.90"), "129.90"} {
		var c money.Cents
		if err := c.Scan(src); err != nil {
			t.Fatalf("Scan(%T) 失败: %v", src, err)
		}
		if c != 12990 {
			t.Fatalf("Scan(%T) 期望 12990，实际 %d", src, c)
		}
	}

	v, err := money.Cents(12990).Value()
	if err != nil {
		t.Fatalf("Value 失败: %v", err)
	}
	if v != "129.90" {
		t.Fatalf("Value 期望 129.90，实际 %v", v)
	}
}

func TestArithmetic(t *testing.T) {
	price, _ := money.FromYuan("19.90")
	if got := price.Mul(3); got != 5970 {
		t.Fatalf("小计计算错误: %d", got)
	}
	total := money.Cents(0)
	for i := 0; i < 3; i++ {
		total = total.Add(price)
	}
	if total.String() != "59.70" {
		t.Fatalf("累加错误: %s", total.String())
	}
}
