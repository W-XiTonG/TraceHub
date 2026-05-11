package userhandler

import (
	"time"

	"tracehub/internal/ent"
	"tracehub/internal/pkg/httpx"
	"tracehub/internal/pkg/jwtx"

	"github.com/gin-gonic/gin"
)

func writeUserSession(c *gin.Context, client *ent.Client, u *ent.User, jwtSecret []byte) {
	ctx := c.Request.Context()
	_ = client.User.UpdateOneID(u.ID).SetLastLoginAt(time.Now()).Exec(ctx)

	accessTTL := 72 * time.Hour
	refreshTTL := 30 * 24 * time.Hour
	at, err := jwtx.SignAccess(jwtSecret, int64(u.ID), u.Email, u.Role, accessTTL)
	if err != nil {
		httpx.ServerError(c, "签发令牌失败")
		return
	}
	rt, err := jwtx.SignRefresh(jwtSecret, int64(u.ID), u.Email, refreshTTL)
	if err != nil {
		httpx.ServerError(c, "签发令牌失败")
		return
	}
	gname := ""
	if u.Edges.Group != nil {
		gname = u.Edges.Group.Name
	}
	httpx.OK(c, gin.H{
		"access_token":  at,
		"refresh_token": rt,
		"expires_in":    int(accessTTL.Seconds()),
		"user": gin.H{
			"id":         u.ID,
			"email":      u.Email,
			"username":   u.Username,
			"balance":    u.Balance,
			"group_name": gname,
		},
	})
}
