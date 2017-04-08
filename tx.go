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

	// ErrTxFailed returned by calls to the transaction if it's in a failed
	// state.
	ErrTxFailed = errors.New("Transaction failed")

	// ErrTxRolledBack returned by calls to the transaction if it has been
	// rolled back.
	ErrTxRolledBack = errors.New("Transaction rolled back")

	// ErrAlreadyCommitted returned if the caller tries to rollback then
	// commit a transaction in the same function.
	ErrAlreadyCommitted = errors.New("Already committed")
)

const (
	pending = iota
	rollback
	commit
	failed
)

// Tx wraps a sqlx.Tx transaction.  Tracks context.
type Tx struct {
	sync.Mutex

	db       *DB
	ctx      context.Context
	internal *sqlx.Tx

	current int   // current state
	history []int // past states

	rollback bool  // is the transaction being rolled back?
	failure  error // if a call fails
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
		tx.failure = ErrBadContext
		return nil, ErrBadContext
	}

	tx.ctx = ctx
	tx.push()

	return tx, nil
}

// Exec executes a database statement with no results..
func (tx *Tx) Exec(query string, args ...interface{}) (sql.Result, error) {
	if tx.failure != nil {
		return nil, ErrTxFailed
	} else if tx.rollback {
		return nil, ErrTxRolledBack
	}

	if tx.ctx != nil {
		return tx.internal.ExecContext(tx.ctx, query, args...)
	}

	return tx.internal.Exec(query, args...)
}

// Query the databsae.
func (tx *Tx) Query(query string, args ...interface{}) (*sqlx.Rows, error) {
	if tx.failure != nil {
		return nil, ErrTxFailed
	} else if tx.rollback {
		return nil, ErrTxRolledBack
	}

	if tx.ctx != nil {
		return tx.internal.QueryxContext(tx.ctx, query, args...)
	}

	return tx.internal.Queryx(query, args...)
}

// Row queries the databsae for a single row.
func (tx *Tx) Row(query string, args ...interface{}) (*sqlx.Row, error) {
	if tx.failure != nil {
		return nil, ErrTxFailed
	} else if tx.rollback {
		return nil, ErrTxRolledBack
	}

	if tx.ctx != nil {
		return tx.internal.QueryRowxContext(tx.ctx, query, args...), nil
	}

	return tx.internal.QueryRowx(query, args...), nil
}

// Prepare a database query.
func (tx *Tx) Prepare(query string) (*sqlx.Stmt, error) {
	if tx.failure != nil {
		return nil, ErrTxFailed
	} else if tx.rollback {
		return nil, ErrTxRolledBack
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
	if tx.failure != nil {
		return ErrTxFailed
	} else if tx.rollback {
		return ErrTxRolledBack
	}

	if tx.ctx != nil {
		return tx.internal.GetContext(tx.ctx, dest, query, args...)
	}

	return tx.internal.Get(dest, query, args...)
}

// Select a collection record from the database.
func (tx *Tx) Select(dest interface{}, query string, args ...interface{}) error {
	if tx.failure != nil {
		return ErrTxFailed
	} else if tx.rollback {
		return ErrTxRolledBack
	}

	if tx.ctx != nil {
		return tx.internal.SelectContext(tx.ctx, dest, query, args...)
	}

	return tx.internal.Select(dest, query, args...)
}

// Commit the current transaction.  If this is a child transaction,
func (tx *Tx) Commit() error {
	tx.Lock()
	defer tx.Lock()

	if tx.rollback || tx.current == rollback {
		return ErrTxRolledBack
	}

	if len(tx.history) == 0 {
		if err := tx.internal.Commit(); err != nil {
			tx.failure = err
			tx.current = failed
			return err
		}
	}

	tx.current = commit

	return nil
}

// Rollback does nothing in a raw connection.
func (tx *Tx) Rollback() error {
	tx.Lock()
	defer tx.Unlock()

	if tx.rollback {
		return ErrTxRolledBack
	}

	if tx.current == commit {
		return ErrAlreadyCommitted
	}

	if err := tx.internal.Rollback(); err != nil {
		tx.failure = err
		tx.pop()
		return err
	}

	tx.current = rollback
	tx.rollback = true
	tx.pop()

	return nil
}

// Close will automatically rollback a transaction if it hasn't been committed.
func (tx *Tx) Close() error {
	tx.Lock()
	defer tx.Unlock()

	if tx.rollback || tx.current == commit {
		tx.pop()
		return nil
	}

	if err := tx.internal.Rollback(); err != nil {
		tx.failure = err
		tx.pop()
		return err
	}

	tx.current = rollback
	tx.rollback = true
	tx.pop()

	return nil
}

func (tx *Tx) push() {
	tx.history = append(tx.history, tx.current)
	tx.current = pending
}

func (tx *Tx) pop() {
	if len(tx.history) == 0 {
		return
	}

	tx.current, tx.history = tx.history[len(tx.history)-1], tx.history[:len(tx.history)-1]
}
