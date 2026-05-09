package bootstrap

import (
	"context"

	"tracehub/internal/ent"
	"tracehub/internal/ent/group"
	"tracehub/internal/ent/user"
	"tracehub/internal/pkg/config"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

func EnsureDefaults(ctx context.Context, client *ent.Client, cfg *config.Config, log *zap.Logger) error {
	gname := cfg.Bootstrap.DefaultGroupName
	if gname == "" {
		gname = "基础分组"
	}
	g, err := client.Group.Query().Where(group.NameEQ(gname)).Only(ctx)
	if ent.IsNotFound(err) {
		g, err = client.Group.Create().
			SetName(gname).
			SetDescription("默认分组").
			SetRateMultiplier(1).
			SetEnabled(true).
			Save(ctx)
		if err != nil {
			return err
		}
		log.Info("created default group", zap.String("name", gname))
	} else if err != nil {
		return err
	}

	if cfg.Bootstrap.AdminEmail == "" || cfg.Bootstrap.AdminPassword == "" {
		return nil
	}
	exists, err := client.User.Query().Where(user.EmailEQ(cfg.Bootstrap.AdminEmail)).Exist(ctx)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(cfg.Bootstrap.AdminPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = client.User.Create().
		SetEmail(cfg.Bootstrap.AdminEmail).
		SetUsername("admin").
		SetPasswordHash(string(hash)).
		SetBalance(cfg.Site.DefaultNewUserBalance).
		SetStatus(1).
		SetRole("superadmin").
		SetGroupID(g.ID).
		Save(ctx)
	if err != nil {
		return err
	}
	log.Info("seeded superadmin", zap.String("email", cfg.Bootstrap.AdminEmail))
	return nil
}
