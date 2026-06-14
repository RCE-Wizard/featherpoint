package store

import (
	"context"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

type WebUser struct {
	ID           string
	Username     string
	PasswordHash string
	Role         string
}

// GetUserByUsername returns a web user by username.
func (db *DB) GetUserByUsername(ctx context.Context, username string) (*WebUser, error) {
	u := &WebUser{}
	err := db.Pool.QueryRow(ctx,
		`SELECT id, username, password_hash, role FROM web_users WHERE username=$1`,
		username,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role)
	if err != nil {
		return nil, err
	}
	return u, nil
}

// CreateUser creates a new web user with a bcrypt-hashed password.
func (db *DB) CreateUser(ctx context.Context, username, password, role string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	_, err = db.Pool.Exec(ctx,
		`INSERT INTO web_users (username, password_hash, role) VALUES ($1,$2,$3)`,
		username, string(hash), role,
	)
	return err
}

// CheckPassword returns true if the plain-text password matches the stored hash.
func CheckPassword(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}
