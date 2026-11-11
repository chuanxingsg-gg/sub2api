package service

import (
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"strings"
	"unicode/utf8"
)

// PasswordPolicy applies only when setting a password. Existing passwords remain
// usable at login. bcrypt's 72-byte boundary is enforced without truncation.
func ValidateNewPassword(password string) error {
	if !utf8.ValidString(password) || utf8.RuneCountInString(password) < 15 || len(password) > 72 {
		return infraerrors.BadRequest("WEAK_PASSWORD", "新密码至少 15 个字符，且 UTF-8 编码不超过 72 字节；建议使用密码管理器生成长密码")
	}
	normalized := strings.ToLower(strings.TrimSpace(password))
	allSame := true
	var first rune
	for i, r := range password {
		if i == 0 {
			first = r
		} else if r != first {
			allSame = false
		}
	}
	if allSame || strings.Contains("012345678901234567890123456789", normalized) ||
		normalized == "passwordpassword" || normalized == "password123456789" || normalized == "qwertyuiopasdfgh" {
		return infraerrors.BadRequest("WEAK_PASSWORD", "请勿使用常见或重复密码")
	}
	return nil
}
