package announcementaudience

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"tracehub/internal/ent"
	"tracehub/internal/ent/announcement"
	"tracehub/internal/ent/predicate"
	"tracehub/internal/ent/user"

	"entgo.io/ent/dialect/sql"
)

// IsGlobal 全员公告：无单用户定向且无多人列表。
func IsGlobal(a *ent.Announcement) bool {
	if a == nil {
		return true
	}
	if a.AudienceUserID != nil {
		return false
	}
	return len(a.AudienceUserIds) == 0
}

// NormalizeIDs 去重、去零、升序。
func NormalizeIDs(ids []int) []int {
	if len(ids) == 0 {
		return nil
	}
	seen := make(map[int]struct{}, len(ids))
	out := make([]int, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	sort.Ints(out)
	return out
}

// ValidateAudienceIDs 校验用户 id 均存在。
func ValidateAudienceIDs(ctx context.Context, client *ent.Client, ids []int) error {
	norm := NormalizeIDs(ids)
	if len(norm) == 0 {
		return fmt.Errorf("至少指定一位用户")
	}
	n, err := client.User.Query().Where(user.IDIn(norm...)).Count(ctx)
	if err != nil {
		return fmt.Errorf("校验用户失败")
	}
	if n != len(norm) {
		return fmt.Errorf("存在无效的用户 id")
	}
	return nil
}

// ResolveRecipientUserIDs 解析邮件/可见范围内的用户 id 列表。
func ResolveRecipientUserIDs(ctx context.Context, client *ent.Client, a *ent.Announcement) ([]int, error) {
	if a == nil {
		return nil, nil
	}
	if a.AudienceUserID != nil && *a.AudienceUserID > 0 {
		return []int{*a.AudienceUserID}, nil
	}
	if ids := NormalizeIDs(a.AudienceUserIds); len(ids) > 0 {
		return ids, nil
	}
	us, err := client.User.Query().Select(user.FieldID).All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]int, 0, len(us))
	for _, u := range us {
		out = append(out, u.ID)
	}
	return out, nil
}

// VisibleToUser 用户侧可见性：全员、单用户遗留、或多人列表包含该用户。
func VisibleToUser(userID int) predicate.Announcement {
	return announcement.Or(
		announcement.And(
			announcement.AudienceUserIDIsNil(),
			announcement.Or(
				announcement.AudienceUserIdsIsNil(),
				audienceUserIDsEmpty(),
			),
		),
		announcement.AudienceUserIDEQ(userID),
		audienceUserIDsContains(userID),
	)
}

func audienceUserIDsEmpty() predicate.Announcement {
	return predicate.Announcement(func(s *sql.Selector) {
		col := s.C(announcement.FieldAudienceUserIds)
		s.Where(sql.Or(
			sql.IsNull(col),
			sql.ExprP(col+" = '[]'::jsonb"),
		))
	})
}

func audienceUserIDsContains(userID int) predicate.Announcement {
	payload, _ := json.Marshal([]int{userID})
	return predicate.Announcement(func(s *sql.Selector) {
		col := s.C(announcement.FieldAudienceUserIds)
		s.Where(sql.ExprP(col+" @> ?::jsonb", string(payload)))
	})
}
