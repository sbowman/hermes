# Hermes 

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
        conn, err := hermes.Connect("postgres", "postgres://postgres@127.0.0.1/engaged?sslmode=disable&connect_timeout=10")
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
