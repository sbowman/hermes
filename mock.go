package hermes

import (
	"github.com/jmoiron/sqlx"
)

// Create a Mock Connection. This connection will ignore all calls to Commit
// and always rollback on close
func Mock(driverName, dataSourceName string, maxOpen, maxIdle int) (*MockDB, error) {
	db, err := sqlx.Connect(driverName, dataSourceName)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(maxOpen)
	db.SetMaxIdleConns(maxIdle)

	return &MockDB{
		&DB{
			name: dataSourceName,
			internal: db,
		},
	}, nil
}

type MockDB struct {*DB}

func (db *MockDB) Begin() (Conn, error) {
	c, err := db.DB.Begin()
	if err != nil {
		return nil, err
	}
	tx := c.(*Tx)
	return &MockTx{tx}, nil
}

type MockTx struct {*Tx}

// ignore all commits
func (tx *MockTx) Commit() error {
	return nil
}

// always rollback on close
func (tx *MockTx) Close() error {
	return tx.Tx.Rollback()
}


