package userhandler

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"strings"
	"time"

	"tracehub/internal/ent"
	"tracehub/internal/ent/user"
	"tracehub/internal/pkg/config"
	"tracehub/internal/pkg/httpx"
	"tracehub/internal/pkg/mail"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

const (
	redisKeyLoginCode   = "tracehub:login_code:%s"
	redisKeyLoginCD     = "tracehub:login_code_cd:%s"
	redisKeyLoginRateH  = "tracehub:login_code_h:%d:%s"
	redisKeyLoginVerify = "tracehub:login_code_try:%s"
)

func randomDigits6() string {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%06d", time.Now().UnixNano()%1000000)
	}
	n := binary.BigEndian.Uint32(b[:]) % 1000000
	return fmt.Sprintf("%06d", n)
}

func loginCodeRedisKeys(email string) (codeKey, cdKey, tryKey string, hourKey string) {
	em := strings.ToLower(strings.TrimSpace(email))
	h := time.Now().Unix() / 3600
	return fmt.Sprintf(redisKeyLoginCode, em),
		fmt.Sprintf(redisKeyLoginCD, em),
		fmt.Sprintf(redisKeyLoginVerify, em),
		fmt.Sprintf(redisKeyLoginRateH, h, em)
}

// SendLoginCode 向已注册邮箱发送登录验证码（需启用邮件且 Redis 可用）。
func SendLoginCode(client *ent.Client, cfg *config.Config, rdb *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		if rdb == nil {
			httpx.ServiceUnavailable(c, "未连接 Redis，无法使用邮箱验证码登录")
			return
		}
		if !cfg.Email.Enabled {
			httpx.ServiceUnavailable(c, "未启用邮件发送，请使用密码登录或联系管理员配置 email")
			return
		}
		if cfg.Email.SMTPHost == "" || cfg.Email.From == "" {
			httpx.ServiceUnavailable(c, "邮件 SMTP 未配置完整")
			return
		}

		var req struct {
			Email  string `json:"email"`
			Locale string `json:"locale"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			httpx.BadRequest(c, "参数无效")
			return
		}
		email := strings.TrimSpace(req.Email)
		if email == "" || !strings.Contains(email, "@") {
			httpx.BadRequest(c, "请输入有效邮箱")
			return
		}

		ctx := c.Request.Context()
		codeKey, cdKey, tryKey, hourKey := loginCodeRedisKeys(email)

		exists, err := rdb.Exists(ctx, cdKey).Result()
		if err != nil {
			httpx.ServerError(c, "服务暂不可用")
			return
		}
		if exists > 0 {
			ttl, _ := rdb.TTL(ctx, cdKey).Result()
			sec := int(ttl.Seconds())
			if sec < 0 {
				sec = 0
			}
			httpx.TooManyRequests(c, fmt.Sprintf("发送过于频繁，请 %d 秒后再试", sec))
			return
		}

		u, err := client.User.Query().Where(user.EmailEQ(email)).Only(ctx)
		if err != nil {
			if ent.IsNotFound(err) {
				httpx.BadRequest(c, "该邮箱未注册，请先注册后再使用验证码登录")
				return
			}
			httpx.ServerError(c, "服务暂不可用")
			return
		}
		if u.Status != 1 {
			httpx.BadRequest(c, "账号已停用，无法使用验证码登录")
			return
		}

		maxH := cfg.Email.LoginCodeMaxHour
		if maxH <= 0 {
			maxH = 8
		}
		n, err := rdb.Incr(ctx, hourKey).Result()
		if err != nil {
			httpx.ServerError(c, "服务暂不可用")
			return
		}
		if n == 1 {
			_ = rdb.Expire(ctx, hourKey, 3660*time.Second).Err()
		}
		if n > int64(maxH) {
			_, _ = rdb.Decr(ctx, hourKey).Result()
			httpx.TooManyRequests(c, "本小时发送次数已达上限，请稍后再试")
			return
		}

		code := randomDigits6()
		ttlMin := cfg.Email.LoginCodeTTLMin
		if ttlMin <= 0 {
			ttlMin = 10
		}
		resend := cfg.Email.LoginCodeResendSec
		if resend <= 0 {
			resend = 60
		}

		if err := rdb.Set(ctx, codeKey, code, time.Duration(ttlMin)*time.Minute).Err(); err != nil {
			httpx.ServerError(c, "服务暂不可用")
			return
		}
		_ = rdb.Set(ctx, cdKey, "1", time.Duration(resend)*time.Second).Err()
		_ = rdb.Del(ctx, tryKey).Err()

		localeRaw := strings.TrimSpace(req.Locale)
		if localeRaw == "" {
			localeRaw = mail.PickLocaleFromAcceptLanguage(c.GetHeader("Accept-Language"))
		}
		loc := mail.ParseCodeLocale(localeRaw, cfg.Email.CodeEmailTrilingual)
		subj, body := mail.LoginCodeEmail(loc, cfg.Site.OperatorDisplayName, code, ttlMin, cfg.Email.LoginCodeSubject)
		if err := mail.SendPlain(&cfg.Email, []string{email}, subj, body); err != nil {
			_, _ = rdb.Del(ctx, codeKey, cdKey).Result()
			_, _ = rdb.Decr(ctx, hourKey).Result()
			httpx.ServerError(c, "邮件发送失败，请稍后重试或改用密码登录")
			return
		}

		httpx.OKMsg(c, "验证码已发送", gin.H{"next_resend_sec": resend})
	}
}

// VerifyLoginCode 校验邮箱验证码并签发与密码登录相同的令牌。
func VerifyLoginCode(client *ent.Client, cfg *config.Config, jwtSecret []byte, rdb *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		if rdb == nil {
			httpx.ServiceUnavailable(c, "未连接 Redis，无法使用邮箱验证码登录")
			return
		}
		if !cfg.Email.Enabled {
			httpx.ServiceUnavailable(c, "未启用邮件验证码登录")
			return
		}

		var req struct {
			Email string `json:"email"`
			Code  string `json:"code"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			httpx.BadRequest(c, "参数无效")
			return
		}
		email := strings.TrimSpace(req.Email)
		code := strings.TrimSpace(req.Code)
		if email == "" || code == "" {
			httpx.BadRequest(c, "邮箱与验证码必填")
			return
		}
		if len(code) != 6 {
			httpx.BadRequest(c, "验证码为 6 位数字")
			return
		}

		ctx := c.Request.Context()
		codeKey, _, tryKey, _ := loginCodeRedisKeys(email)

		tryN, _ := rdb.Incr(ctx, tryKey).Result()
		if tryN == 1 {
			_ = rdb.Expire(ctx, tryKey, 15*time.Minute).Err()
		}
		if tryN > 8 {
			_, _ = rdb.Del(ctx, codeKey, tryKey).Result()
			httpx.TooManyRequests(c, "验证尝试过多，请重新获取验证码")
			return
		}

		stored, err := rdb.Get(ctx, codeKey).Result()
		if err == redis.Nil {
			httpx.Unauthorized(c, "验证码错误或已过期")
			return
		}
		if err != nil {
			httpx.ServerError(c, "服务暂不可用")
			return
		}
		if stored != code {
			httpx.Unauthorized(c, "验证码错误或已过期")
			return
		}
		_, _ = rdb.Del(ctx, codeKey, tryKey).Result()

		u, err := client.User.Query().Where(user.EmailEQ(email)).WithGroup().Only(ctx)
		if err != nil || u.Status != 1 {
			httpx.Unauthorized(c, "账号不可用")
			return
		}

		writeUserSession(c, client, u, jwtSecret)
	}
}
