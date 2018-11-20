package hermes_test

import (
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/sbowman/hermes"
)

const (
	driver   = "postgres"
	database = "postgres://postgres@127.0.0.1/hermes_test?sslmode=disable&connect_timeout=10"
)

func init() {
	hermes.MaxRetryTime = 1 * time.Second
}

// Return a connection to the database.  Will generate a fatal error if unable
// to connect.
func connect(t *testing.T) *hermes.DB {
	db, err := hermes.Connect(driver, database, 5, 1)
	if err != nil {
		t.Fatalf("Failed to connect to the hermes_test database: %s", err)
	}

	return db
}

func TestConnection(t *testing.T) {
	db := connect(t)
	defer db.Close()

	if err := db.Ping(); err != nil {
		t.Fatalf("Unable to ping the database: %s", err)
	}
}

func TestBadDatabase(t *testing.T) {
	_, err := hermes.Connect(driver, "postgres://postgres@127.0.0.1/nemo?sslmode=disable&connect_timeout=10", 5, 1)
	if err == nil {
		t.Fatalf(`Missing "nemo" database didn't generate an error!  Does it exist?`)
	}
}
