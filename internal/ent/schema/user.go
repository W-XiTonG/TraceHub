package schema

import (
	"time"

	"tracehub/internal/pkg/alertconfig"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type User struct {
	ent.Schema
}

func (User) Fields() []ent.Field {
	return []ent.Field{
		field.Int("group_id").Optional(),
		field.String("email").MaxLen(255).Unique().NotEmpty(),
		field.String("username").MaxLen(100).Optional(),
		field.String("password_hash").MaxLen(255).Optional().Sensitive(),
		field.Int64("balance").Default(0),
		field.Int("status").Default(1).Comment("1 ok 2 banned"),
		field.String("role").Default("user").MaxLen(50),
		field.JSON("ip_whitelist", []string{}).Optional(),
		field.JSON("ip_blacklist", []string{}).Optional(),
		field.Int("concurrency").Default(0),
		field.Float("rate_multiplier").Default(1),
		field.String("totp_secret").Optional().MaxLen(255).Sensitive(),
		field.String("webhook_url").Optional().MaxLen(500),
		field.Int64("balance_alert_threshold").Default(0),
		field.JSON("custom_alert_config", alertconfig.Stored{}).Optional(),
		field.Time("last_login_at").Optional(),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (User) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("group", Group.Type).
			Ref("users").
			Field("group_id").
			Unique(),
		edge.To("api_keys", APIKey.Type),
		edge.To("usage_logs", UsageLog.Type),
		edge.To("redeemed_codes", RedeemCode.Type),
		edge.To("created_redeem_codes", RedeemCode.Type),
		edge.To("redeem_ledgers", RedeemLedger.Type),
		edge.To("admin_audit_logs", AdminAuditLog.Type),
		edge.To("created_announcements", Announcement.Type),
	}
}

func (User) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("email"),
		index.Fields("group_id"),
		index.Fields("status"),
	}
}
