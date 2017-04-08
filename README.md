# Hermes 

Hermes wraps the `jmoiron/sqlx` *sqlx.DB and *sqlx.Tx models in a common 
interface, hermes.Conn.  Makes it easier to build small functions that can
be aggregated and used in a single transaction, as well as for testing.

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
