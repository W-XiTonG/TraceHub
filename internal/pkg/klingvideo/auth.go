package klingvideo

import (
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	tokenLifetime      = 30 * time.Minute
	tokenCacheLifetime = 25 * time.Minute
)

type tokenCacheEntry struct {
	token string
	exp   time.Time
}

var tokenCache sync.Map

// ParseCredentials 从渠道 api_key 解析 AccessKey 与 SecretKey（格式 AccessKey:SecretKey）。
func ParseCredentials(apiKey string) (accessKey, secretKey string, err error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return "", "", errors.New("empty kling credentials")
	}
	i := strings.Index(apiKey, ":")
	if i <= 0 || i >= len(apiKey)-1 {
		return "", "", errors.New("kling api_key must be AccessKey:SecretKey")
	}
	return strings.TrimSpace(apiKey[:i]), strings.TrimSpace(apiKey[i+1:]), nil
}

// BearerToken 签发可灵 JWT 并缓存（按 AccessKey）。
func BearerToken(apiKey string) (string, error) {
	ak, sk, err := ParseCredentials(apiKey)
	if err != nil {
		return "", err
	}
	if v, ok := tokenCache.Load(ak); ok {
		e := v.(tokenCacheEntry)
		if time.Now().Before(e.exp) {
			return e.token, nil
		}
	}
	token, err := signJWT(ak, sk)
	if err != nil {
		return "", err
	}
	tokenCache.Store(ak, tokenCacheEntry{token: token, exp: time.Now().Add(tokenCacheLifetime)})
	return token, nil
}

func signJWT(accessKey, secretKey string) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"iss": accessKey,
		"exp": now.Add(tokenLifetime).Unix(),
		"nbf": now.Add(-5 * time.Second).Unix(),
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return t.SignedString([]byte(secretKey))
}
