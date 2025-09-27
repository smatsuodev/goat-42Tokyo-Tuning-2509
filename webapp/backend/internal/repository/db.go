package repository

import (
	"context"
	"database/sql"
)

type DBTX interface {
	GetContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error
	SelectContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	Rebind(query string) string
	NamedExecContext(ctx context.Context, query string, argc interface{}) (sql.Result, error)
	NamedExec(query string, argc interface{}) (sql.Result, error)
}
