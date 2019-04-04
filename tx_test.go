package hermes_test

import (
	"database/sql"
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/sbowman/hermes"
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
	// see tx.Close() below

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

// Will transactions alert if it lasts too long (in dev mode)?
func TestTxTimer(t *testing.T) {
	timeout := 100 * time.Millisecond

	hermes.EnableTimeouts(timeout, false)
	defer hermes.DisableTimeouts()

	db := connect(t)
	defer db.Close()

	stderr := os.Stderr
	defer func() {
		os.Stderr = stderr
	}()

	r, w, _ := os.Pipe()
	os.Stderr = w

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("Unable to create transaction: %s", err)
	}
	defer tx.Close()

	// Should trip the timer...
	time.Sleep(timeout * 2)

	w.Close()
	out, _ := ioutil.ReadAll(r)
	output := string(out)

	if !strings.Contains(output, "Transaction lifetime exceeded timeout") {
		t.Error("Failed to timeout the transaction")
	}
}

func TestDeepRollback(t *testing.T) {
	db := connect(t)
	defer db.Close()

	if _, err := db.Exec("create table test_deep_r(wonder varchar(64))"); err != nil {
		t.Fatalf("Unable to create test_deep_r table: %s", err)
	}
	defer func() {
		db.Exec("drop table test_deep_r")
	}()

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("Unable to start transaction :%s", err)
	}
	// no defer; see tx.Close() below...

	if _, err := tx.Exec("insert into test_deep_r values ($1)", "Mahogany"); err != nil {
		t.Errorf("Unable to insert via transaction: %s", err)
	}

	err = func(conn hermes.Conn) error {
		txn, err := conn.Begin()
		if err != nil {
			return err
		}
		defer txn.Close()

		if _, err = txn.Exec("insert into test_deep_r values ($1)", "Oak"); err != nil {
			return err
		}

		txn.Rollback()
		return nil
	}(tx)

	if err != nil {
		t.Fatalf("Deep tx failed unexpectedly: %s", err)
	}

	if !tx.RolledBack() {
		t.Error("Expected transaction to indicate it was rolled back")
	}

	_, err = tx.Query("select wonder from test_deep_r")
	if err != hermes.ErrTxRolledBack {
		t.Errorf(`Expected error "%s"; got "%s"`, hermes.ErrTxRolledBack, err)
	}

	if err := tx.Commit(); err != hermes.ErrTxRolledBack {
		t.Errorf("Expected rolled back error")
	}

	if err := tx.Rollback(); err != nil {
		t.Errorf("Unexpected error from rollback: %s", err)
	}

	tx.Close()

	rows, _ := db.Query("select wonder from test_deep_r")
	if rows.Next() {
		t.Error("Unexpected results; was table cleared?")
	}
}

func TestDeepCommit(t *testing.T) {
	db := connect(t)
	defer db.Close()

	if _, err := db.Exec("create table test_deep_c(wonder varchar(64))"); err != nil {
		t.Fatalf("Unable to create test_deep_c table: %s", err)
	}
	defer func() {
		db.Exec("drop table test_deep_c")
	}()

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("Unable to start transaction :%s", err)
	}
	defer tx.Close()

	if _, err := tx.Exec("insert into test_deep_c values ($1)", "Mahogany"); err != nil {
		t.Errorf("Unable to insert via transaction: %s", err)
	}

	err = func(conn hermes.Conn) error {
		txn, err := conn.Begin()
		if err != nil {
			return err
		}
		defer txn.Close()

		if _, err = txn.Exec("insert into test_deep_c values ($1)", "Oak"); err != nil {
			return err
		}

		txn.Commit()
		return nil
	}(tx)

	if err != nil {
		t.Fatalf("Deep tx failed: %s", err)
	}

	rows, err := tx.Query("select wonder from test_deep_c")
	if err != nil {
		t.Fatalf("Failed to query database: %s", err)
	}

	var counter int
	for rows.Next() {
		var w string
		if err := rows.Scan(&w); err != nil {
			t.Errorf("Unable to load wonder value: %s", err)
			continue
		}

		if w == "Mahogany" || w == "Oak" {
			counter++
		} else {
			t.Errorf("Unexpected value, %s", w)
		}
	}

	if counter != 2 {
		t.Error("Didn't find the expected values!")
	}
}

func TestMultipleCommit(t *testing.T) {
	db := connect(t)
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("Unable to start transaction :%s", err)
	}
	defer tx.Close()

	tx.Exec("select 1")

	if err := tx.Commit(); err != nil {
		t.Error(err)
	}

	if err := tx.Commit(); err != hermes.ErrTxCommitted {
		t.Error("Expected error already committed on second commit")
	}

	if err := tx.Rollback(); err != hermes.ErrTxCommitted {
		t.Error("Expected error already committed on rollback")
	}
}

func TestMultipleRollback(t *testing.T) {
	db := connect(t)
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("Unable to start transaction :%s", err)
	}
	defer tx.Close()

	tx.Exec("select 1")

	if err := tx.Rollback(); err != nil {
		t.Error(err)
	}

	if err := tx.Rollback(); err != nil {
		t.Errorf("Unexpected error on second rollback: %s", err)
	}

	if err := tx.Commit(); err != hermes.ErrTxRolledBack {
		t.Error("Expected error already rolled back")
	}
}

func TestAutoRollback(t *testing.T) {
	db := connect(t)
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("Unable to start transaction :%s", err)
	}

	tx.Exec("select 1")

	if err := tx.Close(); err != nil {
		t.Error(err)
	}

	if !tx.RolledBack() {
		t.Error("Expected to be rolled back")
	}

	if err := tx.Rollback(); err != nil {
		t.Errorf("Unexpected error on second rollback: %s", err)
	}

	if err := tx.Commit(); err != hermes.ErrTxRolledBack {
		t.Error("Expected error already rolled back")
	}
}

func TestDeepAutoRollback(t *testing.T) {
	db := connect(t)
	defer db.Close()

	if _, err := db.Exec("create table test_deep_ar(wonder varchar(64))"); err != nil {
		t.Fatalf("Unable to create test_deep_ar table: %s", err)
	}
	defer func() {
		db.Exec("drop table test_deep_ar")
	}()

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("Unable to start transaction :%s", err)
	}
	// no defer; see tx.Close() below...

	if _, err := tx.Exec("insert into test_deep_ar values ($1)", "Peanuts"); err != nil {
		t.Errorf("Unable to insert via transaction: %s", err)
	}

	err = func(conn hermes.Conn) error {
		txn, err := conn.Begin()
		if err != nil {
			return err
		}
		defer txn.Close()

		if _, err = txn.Exec("insert into test_deep_ar values ($1)", "Almonds"); err != nil {
			return err
		}

		// NOTE: no commit; should auto-rollback...
		return nil
	}(tx)

	if err != nil {
		t.Fatalf("Deep tx failed unexpectedly: %s", err)
	}

	if !tx.RolledBack() {
		t.Error("Expected transaction to indicate it was rolled back")
	}

	_, err = tx.Query("select wonder from test_deep_ar")
	if err != hermes.ErrTxRolledBack {
		t.Errorf(`Expected error "%s"; got "%s"`, hermes.ErrTxRolledBack, err)
	}

	if err := tx.Commit(); err != hermes.ErrTxRolledBack {
		t.Errorf("Expected rolled back error")
	}

	if err := tx.Rollback(); err != nil {
		t.Errorf("Unexpected error from rollback: %s", err)
	}

	tx.Close()

	rows, _ := db.Query("select wonder from test_deep_ar")
	if rows.Next() {
		t.Error("Unexpected results; was table cleared?")
	}
}
