package audit

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/shareinto/paas/internal/platform/database"
	"github.com/shareinto/paas/internal/shared"
)

type MySQLRepository struct {
	db *sql.DB
}

func NewMySQLRepository(_ context.Context, db *sql.DB) (*MySQLRepository, error) {
	return &MySQLRepository{db: db}, nil
}

func (r *MySQLRepository) Append(ctx context.Context, log AuditLog) error {
	details, err := database.MarshalJSON(log.Details)
	if err != nil {
		return err
	}
	_, err = database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
INSERT INTO audit_logs (
  id, actor_id, actor_type, subject_type, tenant_id, project_id, resource_type,
  resource_id, action, result, summary, details, occurred_at, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		log.ID, log.ActorID, log.ActorType, log.SubjectType, log.TenantID, log.ProjectID,
		log.ResourceType, log.ResourceID, log.Action, log.Result, log.Summary, string(details),
		mysqlTime(log.OccurredAt), mysqlTime(log.CreatedAt))
	return database.ConflictOrUnavailable(err, "audit log already exists", "append audit log failed")
}

func mysqlTime(value time.Time) time.Time {
	if value.IsZero() {
		return time.Now().UTC()
	}
	return value
}

func (r *MySQLRepository) Get(ctx context.Context, id shared.ID) (AuditLog, error) {
	log, err := scanAuditLog(database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, `
SELECT id, actor_id, actor_type, subject_type, tenant_id, project_id, resource_type,
       resource_id, action, result, summary, details, occurred_at, created_at
FROM audit_logs WHERE id = ?`, id))
	if err != nil {
		return AuditLog{}, database.NotFound(err, "audit log not found")
	}
	return log, nil
}

func (r *MySQLRepository) List(ctx context.Context, query Query, page shared.PageRequest) (shared.PageResult[AuditLog], error) {
	where, args := auditWhere(query)
	var total int64
	countSQL := "SELECT COUNT(*) FROM audit_logs" + where
	if err := database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, countSQL, args...).Scan(&total); err != nil {
		return shared.PageResult[AuditLog]{}, database.WrapUnavailable(err, "count audit logs failed")
	}
	page, limit, offset := database.LimitOffset(page)
	args = append(args, limit, offset)
	rows, err := database.ExecutorFromContext(ctx, r.db).QueryContext(ctx, `
SELECT id, actor_id, actor_type, subject_type, tenant_id, project_id, resource_type,
       resource_id, action, result, summary, details, occurred_at, created_at
FROM audit_logs`+where+` ORDER BY occurred_at DESC, created_at DESC, id DESC LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return shared.PageResult[AuditLog]{}, database.WrapUnavailable(err, "list audit logs failed")
	}
	defer rows.Close()
	items := []AuditLog{}
	for rows.Next() {
		log, err := scanAuditLog(rows)
		if err != nil {
			return shared.PageResult[AuditLog]{}, err
		}
		items = append(items, log)
	}
	if err := rows.Err(); err != nil {
		return shared.PageResult[AuditLog]{}, database.WrapUnavailable(err, "list audit logs failed")
	}
	return shared.NewPageResult(items, total, page), nil
}

type auditScanner interface {
	Scan(dest ...any) error
}

func scanAuditLog(scanner auditScanner) (AuditLog, error) {
	var log AuditLog
	var details []byte
	if err := scanner.Scan(&log.ID, &log.ActorID, &log.ActorType, &log.SubjectType, &log.TenantID, &log.ProjectID, &log.ResourceType, &log.ResourceID, &log.Action, &log.Result, &log.Summary, &details, &log.OccurredAt, &log.CreatedAt); err != nil {
		return AuditLog{}, err
	}
	var decoded map[string]string
	if err := database.UnmarshalJSON(details, &decoded); err != nil {
		return AuditLog{}, err
	}
	log.Details = decoded
	if log.Details == nil {
		log.Details = map[string]string{}
	}
	return log, nil
}

func auditWhere(query Query) (string, []any) {
	clauses := []string{}
	args := []any{}
	addID := func(column string, id shared.ID) {
		if id.IsZero() {
			return
		}
		clauses = append(clauses, column+" = ?")
		args = append(args, id)
	}
	addID("tenant_id", query.TenantID)
	addID("project_id", query.ProjectID)
	addID("actor_id", query.ActorID)
	if query.ResourceType != "" {
		clauses = append(clauses, "resource_type = ?")
		args = append(args, query.ResourceType)
	}
	addID("resource_id", query.ResourceID)
	if query.Action != "" {
		clauses = append(clauses, "action = ?")
		args = append(args, query.Action)
	}
	if query.From != nil {
		clauses = append(clauses, "occurred_at >= ?")
		args = append(args, *query.From)
	}
	if query.To != nil {
		clauses = append(clauses, "occurred_at <= ?")
		args = append(args, *query.To)
	}
	if len(clauses) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}
