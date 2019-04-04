# Hermes 1.2.0

Hermes wraps the `jmoiron/sqlx` *sqlx.DB and *sqlx.Tx models in a common 
interface, hermes.Conn.  Makes it easier to build small functions that can
be aggregated and used in a single transaction, as well as for testing.

[![Godoc](http://img.shields.io/badge/godoc-reference-blue.svg?style=flat)](https://godoc.org/github.com/sbowman/hermes) 
![license](http://img.shields.io/badge/license-MIT-red.svg?style=flat)

## Usage

    func Sample(conn hermes.Conn, name string) error {
        tx, err := conn.Begin()
        if err != nil {
            return err
        }
        
        // Will automatically rollback if an error short-circuits the return
        // before tx.Commit() is called...
        defer tx.Close() 

        res, err := conn.Exec("insert into samples (name) values ($1)", name)
        if err != nil {
            return err
        }

        check, err := res.RowsAffected()
        if check == 0 {
            return fmt.Errorf("Failed to insert row (%s)", err)
        }

        return tx.Commit()
    }

    func main() {
        // Create a connection pool with max 10 connections, min 2 idle connections...
        conn, err := hermes.Connect("postgres", 
            "postgres://postgres@127.0.0.1/engaged?sslmode=disable&connect_timeout=10", 
            10, 2)
        if err != nil {
            return err
        }

        // This works...
        if err := Sample(conn, "Bob"); err != nil {
            fmt.Println("Bob failed!", err.Error())
        }

        // So does this...
        tx, err := conn.Begin()
        if err != nil {
            panic(err)
        }

        // Will automatically rollback if call to sample fails...
        defer tx.Close() 

        if err := Sample(tx, "Frank"); err != nil {
            fmt.Println("Frank failed!", err.Error())
            return
        }

        // Don't forget to commit, or you'll automatically rollback on 
        // "defer tx.Close()" above!
        tx.Commit() 
    }

## OnFailure (1.1.x)

Hermes supports an `OnFailure` function that may be called any time a database
error appears to be an unrecoverable connection or server failure.  This
function is set on the database connection (`hermes.DB`), and may be customized
to your environment with custom handling or logging functionality.

    // Create a connection pool with max 10 connections, min 2 idle connections...
    conn, err := hermes.Connect("postgres", 
        "postgres://postgres@127.0.0.1/engaged?sslmode=disable&connect_timeout=10", 
        10, 2)
    if err != nil {
        return err
    }
    
    // In a Kubernetes deployment, this will cause the app to shutdown and let
    // Kubernetes restart the pod...
    conn.OnFailure = hermes.ExitOnFailure

    // If the connection fails when conn.Exec is called, hermes.ExitOnFailure
    // is called, the application exits, and Kubernetes restarts the app, 
    // allowing the app to try to reconnect to the database.
    if _, err := conn.Exec("...."); err != nil {
        return err
    }

### DidConnectionFail (1.1.x)

If `OnFailure` is not defined, Hermes simply returns the error as normal,
expecting the application to do something with it.  In these situations, there
is a function in Hermes that can check if the error returned by `lib/pq` is a
connection error: `hermes.DidConnectionFail`.  Pass the error to that, and if
it's a connection error, the function returns true.

## Transaction Timers (1.2.x)

Hermes supports configurable transaction timers to watch transactions and warn
the developer if the transaction was open longer than expected.  This can be
useful in testing for transactions that weren't properly cleaned up.

Simply call `hermes.EnableTimeouts(time.Duration, bool)` with the worst-case
expected transaction duration (presumably less than a second).

    func main() {
        // If a transaction takes longer than one second, you'll see an 
        // error message in stderr
        hermes.EnableTimeouts(time.Second, false)
        
        // Create a connection pool with max 10 connections, min 2 idle 
        // connections...
        conn, err := hermes.Connect("postgres", 
            "postgres://postgres@127.0.0.1/engaged?sslmode=disable&connect_timeout=10", 
            10, 2)
        if err != nil {
            return err
        }

        tx, err := conn.Begin()
        if err != nil {
            panic(err)
        }

        // Oops...we forgot tx.Close()!
       
        // This will cause an error message to print out to stderr
        time.Sleep(5 * time.Second)
    }

If you pass in `true` to the `hermes.EnableTimeouts` function, the application
will panic when a transaction times out.

You may disable transaction timers using the `hermes.DisableTimeouts()` call. 

**Do not run transaction timers in production!** There is overhead with the
timers enabled; enabling them in production could cause performance and memory
issues under load (each transaction will get a time.Timer).

## Testing

Testing requires the lib/pq library, a PostgreSQL database, and a test database
called "hermes_test".

(A future release may mock a database driver.)

### On a Mac...

    $ brew install postgresql
    $ createdb hermes_test
    $ cd $GOPATH/src/github.com/sbowman/hermes
    $ go get
    $ go test
