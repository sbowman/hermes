package hermes

import (
	"context"
	"database/sql"

	"github.com/jmoiron/sqlx"
)

// Conn masks the *sqlx.DB and *sqlx.Tx.
type Conn interface {
	// DB returns the base database connection.
	DB() *sqlx.DB

	// Tx returns the base database transaction, or nil if there is no
	// transaction.
	Tx() *sqlx.Tx

	// Context returns the context associated with this transaction, or nil
	// if a context is not associated.
	Context() context.Context

	// Begin a new transaction.  Returns a Conn wrapping the transaction
	// (*sqlx.Tx).
	Begin() (Conn, error)

	// Begin a new transaction in context.  The Conn will have the context
	// associated with it and use it for all subsequent commands.
	BeginCtx(ctx context.Context) (Conn, error)

	// Exec executes a database statement with no results..
	Exec(query string, args ...interface{}) (sql.Result, error)

	// Query the databsae.
	Query(query string, args ...interface{}) (*sqlx.Rows, error)

	// Row queries for a single row.
	Row(query string, args ...interface{}) (*sqlx.Row, error)

	// Prepare a database query.
	Prepare(query string) (*sqlx.Stmt, error)

	// Get a single record from the database, e.g. "SELECT ... LIMIT 1".
	Get(dest interface{}, query string, args ...interface{}) error

	// Select a collection of results.
	Select(dest interface{}, query string, args ...interface{}) error

	// Commit the transaction.
	Commit() error

	// Rollback the transaction.  This will rollback any parent transactions
	// as well.
	Rollback() error

	// Close rolls back a transaction (and all its parent transactions) if
	// it hasn't been committed.  Useful in a defer.
	Close() error
}

// Connect opens a connection to the database and pings it.
func Connect(driverName, dataSourceName string) (*DB, error) {
	db, err := sqlx.Connect(driverName, dataSourceName)
	if err != nil {
		return nil, err
	}

	return &DB{db}, nil
}
