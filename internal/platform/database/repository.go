package database

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"

	"github.com/go-sql-driver/mysql"
	"github.com/shareinto/paas/internal/shared"
)

func NotFound(err error, message string) error {
	if errors.Is(err, sql.ErrNoRows) {
		return shared.NewError(shared.CodeNotFound, message)
	}
	return err
}

func WrapUnavailable(err error, message string) error {
	if err == nil {
		return nil
	}
	return shared.WrapError(shared.CodeUnavailable, message, err)
}

func ConflictOrUnavailable(err error, conflictMessage string, unavailableMessage string) error {
	if err == nil {
		return nil
	}
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
		return shared.NewError(shared.CodeConflict, conflictMessage)
	}
	return shared.WrapError(shared.CodeUnavailable, unavailableMessage, err)
}

func MarshalJSON(value any) ([]byte, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return nil, shared.WrapError(shared.CodeInternal, "encode json failed", err)
	}
	return payload, nil
}

func UnmarshalJSON(payload []byte, target any) error {
	if len(payload) == 0 || strings.EqualFold(string(payload), "null") {
		return nil
	}
	if err := json.Unmarshal(payload, target); err != nil {
		return shared.WrapError(shared.CodeInternal, "decode json failed", err)
	}
	return nil
}

func LimitOffset(page shared.PageRequest) (shared.PageRequest, int, int) {
	page = page.Normalize()
	return page, page.PageSize, page.Offset()
}

func RequireAffected(result sql.Result, notFoundMessage string) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return WrapUnavailable(err, "read affected rows failed")
	}
	if affected == 0 {
		return shared.NewError(shared.CodeNotFound, notFoundMessage)
	}
	return nil
}
