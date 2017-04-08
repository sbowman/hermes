package hermes

import (
	"context"
	"database/sql"

	"github.com/jmoiron/sqlx"
)

// DB represents a database connection.  Implements the hermes.Conn interface.
type DB struct {
	internal *sqlx.DB
}

// MaxOpen sets the maximum number of database connections to pool.
func (db *DB) MaxOpen(n int) {
	db.internal.SetMaxOpenConns(n)
}

// MaxIdle set the maximum number of idle connections to leave in the pool.
func (db *DB) MaxIdle(n int) {
	db.internal.SetMaxIdleConns(n)
}

// Ping the database to ensure it's alive.
func (db *DB) Ping() error {
	return db.internal.Ping()
}

// DB returns the base database connection.
func (db *DB) DB() *sqlx.DB {
	return db.internal
}

// Tx returns nil.
func (db *DB) Tx() *sqlx.Tx {
	return nil
}

// Context returns the context associated with this transaction.
func (db *DB) Context() context.Context {
	return nil
}

// Begin a new transaction.  Returns a Conn wrapping the transaction
// (*sqlx.Tx).
func (db *DB) Begin() (Conn, error) {
	tx, err := db.internal.Beginx()
	if err != nil {
		return nil, err
	}

	return &Tx{
		db:       db,
		internal: tx,
	}, nil
}

// BeginCtx begins a new transaction in context.  The Conn will have the context
// associated with it and use it for all subsequent commands.
func (db *DB) BeginCtx(ctx context.Context) (Conn, error) {
	tx, err := db.internal.Beginx()
	if err != nil {
		return nil, err
	}

	return &Tx{
		ctx:      ctx,
		db:       db,
		internal: tx,
	}, nil
}

// Exec executes a database statement with no results..
func (db *DB) Exec(query string, args ...interface{}) (sql.Result, error) {
	return db.internal.Exec(query, args...)
}

// Query the databsae.
func (db *DB) Query(query string, args ...interface{}) (*sqlx.Rows, error) {
	return db.internal.Queryx(query, args...)
}

// Row returns the results for a single row.
func (db *DB) Row(query string, args ...interface{}) (*sqlx.Row, error) {
	return db.internal.QueryRowx(query, args...), nil
}

// Prepare a database query.
func (db *DB) Prepare(query string) (*sqlx.Stmt, error) {
	return db.internal.Preparex(query)
}

// Get a single record from the database, e.g. "SELECT ... LIMIT 1".
func (db *DB) Get(dest interface{}, query string, args ...interface{}) error {
	return db.internal.Get(dest, query, args...)
}

// Select a collection of records from the database.
func (db *DB) Select(dest interface{}, query string, args ...interface{}) error {
	return db.internal.Select(dest, query, args...)
}

// Commit does nothing in a raw connection.
func (db *DB) Commit() error {
	return nil
}

// Rollback does nothing in a raw connection.
func (db *DB) Rollback() error {
	return nil
}

// Close closes the database connection and returns it to the pool.
func (db *DB) Close() error {
	return db.internal.Close()
}

// RolledBack always returns false.
func (db *DB) RolledBack() bool {
	return false
}
