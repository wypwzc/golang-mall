// Package password 密码哈希。
//
// 使用 bcrypt：自带随机盐、计算慢，即使数据库泄漏也难以离线爆破。
// 绝不能用 MD5/SHA1 这类快速哈希存密码。
package password

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// cost 越大越安全但越慢。10 是常见折中值，单次校验约几十毫秒。
const cost = 10

// bcrypt 只处理前 72 字节，超长密码会被静默截断，因此这里直接拒绝。
const maxLength = 72

// Hash 生成哈希。
func Hash(plain string) (string, error) {
	if len(plain) > maxLength {
		return "", fmt.Errorf("密码过长（最多 %d 字节）", maxLength)
	}
	b, err := bcrypt.GenerateFromPassword([]byte(plain), cost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Verify 校验密码是否匹配。
// 故意不区分"用户不存在"和"密码错误"，避免通过响应差异枚举账号。
func Verify(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}
