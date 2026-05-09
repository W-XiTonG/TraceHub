package pginit

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/lib/pq"
)

// ErrUnsupportedDSN 表示当前 DSN 无法用于自动建库（需为 postgres:// 或 postgresql:// URL）。
var ErrUnsupportedDSN = errors.New("pginit: DSN 需为 postgres:// 或 postgresql:// 形式的 URL 才能自动创建数据库")

// EnsureDatabase 在目标库不存在时，使用同主机/账号连接维护库 postgres 并执行 CREATE DATABASE。
// 需要 DSN 中的用户对维护库有连接权限且具备 CREATEDB（或超级用户）权限。
// 表结构仍由调用方通过 Ent Schema.Create 创建。
// 若新建了数据库，createdName 为非空字符串。
func EnsureDatabase(ctx context.Context, dsn string) (createdDB bool, createdName string, err error) {
	if err := ping(ctx, dsn); err == nil {
		return false, "", nil
	} else if !isUndefinedDatabase(err) {
		return false, "", err
	}

	target, adminDSN, err := postgresURLWithDB(dsn, "postgres")
	if err != nil {
		return false, "", err
	}
	if target == "" || strings.EqualFold(target, "postgres") {
		return false, "", fmt.Errorf("%w: 目标库名为空或为 postgres", ErrUnsupportedDSN)
	}
	if !isSafeIdent(target) {
		return false, "", fmt.Errorf("pginit: 非法数据库名 %q（仅允许字母、数字、下划线）", target)
	}

	adm, err := sql.Open("postgres", adminDSN)
	if err != nil {
		return false, "", fmt.Errorf("pginit: 打开维护库连接: %w", err)
	}
	defer func() { _ = adm.Close() }()

	if err := adm.PingContext(ctx); err != nil {
		return false, "", fmt.Errorf("pginit: 连接维护库失败（请确认用户可连 postgres 库）: %w", clarifyPQ(adminDSN, err))
	}

	var one int
	switch err := adm.QueryRowContext(ctx, `SELECT 1 FROM pg_database WHERE datname = $1`, target).Scan(&one); {
	case err == nil:
		// 已存在（竞态或其它原因）
	case errors.Is(err, sql.ErrNoRows):
		qname := pq.QuoteIdentifier(target)
		if _, err := adm.ExecContext(ctx, "CREATE DATABASE "+qname); err != nil {
			var pqErr *pq.Error
			if errors.As(err, &pqErr) && pqErr.Code == "42P04" { // duplicate_database
				break
			}
			return false, "", fmt.Errorf("pginit: CREATE DATABASE %s: %w", qname, err)
		}
		createdDB = true
		createdName = target
	default:
		return false, "", err
	}

	if err := ping(ctx, dsn); err != nil {
		return createdDB, createdName, fmt.Errorf("pginit: 建库后仍无法连接目标库: %w", err)
	}
	return createdDB, createdName, nil
}

func ping(ctx context.Context, dsn string) error {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	return clarifyPQ(dsn, db.PingContext(ctx))
}

// clarifyPQ 将常见 pq 错误转为可读中文说明（避免服务端返回中文在 Windows 控制台乱码看不出原因）。
func clarifyPQ(dsn string, err error) error {
	if err == nil {
		return nil
	}
	var pqErr *pq.Error
	if !errors.As(err, &pqErr) {
		return err
	}
	userHint := ""
	if u, e := url.Parse(dsn); e == nil && u.User != nil {
		userHint = u.User.Username()
	}
	switch pqErr.Code {
	case "28P01": // invalid_password — 不 %w 嵌 pq 原文，避免服务端中文在 Windows 控制台乱码污染日志
		const hint = "请核对 configs 中 database.dsn 与该实例上的口令一致（若用 Docker，请对照 deploy/docker-compose.local.yml 的 POSTGRES_USER / POSTGRES_PASSWORD）"
		if userHint != "" {
			return fmt.Errorf("PostgreSQL 用户 %q 密码验证失败，%s（SQLSTATE 28P01）", userHint, hint)
		}
		return fmt.Errorf("PostgreSQL 密码验证失败，%s（SQLSTATE 28P01）", hint)
	case "28000": // invalid_authorization_specification（如 pg_hba 拒绝）
		return fmt.Errorf("PostgreSQL 拒绝连接（身份/访问策略）：%w", err)
	default:
		if strings.Contains(strings.ToLower(err.Error()), "password authentication failed") {
			return fmt.Errorf("PostgreSQL 密码认证失败，请核对 database.dsn")
		}
	}
	return err
}

func isUndefinedDatabase(err error) bool {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		return pqErr.Code == "3D000" // invalid_catalog_name — database "x" does not exist
	}
	return strings.Contains(err.Error(), "does not exist")
}

func postgresURLWithDB(dsn, newDB string) (targetDB string, rewrittenDSN string, err error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return "", "", fmt.Errorf("pginit: 解析 DSN: %w", err)
	}
	if u.Scheme != "postgres" && u.Scheme != "postgresql" {
		return "", "", ErrUnsupportedDSN
	}
	targetDB = strings.TrimPrefix(u.EscapedPath(), "/")
	if targetDB != "" {
		if t, e := url.PathUnescape(targetDB); e == nil {
			targetDB = t
		}
	}
	u2 := *u
	u2.Path = "/" + newDB
	return targetDB, u2.String(), nil
}

func isSafeIdent(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			continue
		}
		return false
	}
	return true
}
