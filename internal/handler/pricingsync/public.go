package pricingsynchandler

import (
	"io"
	"strings"

	"tracehub/internal/ent"
	"tracehub/internal/pkg/httpx"
	"tracehub/internal/pkg/sitecfg"
	"tracehub/internal/service/pricingsync"

	"github.com/gin-gonic/gin"
)

type PublicDeps struct {
	Client *ent.Client
}

func bearerFromRequest(c *gin.Context) string {
	h := c.GetHeader("Authorization")
	if !strings.HasPrefix(strings.ToLower(h), "bearer ") {
		return ""
	}
	return strings.TrimSpace(h[7:])
}

func requireInboundToken(c *gin.Context, client *ent.Client) bool {
	if !sitecfg.EffectivePricingSyncEnabled(c.Request.Context(), client) {
		httpx.Forbidden(c, "价格同步功能未开启")
		return false
	}
	tok := bearerFromRequest(c)
	if !sitecfg.ValidateInboundSyncToken(c.Request.Context(), client, tok) {
		httpx.Unauthorized(c, "无效或缺失同步密钥")
		return false
	}
	return true
}

// ExportInbound GET /api/pricing/sync/export — 供外部 Pull 本站点定价。
func ExportInbound(d PublicDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireInboundToken(c, d.Client) {
			return
		}
		t := &ent.ModelPricingSyncTarget{Scopes: []string{"global", "channel", "group"}}
		payload, err := pricingsync.ExportLocal(c.Request.Context(), d.Client, t)
		if err != nil {
			httpx.ServerError(c, "导出失败")
			return
		}
		httpx.OK(c, payload)
	}
}

// ImportInbound POST /api/pricing/sync/import — 供外部 Push 定价到本站点。
func ImportInbound(d PublicDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireInboundToken(c, d.Client) {
			return
		}
		body, err := io.ReadAll(io.LimitReader(c.Request.Body, 8<<20))
		if err != nil {
			httpx.BadRequest(c, "读取请求体失败")
			return
		}
		p, err := pricingsync.ParsePayloadJSON(body)
		if err != nil {
			httpx.BadRequest(c, err.Error())
			return
		}
		added, updated, warnings, err := pricingsync.ApplyInboundPayload(c.Request.Context(), d.Client, p)
		if err != nil {
			httpx.ServerError(c, err.Error())
			return
		}
		httpx.OK(c, gin.H{
			"ok":       true,
			"added":    added,
			"updated":  updated,
			"warnings": warnings,
		})
	}
}
