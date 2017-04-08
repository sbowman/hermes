# Hermes 

Hermes wraps the `database/sql` DB and Tx models in a common interface.

## Usage

    func Sample(conn hermes.Conn, name string) error {
        tx, err := conn.Begin()
        if err != nil {
            return err
        }
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
        defer tx.Close()

        if err := Sample(tx, "Frank"); err != nil {
            fmt.Println("Frank failed!", err.Error())
            return
        }

        // Don't forget to commit, or you'll automatically rollback on 
        // tx.Close()!
        tx.Commit() 
    }

## Testing

To run the test cases, create a `hermes_test` database before running the tests.
If a test fails, there is a chance data will remain in the database, but 
typically the database should be empty when all tests are complete.
