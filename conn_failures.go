package hermes

import (
	"net"
	"os"

	"github.com/lib/pq"
)

// FailureFn defines the template for the check function called when the
// database action returns a connection-related error.  Useful for trapping
// connection failures and resetting the database connection pool.
type FailureFn func(db *DB, err error)

// CheckPanic panics when the connection fails or the database server errors.
// If your application traps panics, see ExitOnFailure.
func PanicOnFailure(db *DB, err error) {
	panic(err)
}

// ExitOnFailure forces an `os.Exit(2)` when the connection fails.  This can be
// useful in applications that trap panics, such as in HTTP middleware.
func ExitOnFailure(db *DB, err error) {
	os.Exit(2)
}

// DidConnectionFail checks the error message returned from a database request
// Used by hermes.PanicDB in several instances.  May be used by applications
// with other connection types, or to test queries not covered by PanicDB, such
// as scanning row results.
//
// If exit is nil, simply returns the error, skipping the check.
func DidConnectionFail(err error) bool {
	if err == nil {
		return false
	}

	switch e := err.(type) {
	case *net.OpError:
		return true

	case *pq.Error:
		code := e.Code[0:2]
		if code == "08" || // connection failed
			code == "3D" || // database not found
			code == "53" || // insufficient resources (disk, memory, etc.)
			code == "57" || // operator intervention
			code == "58" || // system error (external to PostgreSQL)
			code == "XX" { // internal server error
			return true
		}
	}

	return false
}
