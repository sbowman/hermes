package hermes_test

import (
	"testing"

	"github.com/sbowman/hermes"
)

// Open but don't test the connection; we want to try it with a query
func unchecked(t *testing.T) *hermes.DB {
	db, err := hermes.ConnectUnchecked(driver, "postgres://postgres@127.0.0.1/nemo?sslmode=disable&connect_timeout=10", 5, 1)
	if err != nil {
		t.Fatal(err)
	}

	return db
}

func TestExecWithFailure(t *testing.T) {
	db := unchecked(t)
	defer db.Close()

	var failed bool

	db.OnFailure = func(db *hermes.DB, err error) {
		failed = true
	}

	_, err := db.Exec("create table sample(name varchar(64) not null)")
	if err == nil {
		t.Error("Expected exec to fail with a bad connection")
	}

	if !hermes.DidConnectionFail(err) {
		t.Errorf("Expected a connection failure; was %s", err)
	}

	if !failed {
		t.Error("Failed to call the db.OnError function!")
	}
}

func TestGetWithFailure(t *testing.T) {
	db := unchecked(t)
	defer db.Close()

	var failed bool

	db.OnFailure = func(db *hermes.DB, err error) {
		failed = true
	}

	var person struct {
		Name string `db:"name"`
		Age  int    `db:"age"`
	}

	err := db.Get(&person, "select * from test_get limit 1")
	if err == nil {
		t.Error("Expected get to fail with a bad connection")
	}

	if !hermes.DidConnectionFail(err) {
		t.Errorf("Expected a connection failure; was %s", err)
	}

	if !failed {
		t.Error("Failed to call the db.OnError function!")
	}
}
