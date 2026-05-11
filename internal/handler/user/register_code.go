package userhandler

import (
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
	redisKeyRegisterCode   = "tracehub:register_code:%s"
	redisKeyRegisterCD     = "tracehub:register_code_cd:%s"
	redisKeyRegisterRateH  = "tracehub:register_code_h:%d:%s"
	redisKeyRegisterVerify = "tracehub:register_code_try:%s"
)

// registerCanSendRegisterCode 为 true 时可发送注册验证码（Redis + SMTP 就绪）。
func registerCanSendRegisterCode(cfg *config.Config, rdb *redis.Client) bool {
	if rdb == nil || !cfg.Email.Enabled {
		return false
	}
	return strings.TrimSpace(cfg.Email.SMTPHost) != "" && strings.TrimSpace(cfg.Email.From) != ""
}

func registerCodeRedisKeys(email string) (codeKey, cdKey, tryKey, hourKey string) {
	em := strings.ToLower(strings.TrimSpace(email))
	h := time.Now().Unix() / 3600
	return fmt.Sprintf(redisKeyRegisterCode, em),
		fmt.Sprintf(redisKeyRegisterCD, em),
		fmt.Sprintf(redisKeyRegisterVerify, em),
		fmt.Sprintf(redisKeyRegisterRateH, h, em)
}

// RegisterEmailCodeMeta 供前端展示验证码 UI：required=启用邮件验证注册；can_send=可调用发送接口。
func RegisterEmailCodeMeta(cfg *config.Config, rdb *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		redisOK := rdb != nil
		smtpReady := strings.TrimSpace(cfg.Email.SMTPHost) != "" && strings.TrimSpace(cfg.Email.From) != ""
		httpx.OK(c, gin.H{
			"required":   cfg.Email.Enabled,
			"can_send":   registerCanSendRegisterCode(cfg, rdb),
			"redis_ok":   redisOK,
			"smtp_ready": smtpReady,
		})
	}
}

// SendRegisterCode 向未注册邮箱发送注册验证码。
func SendRegisterCode(client *ent.Client, cfg *config.Config, rdb *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !registerCanSendRegisterCode(cfg, rdb) {
			httpx.ServiceUnavailable(c, "未启用邮件验证注册或未连接 Redis / SMTP 未配置完整")
			return
		}

		var req struct {
			Email string `json:"email"`
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
		taken, err := client.User.Query().Where(user.EmailEQ(email)).Exist(ctx)
		if err != nil {
			httpx.ServerError(c, "服务暂不可用")
			return
		}
		if taken {
			httpx.BadRequest(c, "该邮箱已注册")
			return
		}

		codeKey, cdKey, tryKey, hourKey := registerCodeRedisKeys(email)

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

		subj := cfg.Email.RegisterCodeSubject
		if subj == "" {
			subj = "TraceHub 注册验证码"
		}
		body := fmt.Sprintf("您的注册验证码为：%s\n\n%d 分钟内有效。如非本人操作请忽略本邮件。\n", code, ttlMin)
		if err := mail.SendPlain(&cfg.Email, []string{email}, subj, body); err != nil {
			_, _ = rdb.Del(ctx, codeKey, cdKey).Result()
			_, _ = rdb.Decr(ctx, hourKey).Result()
			httpx.ServerError(c, "邮件发送失败，请稍后重试")
			return
		}

		httpx.OKMsg(c, "验证码已发送", gin.H{"next_resend_sec": resend})
	}
}

// verifyAndConsumeRegisterCode 校验并删除注册验证码；失败时已写入 HTTP 响应。
func verifyAndConsumeRegisterCode(c *gin.Context, rdb *redis.Client, email, code string) bool {
	ctx := c.Request.Context()
	codeKey, _, tryKey, _ := registerCodeRedisKeys(email)
	code = strings.TrimSpace(code)

	tryN, err := rdb.Incr(ctx, tryKey).Result()
	if err != nil {
		httpx.ServerError(c, "服务暂不可用")
		return false
	}
	if tryN == 1 {
		_ = rdb.Expire(ctx, tryKey, 15*time.Minute).Err()
	}
	if tryN > 8 {
		_, _ = rdb.Del(ctx, codeKey, tryKey).Result()
		httpx.TooManyRequests(c, "验证尝试过多，请重新获取验证码")
		return false
	}

	stored, err := rdb.Get(ctx, codeKey).Result()
	if err == redis.Nil {
		httpx.Unauthorized(c, "验证码错误或已过期")
		return false
	}
	if err != nil {
		httpx.ServerError(c, "服务暂不可用")
		return false
	}
	if stored != code {
		httpx.Unauthorized(c, "验证码错误或已过期")
		return false
	}
	_, _ = rdb.Del(ctx, codeKey, tryKey).Result()
	return true
}
