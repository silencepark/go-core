// Package jwt 提供 JWT 签发与校验的通用基础设施。
//
// TokenManager 不预设业务字段——调用方自行定义 Claims 结构体（嵌入 jwt.RegisteredClaims），
// 通过 SignToken / ParseToken 签发与解析。
//
// 典型用法：
//
//	type MyClaims struct {
//	    UserID   uint64 `json:"user_id"`
//	    Username string `json:"username"`
//	    jwt.RegisteredClaims
//	}
//
//	tm := jwt.NewTokenManager(cfg)
//
//	// 签发
//	claims := MyClaims{
//	    UserID: 42,
//	    RegisteredClaims: tm.NewAccessClaims(),
//	}
//	token, _ := tm.SignToken(claims)
//
//	// 解析
//	var parsed MyClaims
//	_ = tm.ParseToken(token, &parsed)
package jwt

import (
	"errors"
	"fmt"
	"time"

	jwtlib "github.com/golang-jwt/jwt/v5"

	"github.com/silencepark/go-core/config"
)

// TokenManager 管理 JWT 签发与校验。不预设业务字段，Claims 由调用方定义。
type TokenManager struct {
	secret       []byte
	appName      string
	expireHours  int
	refreshHours int
}

// NewTokenManager 创建 TokenManager，从 Config 提取所需字段。
func NewTokenManager(cfg *config.JWTConfig, appName string) *TokenManager {
	return &TokenManager{
		secret:       []byte(cfg.Secret),
		appName:      appName,
		expireHours:  cfg.ExpireHours,
		refreshHours: cfg.RefreshHours,
	}
}

// NewAccessClaims 创建填充了 ExpiresAt / IssuedAt / Issuer 的 RegisteredClaims，
// 调用方在自己的 Claims 结构体中嵌入此返回值即可。
func (tm *TokenManager) NewAccessClaims() jwtlib.RegisteredClaims {
	now := time.Now()
	return jwtlib.RegisteredClaims{
		ExpiresAt: jwtlib.NewNumericDate(now.Add(time.Duration(tm.expireHours) * time.Hour)),
		IssuedAt:  jwtlib.NewNumericDate(now),
		Issuer:    tm.appName,
	}
}

// SignToken 签名任意 jwt.Claims 并返回 token 字符串。
func (tm *TokenManager) SignToken(claims jwtlib.Claims) (string, error) {
	token, err := jwtlib.NewWithClaims(jwtlib.SigningMethodHS256, claims).SignedString(tm.secret)
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}
	return token, nil
}

// ParseToken 解析并校验 token，将结果写入 claims（必须为指针类型）。
// 会校验签名算法为 HS256，防止算法混淆攻击。
func (tm *TokenManager) ParseToken(tokenString string, claims jwtlib.Claims) error {
	_, err := jwtlib.ParseWithClaims(tokenString, claims, func(token *jwtlib.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwtlib.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return tm.secret, nil
	})
	if err != nil {
		return fmt.Errorf("parse token: %w", err)
	}
	return nil
}

// SignRefreshToken 创建 refresh token，subject 通常为用户 ID。
func (tm *TokenManager) SignRefreshToken(subject string) (string, error) {
	now := time.Now()
	claims := jwtlib.RegisteredClaims{
		ExpiresAt: jwtlib.NewNumericDate(now.Add(time.Duration(tm.refreshHours) * time.Hour)),
		IssuedAt:  jwtlib.NewNumericDate(now),
		Issuer:    tm.appName,
		Subject:   subject,
	}
	return tm.SignToken(claims)
}

// ParseRefreshToken 解析 refresh token 并返回 subject。
func (tm *TokenManager) ParseRefreshToken(tokenString string) (string, error) {
	claims := &jwtlib.RegisteredClaims{}
	if err := tm.ParseToken(tokenString, claims); err != nil {
		return "", fmt.Errorf("parse refresh token: %w", err)
	}
	if claims.Subject == "" {
		return "", errors.New("refresh token missing subject")
	}
	return claims.Subject, nil
}

// RefreshTTL 返回 refresh token 有效期。
func (tm *TokenManager) RefreshTTL() time.Duration {
	return time.Duration(tm.refreshHours) * time.Hour
}

// AccessTTL 返回 access token 有效期秒数。
func (tm *TokenManager) AccessTTL() int {
	return tm.expireHours * 3600
}
