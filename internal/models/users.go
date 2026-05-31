package models

import (
	"database/sql"
	"errors"

	"golang.org/x/crypto/bcrypt"
)

func hashPassword(pw string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	return string(b), err
}

func CheckPasswordHash(hash, pw string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pw)) == nil
}

func (d *DB) GetUserByUsername(username string) (*User, error) {
	var u User
	var must int
	var created string
	err := d.QueryRow(`SELECT id, username, password_hash, role, must_change_password, created_at FROM users WHERE username = ?`, username).
		Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &must, &created)
	if err != nil {
		return nil, err
	}
	u.MustChangePassword = must == 1
	u.CreatedAt = parseTime(created)
	return &u, nil
}

func (d *DB) GetUserByID(id int64) (*User, error) {
	var u User
	var must int
	var created string
	err := d.QueryRow(`SELECT id, username, password_hash, role, must_change_password, created_at FROM users WHERE id = ?`, id).
		Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &must, &created)
	if err != nil {
		return nil, err
	}
	u.MustChangePassword = must == 1
	u.CreatedAt = parseTime(created)
	return &u, nil
}

func (d *DB) ListUsers() ([]User, error) {
	rows, err := d.Query(`SELECT id, username, password_hash, role, must_change_password, created_at FROM users ORDER BY username`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []User
	for rows.Next() {
		var u User
		var must int
		var created string
		if err := rows.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &must, &created); err != nil {
			return nil, err
		}
		u.MustChangePassword = must == 1
		u.CreatedAt = parseTime(created)
		list = append(list, u)
	}
	return list, rows.Err()
}

func (d *DB) CreateUser(username, password, role string) (*User, error) {
	if role != "admin" && role != "viewer" {
		return nil, errors.New("invalid role")
	}
	hash, err := hashPassword(password)
	if err != nil {
		return nil, err
	}
	res, err := d.Exec(`INSERT INTO users (username, password_hash, role) VALUES (?, ?, ?)`, username, hash, role)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return d.GetUserByID(id)
}

func (d *DB) UpdateUserPassword(id int64, password string, clearMustChange bool) error {
	hash, err := hashPassword(password)
	if err != nil {
		return err
	}
	if clearMustChange {
		_, err = d.Exec(`UPDATE users SET password_hash = ?, must_change_password = 0 WHERE id = ?`, hash, id)
	} else {
		_, err = d.Exec(`UPDATE users SET password_hash = ? WHERE id = ?`, hash, id)
	}
	return err
}

func (d *DB) DeleteUser(id int64) error {
	var n int
	_ = d.QueryRow(`SELECT COUNT(*) FROM users WHERE role = 'admin'`).Scan(&n)
	var role string
	_ = d.QueryRow(`SELECT role FROM users WHERE id = ?`, id).Scan(&role)
	if role == "admin" && n <= 1 {
		return errors.New("cannot delete last admin")
	}
	_, err := d.Exec(`DELETE FROM users WHERE id = ?`, id)
	return err
}

func (d *DB) UpdateUserRole(id int64, role string) error {
	_, err := d.Exec(`UPDATE users SET role = ? WHERE id = ?`, role, id)
	return err
}

var ErrNotFound = sql.ErrNoRows
