// Package hermes wraps the jmoiron/sqlx *sqlx.DB and *sqlx.Tx in a common
// interface, hermes.Conn.
//
// Use hermes.Conn in functions to optionally support transactions in your
// database queries.  It allows you to create database queries composed of
// other functions without having to worry about whether or not you're working
// off a database connection or an existing transaction.
//
// Additionally, testing with the database becomes easier.  Simply create a
// transaction at the beginning of every test with a `defer tx.Close()`, pass
// the transaction into your functions instead of the database connection,
// and don't commit the transaction at the end.  Every database insert, select,
// update, and delete will function normally in your test, then rollback and
// clean out the database automatically at the end.
package hermes

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/cenkalti/backoff"
	"github.com/jmoiron/sqlx"
)

var (
	// MaxElapsedTime is the maximum time hermes.Connect() will spend attempting
	// to connect to the database before returning an error
	MaxElapsedTime = backoff.DefaultMaxElapsedTime
)

// Conn masks the *sqlx.DB and *sqlx.Tx.
type Conn interface {
	// DB returns the base database connection.
	BaseDB() *sqlx.DB

	// Tx returns the base database transaction, or nil if there is no
	// transaction.
	BaseTx() *sqlx.Tx

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

	// Is this connection in a rollback state?
	RolledBack() bool

	// The data source name for this connection
	Name() string
}

// Connect opens a connection to the database and pings it.
func Connect(driverName, dataSourceName string, maxOpen, maxIdle int) (*DB, error) {
	db, err := sqlx.Open(driverName, dataSourceName)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(maxOpen)
	db.SetMaxIdleConns(maxIdle)

	// ping database with exponential back off
	if err := mustPing(db); err != nil {
		return nil, err
	}

	return &DB{
		name:     dataSourceName,
		internal: db,
	}, nil
}

// mustPing pings the database with an exponential back off. If we cannot
// connect after MaxElapsedTime, return an error.
func mustPing(db *sqlx.DB) error {
	var err error
	b := backoff.NewExponentialBackOff()
	b.MaxElapsedTime = MaxElapsedTime
	ticker := backoff.NewTicker(b)

	for range ticker.C {
		if err = db.Ping(); err != nil {
			continue
		}
		ticker.Stop()
		return nil
	}

	return fmt.Errorf("Could not ping database")
}
