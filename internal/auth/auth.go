package auth

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"

	"mikrotik-monitor/internal/models"
)

type contextKey string

const UserContextKey contextKey = "user"
const SessionContextKey contextKey = "session"

type Claims struct {
	UserID    int64  `json:"uid"`
	Username  string `json:"sub"`
	Role      string `json:"role"`
	SessionID int64  `json:"sid"`
}

type Manager struct {
	db         *models.DB
	sessionTTL time.Duration
}

func NewManager(db *models.DB) *Manager {
	return &Manager{db: db, sessionTTL: 24 * time.Hour}
}

func (m *Manager) CreateSession(r *http.Request, user *models.User) (token string, err error) {
	ua := r.UserAgent()
	ip := r.RemoteAddr
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		ip = host
	}
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		ip = strings.TrimSpace(strings.Split(fwd, ",")[0])
	}
	return m.db.CreateSession(user.ID, ua, ip, m.sessionTTL)
}

func (m *Manager) ValidateToken(token string) (*Claims, error) {
	sess, u, err := m.db.ValidateSession(token)
	if err != nil {
		return nil, err
	}
	return &Claims{
		UserID:    u.ID,
		Username:  u.Username,
		Role:      u.Role,
		SessionID: sess.ID,
	}, nil
}

func (m *Manager) RevokeToken(token string) error {
	return m.db.RevokeSession(token)
}

func (m *Manager) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := ExtractToken(r)
		if token == "" {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		claims, err := m.ValidateToken(token)
		if err != nil {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), UserContextKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := ClaimsFromContext(r.Context())
		if claims == nil || claims.Role != "admin" {
			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func ClaimsFromContext(ctx context.Context) *Claims {
	c, _ := ctx.Value(UserContextKey).(*Claims)
	return c
}

func ExtractToken(r *http.Request) string {
	if c, err := r.Cookie("token"); err == nil && c.Value != "" {
		return c.Value
	}
	h := r.Header.Get("Authorization")
	if strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}
	return ""
}

func SetTokenCookie(w http.ResponseWriter, token string, maxAge int) {
	if maxAge <= 0 {
		maxAge = 86400
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   maxAge,
		SameSite: http.SameSiteLaxMode,
	})
}

func ClearTokenCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
		SameSite: http.SameSiteLaxMode,
	})
}

// WSAuth wraps a WebSocket handler with session validation.
func (m *Manager) WSAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := ExtractToken(r)
		if token == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if _, err := m.ValidateToken(token); err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

var ErrUnauthorized = errors.New("unauthorized")
