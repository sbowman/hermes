package hermes

import (
	"context"
	"database/sql"

	"github.com/jmoiron/sqlx"
)

// DB represents a database connection.  Implements the hermes.Conn interface.
type DB struct {
	// OnFailure, if defined, is called when the database connection returns
	// a connection failed or other server-related error.  May be used to
	// reset the database pool connections.  Optional.
	OnFailure FailureFn

	name     string
	internal *sqlx.DB
}

// NewDB creates a new database connection.  Primary used for testing.
func NewDB(name string, internal *sqlx.DB, fn FailureFn) *DB {
	return &DB{
		OnFailure: fn,
		name:      name,
		internal:  internal,
	}
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
	return db.check(db.internal.Ping())
}

// DB returns the base database connection.
func (db *DB) BaseDB() *sqlx.DB {
	return db.internal
}

// Tx returns nil.
func (db *DB) BaseTx() *sqlx.Tx {
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
		return nil, db.check(err)
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
		return nil, db.check(err)
	}

	return &Tx{
		ctx:      ctx,
		db:       db,
		internal: tx,
	}, nil
}

// Exec executes a database statement with no results..
func (db *DB) Exec(query string, args ...interface{}) (sql.Result, error) {
	res, err := db.internal.Exec(query, args...)
	return res, db.check(err)
}

// Query the databsae.
func (db *DB) Query(query string, args ...interface{}) (*sqlx.Rows, error) {
	rows, err := db.internal.Queryx(query, args...)
	return rows, db.check(err)
}

// Row returns the results for a single row.
func (db *DB) Row(query string, args ...interface{}) (*sqlx.Row, error) {
	row := db.internal.QueryRowx(query, args...)

	err := row.Err()
	if err != nil {
		return nil, db.check(err)
	}

	return row, nil
}

// Prepare a database query.
func (db *DB) Prepare(query string) (*sqlx.Stmt, error) {
	stmt, err := db.internal.Preparex(query)
	return stmt, db.check(err)
}

// Get a single record from the database, e.g. "SELECT ... LIMIT 1".
func (db *DB) Get(dest interface{}, query string, args ...interface{}) error {
	return db.check(db.internal.Get(dest, query, args...))
}

// Select a collection of records from the database.
func (db *DB) Select(dest interface{}, query string, args ...interface{}) error {
	return db.check(db.internal.Select(dest, query, args...))
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
	return db.check(db.internal.Close())
}

// RolledBack always returns false.
func (db *DB) RolledBack() bool {
	return false
}

// Name returns the datasource name for this connection
func (db *DB) Name() string {
	return db.name
}

// Checks the error message and alerts if there was a problem.
func (db *DB) check(err error) error {
	if err == nil || db.OnFailure == nil || !DidConnectionFail(err) {
		return err
	}

	db.OnFailure(db, err)
	return err
}
