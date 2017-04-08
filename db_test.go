package hermes_test

import "testing"

func TestExec(t *testing.T) {
	db := connect(t)
	defer db.Close()

	if _, err := db.Exec("create table sample(name varchar(64) not null)"); err != nil {
		t.Errorf("Unable to create sample table in database: %s", err)
	}

	if _, err := db.Exec("drop table sample"); err != nil {
		t.Errorf("Failed to remove the sample table: %s", err)
	}
}

func TestGet(t *testing.T) {
	db := connect(t)
	defer db.Close()

	if _, err := db.Exec("create table test_get(name varchar(64), age int)"); err != nil {
		t.Fatalf("Unable to create test_get table: %s", err)
	}
	defer func() {
		db.Exec("drop table test_get")
	}()

	if _, err := db.Exec("insert into test_get values ($1, $2)", "James", 35); err != nil {
		t.Fatalf("Unable to insert data into test_get: %s", err)
	}

	var person struct {
		Name string `db:"name"`
		Age  int    `db:"age"`
	}

	if err := db.Get(&person, "select * from test_get limit 1"); err != nil {
		t.Errorf("Unable to get entry: %s", err)
	}

	if person.Name != "James" {
		t.Errorf(`Expecting a name of "James"; was "%s"`, person.Name)
	}

	if person.Age != 35 {
		t.Errorf(`Expecting an age of "35"; was "%d"`, person.Age)
	}
}

func TestQuery(t *testing.T) {
	// TODO
}

func TestPrepare(t *testing.T) {
	// TODO
}
