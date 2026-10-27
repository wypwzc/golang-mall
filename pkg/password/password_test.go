package password_test

import (
	"strings"
	"testing"

	"go-mall/pkg/password"
)

func TestHashAndVerify(t *testing.T) {
	hash, err := password.Hash("alice123456")
	if err != nil {
		t.Fatalf("生成哈希失败: %v", err)
	}
	if strings.Contains(hash, "alice123456") {
		t.Fatal("哈希里不能出现明文")
	}
	if !password.Verify(hash, "alice123456") {
		t.Fatal("正确密码应校验通过")
	}
	if password.Verify(hash, "alice1234567") {
		t.Fatal("错误密码必须校验失败")
	}
}

func TestHashIsSalted(t *testing.T) {
	h1, _ := password.Hash("same-password")
	h2, _ := password.Hash("same-password")
	if h1 == h2 {
		t.Fatal("同一密码两次哈希必须不同（bcrypt 自带随机盐）")
	}
	if !password.Verify(h1, "same-password") || !password.Verify(h2, "same-password") {
		t.Fatal("两个哈希都应能校验通过")
	}
}

func TestHashRejectsTooLong(t *testing.T) {
	long := strings.Repeat("a", 73)
	if _, err := password.Hash(long); err == nil {
		t.Fatal("超过 72 字节应报错，避免被 bcrypt 静默截断")
	}
}
