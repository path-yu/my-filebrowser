package users

import (
	"crypto/rand"
	"encoding/base64"

	"golang.org/x/crypto/bcrypt"

	fberrors "github.com/filebrowser/filebrowser/v2/errors"
)

// ValidateAndHashPwd validates and hashes a password.
func ValidateAndHashPwd(password string, minimumLength uint) (string, error) {
	if uint(len(password)) < minimumLength {
		return "", fberrors.ErrShortPassword{MinimumLength: minimumLength}
	}

	// 定制：移除弱密码黑名单校验（common passwords），
	// 允许使用 123456 等简单密码（内网文件管理系统场景）。
	return HashPwd(password)
}

// HashPwd hashes a password.
func HashPwd(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// CheckPwd checks if a password is correct.
func CheckPwd(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func RandomPwd(passwordLength uint) (string, error) {
	randomPasswordBytes := make([]byte, passwordLength)
	var _, err = rand.Read(randomPasswordBytes)
	if err != nil {
		return "", err
	}

	// This is done purely to make the password human-readable
	var randomPasswordString = base64.URLEncoding.EncodeToString(randomPasswordBytes)
	return randomPasswordString, nil
}
