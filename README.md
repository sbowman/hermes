# Hermes 1.2.4

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

Using a `hermes.Conn` parameter in a function also opens up *in situ* testing
of database functionality.  You can create a transaction in the test case and
pass it to a function that takes a `hermes.Conn`, run any tests on the results
of that function, and simply let the transaction rollback at the end of the
test to clean up.

    var Conn hermes.Conn
    
    // We'll just open one database connection pool to speed up testing, so 
    // we're not constantly opening and closing connections.
    func TestMain(m *testing.M) {
	    conn, err := hermes.Connect("postgres", DBTestURI, 5, 2)
	    if err != nil {
	        fmt.Fprintf(os.Stderr, "Unable to open a database connection: %s\n", err)
	        os.Exit(1)
    	}
    	defer conn.Close()
    	
    	Conn = conn
    	
    	os.Exit(m.Run())
    }
    
    // Test getting a user account from the database.  The signature for the
    // function is:  `func GetUser(conn hermes.Conn, email string) (User, error)`
    // 
    // Passing a hermes.Conn value to the function means we can pass in either
    // a reference to the database pool and really update the data, or we can
    // pass in the same transaction reference to both the SaveUser and GetUser
    // functions.  If we use a transaction, we can let the transaction roll back 
    // after we test these functions, or at any failure point in the test case,
    // and we know the data is cleaned up. 
    func TestGetUser(t *testing.T) {
        u := User{
            Email: "jdoe@nowhere.com",
            Name: "John Doe",
        }
        
        tx, err := Conn.Begin()
        if err != nil {
            t.Fatal(err)
        }
        defer tx.Close()
        
        if err := db.SaveUser(tx, u); err != nil {
            t.Fatalf("Unable to create a new user account: %s", err)
        }
        
        check, err := db.GetUser(tx, u.Email)
        if err != nil {
            t.Fatalf("Failed to get user by email address: %s", err)
        }
        
        if check.Email != u.Email {
            t.Errorf("Expected user email to be %s; was %s", u.Email, check.Email)
        } 
        
        if check.Name != u.Name {
            t.Errorf("Expected user name to be %s; was %s", u.Name, check.Name)
        } 
        
        // Note:  do nothing...when the test case ends, the `defer tx.Close()`
        // is called, and all the data in this transaction is rolled back out.
    }
    
Using transactions, even if a test case fails a returns prematurely, the 
database transaction is automatically closed, thanks to `defer`.  The database 
is cleaned up without any fuss or need to remember to delete the data you
created at any point in the test. 
  
## Confirm (1.2.3)

If the network environment is unstable, Hermes may be configured to retry 
connections from the connection pool if those pooled connections lose their
connectivity to the database.  

To enable connection confirmations, set the `hermes.Confirm` global variable to  
a number greater than 0:

    // Create a connection pool with max 10 connections, min 2 idle connections...
    conn, err := hermes.Connect("postgres", 
        "postgres://postgres@127.0.0.1/engaged?sslmode=disable&connect_timeout=10", 
        10, 2)
    if err != nil {
        return err
    }
    
    // Check each database connection at least twice before panicking
    hermes.Confirm = 2

When confirmation is enabled, Hermes pings the database prior to making any
database requests (begin a transaction, select, insert, etc.).  If the ping 
fails, Hermes waits a moment and tries again, up to the number of times 
specified in `hermes.Confirm`.  Each try, the `sql.Ping()` function tries to 
reconnect to the database.

If Hermes can't open the database connection again after trying repeatedly, it
panics and crashes the application.  Ideally systemd, Kubernetes, or whatever 
monitor is watching the application will restart the app and clear up the cause 
of the problem, or at least alert someone there's a problem. 

The `hermes.Confirm` functionality should be coupled with a `connect_timeout`
value in the PostgreSQL configuration, or the equivalent for whatever database
is being used. 

This check is not performed with queries made within a transaction.  If the 
connection is lost mid-transaction, there is no point trying to reconnect, as 
the transaction is lost.  At that point, the transaction should simply fail.

There is the performance hit of an additional `sql.Ping()` request with nearly 
every database query.  If you don't need this functionality, we recommend you
don't enable it.   

By default this functionality is *disabled*.

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

## Savepoints (1.2.4)

Hermes 1.2.4 adds support for transaction "savepoints."  A savepoint acts like a
bookmark in a transaction that stays around until the transaction ends.  It 
allows a transaction to partially rollback to the savepoint.  

At any point in a transaction, use `Conn.Savepoint` to create a savepoint in the 
transaction.  The savepoint is assigned a random identifer, which is the 
returned by the `Conn.Savepoint` function.  When you wish to rollback to this 
savepoint, call `Conn.RollbackTo(savepointID)`.  

For example:

    tx, err := db.Begin()
    if err != nil {
        return err
    }
    defer tx.Close()
    
    // ... do some work ...
    
    savepoint, err := tx.Savepoint
    if err != nil {
        // If the savepoint can't be created, rollback the entire transaction
        return err
    }
    
    // ... do additional work ...
    
    // Whoops!  Something went wrong in the additional work!
    //
    // Also note that RollbackTo does return an error, which you should probably
    // catch.
    tx.RollbackTo(savepoint)
    
    // Continue working; the transaction is still valid, but we just lost the 
    // additional work.

Savepoints remain valid once created.  You can create a savepoint, rollback to
the savepoint, do more work, and rollback to the savepoint again.

Cursors created before a savepoint are unaffected by a rollback to the 
savepoint, even if they have been manipulated after the savepoint was created.
Cursors created after a savepoint are closed when the savepoint is rolled back.
See the documentation below for more details.  

While `Savepoint()` and `RollbackTo()` are part of the `hermes.Conn` interface,
when called on a `hermes.DB` object they do nothing.

Savepoints have only been tested with PostgreSQL, though they should also work
with MySQL.  

### Usages

Savepoints can be very useful for database testing.  For example, you can create
a Hermes transaction (`hermes.Tx`) at the start of a test case containing 
multiple scenarios, setup your initial data, then create a savepoint before each 
scenario you're testing.  

After each scenario, simply rollback to the savepoint and test the next 
scenario.  At the end of the test case, allow the transaction to close 
(`defer tx.Close()`) and rollback all the data, leaving the database in a 
pristine state.  


For example:

    // Test uniqueness when saving users.  Based off the example above.  Both
    // `User.Email` and `User.Name` must be unique. 
    func TestUserUniquness(t *testing.T) {
        u := User{
            Email: "jdoe@nowhere.com",
            Name: "John Doe",
        }
        
        tx, err := Conn.Begin()
        if err != nil {
            t.Fatal(err)
        }
        defer tx.Close()
        
        // Create our valid user account
        if err := db.SaveUser(tx, u); err != nil {
            t.Fatalf("Unable to create a new user account: %s", err)
        }
        
        // Leave a savepoint to rollback to
        savepoint, err := tx.Savepoint()
        if err != nil {
            t.Fatalf("Couldn't create a savepoint: %s", err)
        }
        
        // Test email uniquness
        other := User{
            Email: "jdoe@nowhere.com,
            Name: "Another name",
        }
        
        if err = db.SaveUser(tx, other); err == nil {
            t.Error("Appears that user emails lack a uniqueness constraint in the database")
        }
        
        // Just to be safe, rollback to our valid user and try the name
        if err = tx.RollbackTo(savepoint); err != nil {
            t.Fatalf("Unable to rollback to savepoint: %s", err)
        }
        
        // Test name uniqueness
        other = User{
            Email: "another@nowhere.com",
            Name: "John Doe",
        }

        if err = db.SaveUser(tx, other); err == nil {
            t.Error("Appears that user names lack a uniqueness constraint in the database")
        }

        // Again, let the test case end and allow `defer tx.Close()` to wipe all
        // the data and savepoints created by the transaction; no need for any
        // delete calls.        
    }

### Additional information

For more information on savepoints, see the PostgreSQL documentation:

* https://www.postgresql.org/docs/12/sql-savepoint.html
* https://www.postgresql.org/docs/12/sql-rollback-to.html

Or the MySQL documentation:

* https://dev.mysql.com/doc/refman/8.0/en/savepoint.html

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
