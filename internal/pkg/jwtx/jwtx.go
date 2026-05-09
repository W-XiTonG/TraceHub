package jwtx

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID int64  `json:"uid"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

func SignAccess(secret []byte, userID int64, email, role string, ttl time.Duration) (string, error) {
	now := time.Now()
	c := Claims{
		UserID: userID,
		Email:  email,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(now),
			Subject:   email,
		},
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
	return t.SignedString(secret)
}

func SignRefresh(secret []byte, userID int64, email string, ttl time.Duration) (string, error) {
	now := time.Now()
	c := jwt.MapClaims{
		"typ": "refresh",
		"uid": userID,
		"sub": email,
		"exp": now.Add(ttl).Unix(),
		"iat": now.Unix(),
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
	return t.SignedString(secret)
}

func ParseAccess(secret []byte, token string) (*Claims, error) {
	var c Claims
	t, err := jwt.ParseWithClaims(token, &c, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return secret, nil
	})
	if err != nil {
		return nil, err
	}
	if !t.Valid {
		return nil, errors.New("invalid token")
	}
	return &c, nil
}

func ParseRefresh(secret []byte, token string) (userID int64, email string, err error) {
	mc := jwt.MapClaims{}
	t, err := jwt.ParseWithClaims(token, mc, func(t *jwt.Token) (any, error) {
		return secret, nil
	})
	if err != nil || !t.Valid {
		return 0, "", errors.New("invalid refresh token")
	}
	if typ, _ := mc["typ"].(string); typ != "refresh" {
		return 0, "", errors.New("not refresh token")
	}
	v, ok := mc["uid"].(float64)
	if !ok {
		return 0, "", errors.New("missing uid")
	}
	em, _ := mc["sub"].(string)
	return int64(v), em, nil
}
