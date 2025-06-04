package client

import "context"

// TableUpdates represents a set of table updates.
type TableUpdates map[string]TableUpdate

// TableUpdate represents updates for a single table.
type TableUpdate struct {
	Rows map[string]RowUpdate
}

// RowUpdate contains old and new versions of a row.
type RowUpdate struct {
	New map[string]interface{}
	Old map[string]interface{}
}

// Client is a minimal interface implemented by libovsdb clients.
type Client interface {
	MonitorAll(ctx context.Context, tables []string) (<-chan TableUpdates, error)
	List(ctx context.Context, table string, out interface{}) error
	Close()
}

type Option interface{}

// NewOVSDBClient is a stub constructor used in tests.
func NewOVSDBClient(_ ...Option) (Client, error) { return nil, nil }
