package hermes_test

import (
	"fmt"
	"testing"
)

// Test using savepoints for partial rollbacks.
func TestSavepoint(t *testing.T) {
	db := connect(t)
	defer db.Close()

	fmt.Println("here")
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}

	if _, err := tx.Exec("create table test_savepoint(name varchar(64) not null)"); err != nil {
		t.Errorf("Unable to create test_savepoint table in database: %s", err)
	}

	if _, err := tx.Exec("insert into test_savepoint (name) values ('abc')"); err != nil {
		t.Errorf("Unable to insert first record in test_savepoint: %s", err)
	}

	savepoint, err := tx.Savepoint()
	if err != nil {
		t.Errorf("Unable to create savepoint: %s", err)
	}

	if savepoint == "" {
		t.Error("invalid savepoint ID")
	}

	if _, err := tx.Exec("insert into test_savepoint (name) values ('def')"); err != nil {
		t.Errorf("Unable to insert first record in test_savepoint: %s", err)
	}

	var count int
	if err := tx.Get(&count, "select count(1) from test_savepoint"); err != nil {
		t.Errorf("Failed to get test_savepoint record count: %s", err)
	}

	if err := tx.RollbackTo(savepoint); err != nil {
		t.Errorf("Unable to rollback to savepoint: %s", err)
	}

	if count != 2 {
		t.Errorf("Expected two records; got %d", count)
	}

	if err := tx.Get(&count, "select count(1) from test_savepoint"); err != nil {
		t.Errorf("Failed to get test_savepoint record count: %s", err)
	}

	if count != 1 {
		t.Errorf("Expected one record; got %d", count)
	}
}
