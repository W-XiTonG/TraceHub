package userhandler

import (
	"context"
	"strings"
	"time"

	"tracehub/internal/ent"
	"tracehub/internal/pkg/config"
	"tracehub/internal/pkg/httpx"
	"tracehub/internal/pkg/sitecfg"

	"github.com/gin-gonic/gin"
	"github.com/mojocn/base64Captcha"
	"github.com/redis/go-redis/v9"
)

// 原子 GET 并删除；避免依赖 Redis 6.2+ 的 GETDEL（旧版 Redis 会报 unknown command 导致校验永远失败）。
const luaCaptchaGetDel = `
local v = redis.call('GET', KEYS[1])
if v == false then return nil end
redis.call('DEL', KEYS[1])
return v
`

const redisKeyLoginCaptchaPrefix = "tracehub:login_captcha:"

type redisCaptchaStore struct {
	rdb *redis.Client
	ttl time.Duration
}

func loginCaptchaRedisKey(id string) string {
	return redisKeyLoginCaptchaPrefix + id
}

// normalizeLoginCaptchaAnswer 将全角/阿拉伯数字及中间空白规整为半角连续数字，与 base64Captcha 存库的 ASCII 答案比对。
func normalizeLoginCaptchaAnswer(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "\ufeff")
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r >= '０' && r <= '９': // U+FF10–FF19
			b.WriteRune('0' + (r - '０'))
		case r >= '٠' && r <= '٩': // U+0660–U+0669
			b.WriteRune('0' + (r - '٠'))
		case r >= '۰' && r <= '۹': // U+06F0–U+06F9
			b.WriteRune('0' + (r - '۰'))
		}
	}
	return b.String()
}

func loginCaptchaTTL(cfg *config.Config) time.Duration {
	sec := cfg.Site.LoginCaptcha.TTLSeconds
	if sec <= 0 {
		return 5 * time.Minute
	}
	return time.Duration(sec) * time.Second
}

func (s *redisCaptchaStore) Set(id string, value string) error {
	ctx := context.Background()
	return s.rdb.Set(ctx, loginCaptchaRedisKey(id), value, s.ttl).Err()
}

func (s *redisCaptchaStore) Get(id string, clear bool) string {
	ctx := context.Background()
	key := loginCaptchaRedisKey(id)
	if clear {
		v, err := s.rdb.Eval(ctx, luaCaptchaGetDel, []string{key}).Result()
		if err != nil {
			return ""
		}
		if v == nil {
			return ""
		}
		switch x := v.(type) {
		case string:
			return x
		case []byte:
			return string(x)
		default:
			return ""
		}
	}
	v, err := s.rdb.Get(ctx, key).Result()
	if err != nil {
		return ""
	}
	return v
}

func (s *redisCaptchaStore) Verify(id, answer string, clear bool) bool {
	if id == "" || answer == "" {
		return false
	}
	vv := s.Get(id, clear)
	vv = strings.TrimSpace(vv)
	answer = strings.TrimSpace(answer)
	return strings.EqualFold(vv, answer)
}

func newLoginCaptcha(cfg *config.Config, rdb *redis.Client) *base64Captcha.Captcha {
	store := &redisCaptchaStore{rdb: rdb, ttl: loginCaptchaTTL(cfg)}
	driver := base64Captcha.NewDriverDigit(60, 160, 4, 0.65, 60)
	return base64Captcha.NewCaptcha(driver, store)
}

// LoginCaptchaIssue GET /api/user/login-captcha — 未开启时返回 enabled:false；开启时需 Redis。
func LoginCaptchaIssue(client *ent.Client, cfg *config.Config, rdb *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !sitecfg.EffectiveLoginCaptchaEnabled(c.Request.Context(), client, cfg) {
			httpx.OK(c, gin.H{"enabled": false})
			return
		}
		if rdb == nil {
			httpx.ServiceUnavailable(c, "请先连接 Redis，或关闭登录图形验证码（配置文件 site.login_captcha.enabled 或管理后台 site.login_captcha.enabled）")
			return
		}
		c.Header("Cache-Control", "no-store")
		cap := newLoginCaptcha(cfg, rdb)
		id, b64, _, err := cap.Generate()
		if err != nil {
			httpx.ServerError(c, "生成验证码失败")
			return
		}
		httpx.OK(c, gin.H{
			"enabled":       true,
			"captcha_id":    id,
			"image_base64": b64,
		})
	}
}

// loginCaptchaOK 未开启时直接通过；开启时校验并一次性消费 Redis 中的答案。
func loginCaptchaOK(c *gin.Context, client *ent.Client, cfg *config.Config, rdb *redis.Client, captchaID, captchaAnswer string) bool {
	if !sitecfg.EffectiveLoginCaptchaEnabled(c.Request.Context(), client, cfg) {
		return true
	}
	if rdb == nil {
		httpx.ServiceUnavailable(c, "登录图形验证码需要 Redis，请连接 Redis 或在管理后台/配置文件中关闭登录图形验证码")
		return false
	}
	cid := strings.TrimSpace(strings.TrimPrefix(captchaID, "\ufeff"))
	cans := normalizeLoginCaptchaAnswer(captchaAnswer)
	if cid == "" || cans == "" {
		httpx.BadRequest(c, "请填写图形验证码")
		return false
	}
	cap := newLoginCaptcha(cfg, rdb)
	if !cap.Verify(cid, cans, true) {
		httpx.Unauthorized(c, "图形验证码错误或已失效，请刷新后重试")
		return false
	}
	return true
}
