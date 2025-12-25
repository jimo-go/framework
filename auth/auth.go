package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
)

func HashPassword(password string) (string, error) {
	password = strings.TrimSpace(password)
	if password == "" {
		return "", fmt.Errorf("empty password")
	}

	iters := 120000
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	dk := pbkdf2SHA256([]byte(password), salt, iters, 32)
	return fmt.Sprintf("pbkdf2_sha256$%d$%s$%s", iters, base64.RawURLEncoding.EncodeToString(salt), base64.RawURLEncoding.EncodeToString(dk)), nil
}

func CheckPassword(password, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 {
		return false
	}
	if parts[0] != "pbkdf2_sha256" {
		return false
	}
	iters, err := strconv.Atoi(parts[1])
	if err != nil || iters <= 0 {
		return false
	}
	salt, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return false
	}
	expected, err := base64.RawURLEncoding.DecodeString(parts[3])
	if err != nil {
		return false
	}

	dk := pbkdf2SHA256([]byte(password), salt, iters, len(expected))
	return subtle.ConstantTimeCompare(dk, expected) == 1
}

func pbkdf2SHA256(password, salt []byte, iter, keyLen int) []byte {
	hLen := sha256.Size
	numBlocks := (keyLen + hLen - 1) / hLen
	out := make([]byte, 0, numBlocks*hLen)

	buf := make([]byte, 4)
	for block := 1; block <= numBlocks; block++ {
		buf[0] = byte(block >> 24)
		buf[1] = byte(block >> 16)
		buf[2] = byte(block >> 8)
		buf[3] = byte(block)

		mac := hmac.New(sha256.New, password)
		mac.Write(salt)
		mac.Write(buf)
		u := mac.Sum(nil)

		t := make([]byte, len(u))
		copy(t, u)
		for i := 1; i < iter; i++ {
			mac = hmac.New(sha256.New, password)
			mac.Write(u)
			u = mac.Sum(nil)
			for j := 0; j < len(t); j++ {
				t[j] ^= u[j]
			}
		}
		out = append(out, t...)
	}

	return out[:keyLen]
}
