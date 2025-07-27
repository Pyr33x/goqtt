package auth

import (
	"database/sql"
	"errors"

	"github.com/pyr33x/goqtt/pkg/er"
	h "github.com/pyr33x/goqtt/pkg/hash"
)

type Store struct {
	db *sql.DB
}

func New(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) Authenticate(username, password string) error {
	var hash string

	err := s.db.QueryRow("SELECT secret FROM users WHERE username = ?", username).Scan(&hash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &er.Err{
				Context: "Auth",
				Message: er.ErrUserNotFound,
			}
		}
	}

	if h.VerifyPasswd(hash, password) {
		return &er.Err{
			Context: "Auth",
			Message: er.ErrInvalidPassword,
		}
	}

	return nil
}
