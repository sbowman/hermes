package hermes

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jmoiron/sqlx"
)

var (
	// ErrBadContext returned when the caller attempts to reset the context.
	ErrBadContext = errors.New("context mismatch")

	// ErrTxRolledBack returned by calls to the transaction if it has been
	// rolled back.
	ErrTxRolledBack = errors.New("transaction rolled back")

	// ErrTxCommitted returned if the caller tries to rollback then
	// commit a transaction in the same function.
	ErrTxCommitted = errors.New("already committed")
)

const (
	_pending = iota
	_rollback
	_commit
)

// Tx wraps a sqlx.Tx transaction.  Tracks context.
type Tx struct {
	db       *DB
	ctx      context.Context
	internal *sqlx.Tx

	current int   // current state
	history []int // past states

	rollback bool     // is the transaction being rolled back?
	timer    *txTimer // if TxTimeout is set, reports when Tx existence exceeds timeout
}

// BaseDB returns the base database connection.
func (tx *Tx) BaseDB() *sqlx.DB {
	return tx.db.BaseDB()
}

// BaseTx returns the internal sqlx transaction.
func (tx *Tx) BaseTx() *sqlx.Tx {
	return tx.internal
}

// Context returns the context associated with this transaction.
func (tx *Tx) Context() context.Context {
	return tx.ctx
}

// Begin a new transaction.  Returns a Conn wrapping the transaction
// (*sqlx.Tx).
func (tx *Tx) Begin() (Conn, error) {
	if tx.rollback {
		return nil, ErrTxRolledBack
	}

	tx.push()
	return tx, nil
}

// BeginCtx begins a new transaction in context.  The Conn will have the context
// associated with it and use it for all subsequent commands.
func (tx *Tx) BeginCtx(ctx context.Context) (Conn, error) {
	if tx.rollback {
		return nil, ErrTxRolledBack
	}

	if tx.ctx != nil && tx.ctx != ctx {
		return nil, ErrBadContext
	}

	tx.ctx = ctx
	tx.push()

	return tx, nil
}

// Exec executes a database statement with no results..
func (tx *Tx) Exec(query string, args ...interface{}) (sql.Result, error) {
	if err := tx.ok(); err != nil {
		return nil, err
	}

	var res sql.Result
	var err error

	if tx.ctx != nil {
		res, err = tx.internal.ExecContext(tx.ctx, query, args...)
	} else {
		res, err = tx.internal.Exec(query, args...)
	}

	return res, tx.check(err)
}

// Query the database.
func (tx *Tx) Query(query string, args ...interface{}) (*sqlx.Rows, error) {
	if err := tx.ok(); err != nil {
		return nil, err
	}

	var rows *sqlx.Rows
	var err error

	if tx.ctx != nil {
		rows, err = tx.internal.QueryxContext(tx.ctx, query, args...)
	} else {
		rows, err = tx.internal.Queryx(query, args...)
	}

	return rows, tx.check(err)
}

// Row queries the databsae for a single row.
func (tx *Tx) Row(query string, args ...interface{}) (*sqlx.Row, error) {
	if err := tx.ok(); err != nil {
		return nil, err
	}

	var row *sqlx.Row

	if tx.ctx != nil {
		row = tx.internal.QueryRowxContext(tx.ctx, query, args...)
	} else {
		row = tx.internal.QueryRowx(query, args...)
	}

	if row.Err() != nil {
		return nil, tx.check(row.Err())
	}

	return row, nil
}

// Prepare a database query.
func (tx *Tx) Prepare(query string) (*sqlx.Stmt, error) {
	if err := tx.ok(); err != nil {
		return nil, err
	}

	// TODO:  No PreparexContext?
	//
	// if tx.ctx != nil {
	// 	return tx.internal.PreparexContext(tx.ctx, query, args...)
	// }

	stmt, err := tx.internal.Preparex(query)
	return stmt, tx.check(err)
}

// Get a single record from the database, e.g. "SELECT ... LIMIT 1".
func (tx *Tx) Get(dest interface{}, query string, args ...interface{}) error {
	if err := tx.ok(); err != nil {
		return err
	}

	if tx.ctx != nil {
		return tx.check(tx.internal.GetContext(tx.ctx, dest, query, args...))
	}

	return tx.check(tx.internal.Get(dest, query, args...))
}

// Select a collection record from the database.
func (tx *Tx) Select(dest interface{}, query string, args ...interface{}) error {
	if err := tx.ok(); err != nil {
		return err
	}

	if tx.ctx != nil {
		return tx.check(tx.internal.SelectContext(tx.ctx, dest, query, args...))
	}

	return tx.check(tx.internal.Select(dest, query, args...))
}

// Commit the current transaction.  Returns ErrTxRolledBack if the transaction
// was already rolled back, or ErrTxCommitted if it was committed.
func (tx *Tx) Commit() error {
	if tx.rollback {
		return ErrTxRolledBack
	}

	if tx.current == _commit {
		return ErrTxCommitted
	}

	if len(tx.history) == 0 {
		if err := tx.internal.Commit(); err != nil {
			return tx.check(err)
		}
	}

	tx.current = _commit

	return nil
}

// Rollback the transaction.  Ignored if the transaction is already in a
// rollback.  Returns ErrTxCommitted if the transaction was committed.
func (tx *Tx) Rollback() error {
	if tx.rollback {
		return nil
	}

	if tx.current == _commit {
		return ErrTxCommitted
	}

	if err := tx.internal.Rollback(); err != nil {
		return tx.check(err)
	}

	tx.current = _rollback
	tx.rollback = true
	tx.pop()

	return nil
}

// Close will automatically rollback a transaction if it hasn't been committed.
func (tx *Tx) Close() error {
	if tx.current == _rollback || tx.current == _commit {
		tx.pop()
		return nil
	}

	if err := tx.internal.Rollback(); err != nil {
		tx.pop()
		return tx.check(err)
	}

	tx.current = _rollback
	tx.rollback = true
	tx.pop()

	return nil
}

// RolledBack returns true if the transaction was rolled back.
func (tx *Tx) RolledBack() bool {
	return tx.rollback
}

// Name returns the datasource name for this connection
func (tx *Tx) Name() string {
	return tx.db.name
}

// Confirm the transaction is viable before executing a query.
func (tx *Tx) ok() error {
	if tx.rollback {
		return ErrTxRolledBack
	}

	if tx.current == _commit {
		return ErrTxCommitted
	}

	return nil
}

func (tx *Tx) push() {
	tx.history = append(tx.history, tx.current)
	tx.current = _pending
}

func (tx *Tx) pop() {
	if len(tx.history) == 0 {
		if tx.timer != nil {
			tx.timer.stop()
			tx.timer = nil
		}

		return
	}

	tx.current, tx.history = tx.history[len(tx.history)-1], tx.history[:len(tx.history)-1]
}

func (tx *Tx) check(err error) error {
	return tx.db.check(err)
}
