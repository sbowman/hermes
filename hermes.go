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
	"errors"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/jmoiron/sqlx"
)

var (
	// TxTimeout configures the transaction timer, which warns you about
	// long-lived transactions.  This can be used in development to ensure
	// all transactions are closed correctly.
	//
	// Enabling transaction timeouts should not be used in production.  If
	// enabled, a timer is created for each transaction, adding measurable
	// overhead to database processing.
	TxTimeout struct {
		// Enabled must be set to true to enable transaction timers.
		Enabled bool

		// Duration is the time to wait in milliseconds before reporting
		// a transaction being left open.
		Duration time.Duration

		// Panic set to true causes Hermes to panic if the transaction
		// remains open past its duration.  When false, Hermes simply
		// writes a message to os.Stderr.
		Panic bool
	}

	// MaxRetryTime is the maximum time hermes.Connect() will spend attempting
	// to connect to the database before returning an error
	MaxRetryTime = backoff.DefaultMaxElapsedTime

	// ErrTooManyClients matches the error returned by PostgreSQL when the
	// number of client connections exceeds that allowed by the server.
	ErrTooManyClients = errors.New("pq: sorry, too many clients already")
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
	db, err := open(driverName, dataSourceName, maxOpen, maxIdle)
	if err != nil {
		return nil, err // should only return a misconfiguration error
	}

	return NewDB(dataSourceName, db, nil), nil
}

// ConnectUnchecked connects to the database, but does not test the connection
// before returning.
func ConnectUnchecked(driverName, dataSourceName string, maxOpen, maxIdle int) (*DB, error) {
	db, err := dial(driverName, dataSourceName, maxOpen, maxIdle)
	if err != nil {
		return nil, err // should only return a misconfiguration error
	}

	return NewDB(dataSourceName, db, nil), nil
}

// EnableTimeouts enables the transaction timer, which will display an error
// message or panic if a transaction exceeds the precribed duration.  Useful
// during development for tracking down transactions that weren't properly
// cleaned up.
//
// Transaction timers may be enabled and disabled at will without requiring a
// restart.
//
// Do not use in production!  The overhead will measurably slow down your application.
func EnableTimeouts(dur time.Duration, panic bool) {
	if dur == 0 {
		return
	}

	TxTimeout.Duration = dur
	TxTimeout.Panic = panic
	TxTimeout.Enabled = true
}

// DisableTimeouts disables transaction timeouts.  Transaction timers may be
// disabled at any time.  Any existing timers will simply clean themselves up
// quietly.
func DisableTimeouts() {
	TxTimeout.Enabled = false
	TxTimeout.Duration = 0
	TxTimeout.Panic = false
}

// Keep trying to open a database connection.  If the connection times out,
// retries for MaxRetryTime.
func open(driverName, dataSourceName string, maxOpen, maxIdle int) (*sqlx.DB, error) {
	var db *sqlx.DB
	var err error

	// Keep trying the connection until it confirmed
	b := backoff.NewExponentialBackOff()
	b.MaxElapsedTime = MaxRetryTime

	ticker := backoff.NewTicker(b)
	for range ticker.C {
		db, err = dial(driverName, dataSourceName, maxOpen, maxIdle)
		if err != nil {
			return nil, err // only configuration errors
		}

		if err = db.Ping(); err != nil {
			if db != nil {
				db.Close()
			}
			continue
		}

		ticker.Stop()
		break
	}

	if err != nil {
		return nil, err
	}

	return db, nil
}

// Setup a sqlx.DB connection.
func dial(driverName, dataSourceName string, maxOpen, maxIdle int) (*sqlx.DB, error) {
	db, err := sqlx.Open(driverName, dataSourceName)
	if err != nil {
		return nil, err // note: only returns misconfiguration errors
	}

	db.SetMaxOpenConns(maxOpen)
	db.SetMaxIdleConns(maxIdle)

	return db, nil
}
