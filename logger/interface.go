package logger

type Sqlable interface {
	InsertQuery() (query string, fields string, values []any)
}

type PurgeQueryFunc func() string
type CreateTableQueryFunc func() string
