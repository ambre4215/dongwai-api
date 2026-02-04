package utils

import (
	"golang.org/x/crypto/bcrypt"
)

// HashPassword 生成密码哈希
func HashPassword(password string) (string, error) {
	// 建议：将 cost 从 16 降到 10 或 12。
	// 16 对服务器压力非常大，验证一次可能需要几秒钟，导致前端超时。
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	return string(bytes), err
}

// CheckPassword 验证密码
func CheckPassword(password, hash string) bool {
	// 🔴 修复前 (错误): bcrypt.CompareHashAndPassword([]byte(password), []byte(hash))
	// 🟢 修复后 (正确): 第一个参数必须是 hash，第二个是 password
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}
