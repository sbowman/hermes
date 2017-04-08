package hermes_test

import (
	"database/sql"
	"testing"
)

func TestTransaction(t *testing.T) {
	db := connect(t)
	defer db.Close()

	if _, err := db.Exec("create table test_tx(wonder varchar(64))"); err != nil {
		t.Fatalf("Unable to create test_tx table: %s", err)
	}
	defer func() {
		db.Exec("drop table test_tx")
	}()

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("Unable to start transaction :%s", err)
	}

	if _, err := tx.Exec("insert into test_tx values ($1)", "Sphinx"); err != nil {
		t.Errorf("Unable to insert via transaction: %s", err)
	}

	var wonder string

	row, err := tx.Row("select wonder from test_tx where wonder = $1", "Sphinx")
	if err != nil {
		t.Errorf("Failed to query test_tx for wonder: %s", err)
	}

	if err := row.Scan(&wonder); err != nil {
		t.Errorf("Unable to get wonder: %s", err)
	}

	// Should rollback...
	tx.Close()

	var check string

	row, err = db.Row("select wonder from test_tx where wonder = $1", "Sphinx")
	if err != nil {
		t.Errorf("Failed to query test_tx for wonder: %s", err)
	}

	if err = row.Scan(&check); err == nil {
		if err != sql.ErrNoRows {
			t.Errorf(`Expected "%s"; got %s`, sql.ErrNoRows, err)
		}

		if check == "Sphinx" {
			t.Error("Record not deleted")
		}
	}
}

func TestDeepRollback(t *testing.T) {

}

func TestDeepCommit(t *testing.T) {

}
