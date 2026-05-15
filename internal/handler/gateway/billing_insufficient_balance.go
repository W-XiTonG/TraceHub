package gateway

import (
	"fmt"
	"strconv"

	"tracehub/internal/middleware"

	"github.com/gin-gonic/gin"
)

// HTTP 响应头：便于客户端（含浏览器）读取计费预检分解，且不改变 OpenAI 风格 JSON 主体。
const (
	headerBillingPhase          = "X-TraceHub-Billing-Phase"
	headerBalance               = "X-TraceHub-Balance"
	headerEstimatedCost         = "X-TraceHub-Estimated-Cost"
	headerEstPromptTokens       = "X-TraceHub-Estimated-Prompt-Tokens"
	headerEstCompletionTokens   = "X-TraceHub-Estimated-Completion-Tokens"
	headerDebitCost             = "X-TraceHub-Debit-Cost"
)

const (
	billingPhasePrecheck     = "precheck"
	billingPhasePostUpstream = "post_upstream"
)

func balancePrecheckInsufficientMessage(balance, costPre int64, promptToks, compAssumed int) string {
	shortfall := costPre - balance
	if shortfall < 0 {
		shortfall = 0
	}
	var compPhrase string
	if compAssumed <= 0 {
		compPhrase = "输出侧未设置 max_tokens 等上限，按 0 token 计"
	} else {
		compPhrase = fmt.Sprintf(
			"输出按最多 %d token 计（通常来自请求的 max_tokens / max_completion_tokens，或接口默认上限）",
			compAssumed,
		)
	}
	tip := "若仍不足请充值后再试。"
	if compAssumed > 0 {
		tip = "若预检仍偏高，可尝试调低 max_tokens / max_completion_tokens 以降低预检；不足时请充值。"
	}
	return fmt.Sprintf(
		"余额不足（预检）：当前余额 %d，本次预检需 %d（还差 %d）。估算：输入约 %d token，%s。预检不假设 prompt cache；请求成功后将按实际上游用量扣费。%s",
		balance, costPre, shortfall, promptToks, compPhrase, tip,
	)
}

func insufficientBalancePostDebitMessage(balance, cost int64) string {
	shortfall := cost - balance
	if shortfall < 0 {
		shortfall = 0
	}
	return fmt.Sprintf(
		"余额不足：当前余额 %d，无法扣除本次用量 %d（还差 %d）。上游已完成响应时请充值后再试。",
		balance, cost, shortfall,
	)
}

func precheck402Headers(balance, costPre int64, promptToks, compAssumed int) map[string]string {
	return map[string]string{
		headerBillingPhase:        billingPhasePrecheck,
		headerBalance:             strconv.FormatInt(balance, 10),
		headerEstimatedCost:       strconv.FormatInt(costPre, 10),
		headerEstPromptTokens:     strconv.Itoa(promptToks),
		headerEstCompletionTokens: strconv.Itoa(compAssumed),
	}
}

func postDebit402Headers(balance, cost int64) map[string]string {
	return map[string]string{
		headerBillingPhase:  billingPhasePostUpstream,
		headerBalance:       strconv.FormatInt(balance, 10),
		headerDebitCost:     strconv.FormatInt(cost, 10),
		headerEstimatedCost: strconv.FormatInt(cost, 10),
	}
}

// WriteInsufficientBalance402Precheck 网关预检阶段余额不足（402）。
func WriteInsufficientBalance402Precheck(c *gin.Context, balance, costPre int64, promptToks, compAssumed int) {
	msg := balancePrecheckInsufficientMessage(balance, costPre, promptToks, compAssumed)
	middleware.WriteOpenAIErrorWithHeaders(c, 402, "insufficient_balance_error", msg, precheck402Headers(balance, costPre, promptToks, compAssumed))
}

// WriteInsufficientBalance402PostDebit 上游已成功但本地扣费失败（402）。
func WriteInsufficientBalance402PostDebit(c *gin.Context, balance, cost int64) {
	msg := insufficientBalancePostDebitMessage(balance, cost)
	middleware.WriteOpenAIErrorWithHeaders(c, 402, "insufficient_balance_error", msg, postDebit402Headers(balance, cost))
}
