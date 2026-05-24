package usagelogfilter

import (
	"tracehub/internal/ent"
	"tracehub/internal/ent/predicate"
	"tracehub/internal/ent/usagelog"

	"entgo.io/ent/dialect/sql"
)

// ApplyClientOutcomeOnly 仅保留「一次客户端请求」对应的统计行：
// is_client_outcome=true，或迁移前遗留的单行（attempt=1 且同 request_id 无更高序号尝试）。
func ApplyClientOutcomeOnly(q *ent.UsageLogQuery) *ent.UsageLogQuery {
	return q.Where(clientOutcomePredicate())
}

func clientOutcomePredicate() predicate.UsageLog {
	return predicate.UsageLog(func(s *sql.Selector) {
		t := sql.Table(usagelog.Table).As("ul_chain")
		s.Where(sql.Or(
			sql.EQ(s.C(usagelog.FieldIsClientOutcome), true),
			sql.And(
				sql.EQ(s.C(usagelog.FieldIsClientOutcome), false),
				sql.EQ(s.C(usagelog.FieldUpstreamAttempt), 1),
				sql.Or(
					sql.IsNull(s.C(usagelog.FieldRequestID)),
					sql.EQ(s.C(usagelog.FieldRequestID), ""),
					sql.NotExists(
						sql.Select(t.C(usagelog.FieldID)).
							From(t).
							Where(sql.And(
								sql.ColumnsEQ(s.C(usagelog.FieldRequestID), t.C(usagelog.FieldRequestID)),
								sql.GT(t.C(usagelog.FieldUpstreamAttempt), 1),
							)),
					),
				),
			),
		))
	})
}
