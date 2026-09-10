package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// MakeToken 生成 HMAC 签名的 cookie 值: "userID.expireUnix.signature"
func MakeToken(userID int64, ttl time.Duration, secret string) string {
	expire := time.Now().Add(ttl).Unix()
	payload := fmt.Sprintf("%d.%d", userID, expire)
	return payload + "." + sign(payload, secret)
}

// ParseToken 验证签名与有效期，返回用户 ID。
func ParseToken(token, secret string) (int64, bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return 0, false
	}
	payload := parts[0] + "." + parts[1]
	if !hmac.Equal([]byte(parts[2]), []byte(sign(payload, secret))) {
		return 0, false
	}
	expire, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || time.Now().Unix() > expire {
		return 0, false
	}
	userID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, false
	}
	return userID, true
}

func sign(payload, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}
