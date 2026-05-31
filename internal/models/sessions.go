package models

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"
)

const defaultSessionTTL = 24 * time.Hour

type Session struct {
	ID         int64     `json:"id"`
	UserID     int64     `json:"user_id"`
	CreatedAt  time.Time `json:"created_at"`
	ExpiresAt  time.Time `json:"expires_at"`
	LastSeenAt time.Time `json:"last_seen_at"`
	UserAgent  string    `json:"user_agent"`
	IPAddress  string    `json:"ip_address"`
}

func hashSessionToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func newSessionToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// CreateSession inserts a new session (multi-session: each login gets its own row).
func (d *DB) CreateSession(userID int64, userAgent, ip string, ttl time.Duration) (token string, err error) {
	if ttl <= 0 {
		ttl = defaultSessionTTL
	}
	token, err = newSessionToken()
	if err != nil {
		return "", err
	}
	expires := time.Now().Add(ttl)
	_, err = d.Exec(
		`INSERT INTO sessions (user_id, token_hash, expires_at, user_agent, ip_address) VALUES (?, ?, ?, ?, ?)`,
		userID, hashSessionToken(token), expires.Format("2006-01-02 15:04:05"), userAgent, ip,
	)
	if err != nil {
		return "", err
	}
	return token, nil
}

// ValidateSession returns the session and user for a bearer/cookie token.
func (d *DB) ValidateSession(token string) (*Session, *User, error) {
	if token == "" {
		return nil, nil, fmt.Errorf("empty token")
	}
	hash := hashSessionToken(token)
	var s Session
	var created, expires, lastSeen string
	err := d.QueryRow(
		`SELECT id, user_id, created_at, expires_at, last_seen_at, user_agent, ip_address FROM sessions WHERE token_hash = ?`,
		hash,
	).Scan(&s.ID, &s.UserID, &created, &expires, &lastSeen, &s.UserAgent, &s.IPAddress)
	if err != nil {
		return nil, nil, err
	}
	s.CreatedAt = parseTime(created)
	s.ExpiresAt = parseTime(expires)
	s.LastSeenAt = parseTime(lastSeen)
	if time.Now().After(s.ExpiresAt) {
		_, _ = d.Exec(`DELETE FROM sessions WHERE id = ?`, s.ID)
		return nil, nil, fmt.Errorf("session expired")
	}
	_, _ = d.Exec(`UPDATE sessions SET last_seen_at = datetime('now') WHERE id = ?`, s.ID)
	u, err := d.GetUserByID(s.UserID)
	if err != nil {
		return nil, nil, err
	}
	return &s, u, nil
}

// RevokeSession removes one session by token (logout current device).
func (d *DB) RevokeSession(token string) error {
	if token == "" {
		return nil
	}
	_, err := d.Exec(`DELETE FROM sessions WHERE token_hash = ?`, hashSessionToken(token))
	return err
}

// RevokeAllUserSessions removes every session for a user (e.g. password change).
func (d *DB) RevokeAllUserSessions(userID int64) error {
	_, err := d.Exec(`DELETE FROM sessions WHERE user_id = ?`, userID)
	return err
}

// RevokeOtherUserSessions keeps only the session identified by token.
func (d *DB) RevokeOtherUserSessions(userID int64, keepToken string) error {
	if keepToken == "" {
		return d.RevokeAllUserSessions(userID)
	}
	_, err := d.Exec(
		`DELETE FROM sessions WHERE user_id = ? AND token_hash != ?`,
		userID, hashSessionToken(keepToken),
	)
	return err
}

func (d *DB) ListUserSessions(userID int64) ([]Session, error) {
	rows, err := d.Query(
		`SELECT id, user_id, created_at, expires_at, last_seen_at, user_agent, ip_address FROM sessions WHERE user_id = ? ORDER BY last_seen_at DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []Session
	for rows.Next() {
		var s Session
		var created, expires, lastSeen string
		if err := rows.Scan(&s.ID, &s.UserID, &created, &expires, &lastSeen, &s.UserAgent, &s.IPAddress); err != nil {
			return nil, err
		}
		s.CreatedAt = parseTime(created)
		s.ExpiresAt = parseTime(expires)
		s.LastSeenAt = parseTime(lastSeen)
		list = append(list, s)
	}
	if list == nil {
		list = []Session{}
	}
	return list, rows.Err()
}

func (d *DB) PruneExpiredSessions() error {
	_, err := d.Exec(`DELETE FROM sessions WHERE expires_at < datetime('now')`)
	return err
}

func (d *DB) CountActiveSessions(userID int64) (int, error) {
	var n int
	err := d.QueryRow(
		`SELECT COUNT(*) FROM sessions WHERE user_id = ? AND expires_at >= datetime('now')`,
		userID,
	).Scan(&n)
	return n, err
}
