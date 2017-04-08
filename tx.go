package hermes

import (
	"context"
	"database/sql"
	"errors"
	"sync"

	"github.com/jmoiron/sqlx"
)

var (
	// ErrBadContext returned when the caller attempts to reset the context.
	ErrBadContext = errors.New("Context mismatch")

	// ErrTxRolledBack returned by calls to the transaction if it has been
	// rolled back.
	ErrTxRolledBack = errors.New("Transaction rolled back")

	// ErrTxCommitted returned if the caller tries to rollback then
	// commit a transaction in the same function.
	ErrTxCommitted = errors.New("Already committed")
)

const (
	_pending = iota
	_rollback
	_commit
)

// Tx wraps a sqlx.Tx transaction.  Tracks context.
type Tx struct {
	sync.Mutex

	db       *DB
	ctx      context.Context
	internal *sqlx.Tx

	current int   // current state
	history []int // past states

	rollback bool // is the transaction being rolled back?
}

// DB returns the base database connection.
func (tx *Tx) DB() *sqlx.DB {
	return tx.db.DB()
}

// Tx returns the internal sqlx transaction.
func (tx *Tx) Tx() *sqlx.Tx {
	return tx.internal
}

// Context returns the context associated with this transaction.
func (tx *Tx) Context() context.Context {
	return tx.ctx
}

// Begin a new transaction.  Returns a Conn wrapping the transaction
// (*sqlx.Tx).
func (tx *Tx) Begin() (Conn, error) {
	tx.Lock()
	defer tx.Unlock()

	if tx.rollback {
		return nil, ErrTxRolledBack
	}

	tx.push()
	return tx, nil
}

// BeginCtx begins a new transaction in context.  The Conn will have the context
// associated with it and use it for all subsequent commands.
func (tx *Tx) BeginCtx(ctx context.Context) (Conn, error) {
	tx.Lock()
	defer tx.Unlock()

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
	tx.Lock()
	defer tx.Unlock()

	if err := tx.ok(); err != nil {
		return nil, err
	}

	if tx.ctx != nil {
		return tx.internal.ExecContext(tx.ctx, query, args...)
	}

	return tx.internal.Exec(query, args...)
}

// Query the databsae.
func (tx *Tx) Query(query string, args ...interface{}) (*sqlx.Rows, error) {
	tx.Lock()
	defer tx.Unlock()

	if err := tx.ok(); err != nil {
		return nil, err
	}

	if tx.ctx != nil {
		return tx.internal.QueryxContext(tx.ctx, query, args...)
	}

	return tx.internal.Queryx(query, args...)
}

// Row queries the databsae for a single row.
func (tx *Tx) Row(query string, args ...interface{}) (*sqlx.Row, error) {
	tx.Lock()
	defer tx.Unlock()

	if err := tx.ok(); err != nil {
		return nil, err
	}

	if tx.ctx != nil {
		return tx.internal.QueryRowxContext(tx.ctx, query, args...), nil
	}

	return tx.internal.QueryRowx(query, args...), nil
}

// Prepare a database query.
func (tx *Tx) Prepare(query string) (*sqlx.Stmt, error) {
	tx.Lock()
	defer tx.Unlock()

	if err := tx.ok(); err != nil {
		return nil, err
	}

	// TODO:  No PreparexContext?
	//
	// if tx.ctx != nil {
	// 	return tx.internal.PreparexContext(tx.ctx, query, args...)
	// }

	return tx.internal.Preparex(query)
}

// Get a single record from the database, e.g. "SELECT ... LIMIT 1".
func (tx *Tx) Get(dest interface{}, query string, args ...interface{}) error {
	tx.Lock()
	defer tx.Unlock()

	if err := tx.ok(); err != nil {
		return err
	}

	if tx.ctx != nil {
		return tx.internal.GetContext(tx.ctx, dest, query, args...)
	}

	return tx.internal.Get(dest, query, args...)
}

// Select a collection record from the database.
func (tx *Tx) Select(dest interface{}, query string, args ...interface{}) error {
	tx.Lock()
	defer tx.Unlock()

	if err := tx.ok(); err != nil {
		return err
	}

	if tx.ctx != nil {
		return tx.internal.SelectContext(tx.ctx, dest, query, args...)
	}

	return tx.internal.Select(dest, query, args...)
}

// Commit the current transaction.  Returns ErrTxRolledBack if the transaction
// was already rolled back, or ErrTxCommitted if it was committed.
func (tx *Tx) Commit() error {
	tx.Lock()
	defer tx.Unlock()

	if tx.rollback {
		return ErrTxRolledBack
	}

	if tx.current == _commit {
		return ErrTxCommitted
	}

	if len(tx.history) == 0 {
		if err := tx.internal.Commit(); err != nil {
			return err
		}
	}

	tx.current = _commit

	return nil
}

// Rollback the transaction.  Ignored if the transaction is already in a
// rollback.  Returns ErrTxCommitted if the transaction was committed.
func (tx *Tx) Rollback() error {
	tx.Lock()
	defer tx.Unlock()

	if tx.rollback {
		return nil
	}

	if tx.current == _commit {
		return ErrTxCommitted
	}

	if err := tx.internal.Rollback(); err != nil {
		return err
	}

	tx.current = _rollback
	tx.rollback = true
	tx.pop()

	return nil
}

// Close will automatically rollback a transaction if it hasn't been committed.
func (tx *Tx) Close() error {
	tx.Lock()
	defer tx.Unlock()

	if tx.current == _rollback || tx.current == _commit {
		tx.pop()
		return nil
	}

	if err := tx.internal.Rollback(); err != nil {
		tx.pop()
		return err
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
		return
	}

	tx.current, tx.history = tx.history[len(tx.history)-1], tx.history[:len(tx.history)-1]
}
