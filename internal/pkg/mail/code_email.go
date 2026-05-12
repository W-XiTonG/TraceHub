package mail

import (
	"fmt"
	"strings"
)

// CodeLocale 表示验证码邮件的语言模式。
type CodeLocale int

const (
	CodeLocaleZHCN CodeLocale = iota
	CodeLocaleZHTW
	CodeLocaleEN
	// CodeLocaleAll 邮件正文同时包含简中、繁中、英文三段。
	CodeLocaleAll
)

func operatorOrDefault(name string) string {
	s := strings.TrimSpace(name)
	if s != "" {
		return s
	}
	return "循迹未来"
}

// ParseCodeLocale 解析前端或 API 传入的 locale（与 vue-i18n 一致：zh-CN、zh-TW、en），
// 以及可选值 all / trilingual。forceTrilingual 为 true 时（配置项）固定返回 CodeLocaleAll。
func ParseCodeLocale(raw string, forceTrilingual bool) CodeLocale {
	if forceTrilingual {
		return CodeLocaleAll
	}
	l := strings.ToLower(strings.TrimSpace(raw))
	switch l {
	case "all", "triple", "trilingual":
		return CodeLocaleAll
	case "en", "en-us", "en-gb":
		return CodeLocaleEN
	case "zh-tw", "zh-hant", "zh-hk", "zh-mo":
		return CodeLocaleZHTW
	case "zh-cn", "zh-hans", "zh":
		return CodeLocaleZHCN
	case "":
		return CodeLocaleZHCN
	default:
		return CodeLocaleZHCN
	}
}

// PickLocaleFromAcceptLanguage 在请求未带 locale 时根据 Accept-Language 粗判。
func PickLocaleFromAcceptLanguage(h string) string {
	h = strings.TrimSpace(h)
	if h == "" {
		return "zh-CN"
	}
	first := strings.TrimSpace(strings.Split(h, ",")[0])
	first = strings.TrimSpace(strings.Split(first, ";")[0])
	low := strings.ToLower(first)
	if strings.HasPrefix(low, "zh-tw") || strings.HasPrefix(low, "zh-hk") || strings.HasPrefix(low, "zh-mo") || low == "zh-hant" {
		return "zh-TW"
	}
	if strings.HasPrefix(low, "zh") {
		return "zh-CN"
	}
	if strings.HasPrefix(low, "en") {
		return "en"
	}
	return "zh-CN"
}

// LoginCodeEmail 返回登录验证码纯文本邮件的主题与正文。
func LoginCodeEmail(loc CodeLocale, operator, code string, ttlMin int, subjectZHOverride string) (subject, body string) {
	op := operatorOrDefault(operator)
	switch loc {
	case CodeLocaleEN:
		subject = "TraceHub login verification code"
		body = fmt.Sprintf(
			"Your TraceHub login verification code is: %s\n\nIt is valid for %d minutes. If you did not request this email, you can safely ignore it.\n\n— TraceHub · %s\n",
			code, ttlMin, op,
		)
	case CodeLocaleZHTW:
		subject = fmt.Sprintf("%s · TraceHub 登入驗證碼", op)
		body = fmt.Sprintf(
			"您的登入驗證碼為：%s\n\n%d 分鐘內有效。若非本人操作，請忽略本郵件。\n",
			code, ttlMin,
		)
	case CodeLocaleAll:
		subject = "TraceHub login code · 登录验证码 · 登入驗證碼"
		body = loginCodeBodyTrilingual(op, code, ttlMin, false)
	default: // zh-CN
		if strings.TrimSpace(subjectZHOverride) != "" {
			subject = strings.TrimSpace(subjectZHOverride)
		} else {
			subject = fmt.Sprintf("%s · TraceHub 登录验证码", op)
		}
		body = fmt.Sprintf(
			"您的登录验证码为：%s\n\n%d 分钟内有效。如非本人操作请忽略本邮件。\n",
			code, ttlMin,
		)
	}
	return subject, body
}

// RegisterCodeEmail 返回注册验证码纯文本邮件的主题与正文。
func RegisterCodeEmail(loc CodeLocale, operator, code string, ttlMin int, subjectZHOverride string) (subject, body string) {
	op := operatorOrDefault(operator)
	switch loc {
	case CodeLocaleEN:
		subject = "TraceHub registration verification code"
		body = fmt.Sprintf(
			"Your TraceHub registration verification code is: %s\n\nIt is valid for %d minutes. If you did not start registration, you can safely ignore this email.\n\n— TraceHub · %s\n",
			code, ttlMin, op,
		)
	case CodeLocaleZHTW:
		subject = fmt.Sprintf("%s · TraceHub 註冊驗證碼", op)
		body = fmt.Sprintf(
			"您的註冊驗證碼為：%s\n\n%d 分鐘內有效。若非本人操作，請忽略本郵件。\n",
			code, ttlMin,
		)
	case CodeLocaleAll:
		subject = "TraceHub registration code · 注册验证码 · 註冊驗證碼"
		body = loginCodeBodyTrilingual(op, code, ttlMin, true)
	default:
		if strings.TrimSpace(subjectZHOverride) != "" {
			subject = strings.TrimSpace(subjectZHOverride)
		} else {
			subject = fmt.Sprintf("%s · TraceHub 注册验证码", op)
		}
		body = fmt.Sprintf(
			"您的注册验证码为：%s\n\n%d 分钟内有效。如非本人操作请忽略本邮件。\n",
			code, ttlMin,
		)
	}
	return subject, body
}

func loginCodeBodyTrilingual(op, code string, ttlMin int, register bool) string {
	var zhCNIntro, zhTWIntro, enIntro string
	if register {
		zhCNIntro = "您的注册验证码为："
		zhTWIntro = "您的註冊驗證碼為："
		enIntro = "Your TraceHub registration verification code is: "
	} else {
		zhCNIntro = "您的登录验证码为："
		zhTWIntro = "您的登入驗證碼為："
		enIntro = "Your TraceHub login verification code is: "
	}
	enSuffix := "If you did not request this, you can safely ignore this email."
	if register {
		enSuffix = "If you did not start registration, you can safely ignore this email."
	}
	return fmt.Sprintf(`==== English ====
%s%s

This code is valid for %d minutes. %s

— TraceHub · %s

==== 简体中文 ====
%s%s

%d 分钟内有效。如非本人操作请忽略本邮件。

==== 繁體中文 ====
%s%s

%d 分鐘內有效。若非本人操作，請忽略本郵件。
`,
		enIntro, code, ttlMin, enSuffix, op,
		zhCNIntro, code, ttlMin,
		zhTWIntro, code, ttlMin,
	)
}
