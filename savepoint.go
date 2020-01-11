package hermes

import (
	"encoding/hex"

	"github.com/google/uuid"
)

// Savepoint does nothing on the DB.
func (db *DB) Savepoint() (string, error) {
	id := GenerateSavepointID()
	return id, nil
}

// RollbackTo does nothng on DB.
func (db *DB) RollbackTo(savepointID string) error {
	return nil
}

// Savepoint creates a new savepoint that can be rolled back to.
func (tx *Tx) Savepoint() (string, error) {
	id := GenerateSavepointID()
	_, err := tx.Exec("SAVEPOINT " + id)
	if err != nil {
		return "", err
	}

	return id, nil
}

// RollbackTo rolls back to the savepoint.
func (tx *Tx) RollbackTo(savepointID string) error {
	_, err := tx.Exec("ROLLBACK TO SAVEPOINT " + savepointID)
	if err != nil {
		return err
	}

	return nil
}

// GenerateSavepointID generates a globally unique ID to use with a savepoint.
// Note that savepoint identifiers are prefixed with "point_" just in case the
// generated ID starts with a number.
func GenerateSavepointID() string {
	id := uuid.New()
	return "point_" + hex.EncodeToString(id[:])
}
