package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"calisthenics/api/internal/httpx"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	cookieName = "cali_session"
	sessionTTL = 30 * 24 * time.Hour
	minPassLen = 10
)

type User struct {
	ID           string   `json:"id"`
	Email        string   `json:"email"`
	DisplayName  string   `json:"display_name"`
	BodyweightKg *float64 `json:"bodyweight_kg"`
}

type Service struct {
	pool   *pgxpool.Pool
	secure bool
}

func New(pool *pgxpool.Pool, secureCookies bool) *Service {
	return &Service{pool: pool, secure: secureCookies}
}

type ctxKey struct{}

// UserFrom returns the signed-in user attached by Required.
func UserFrom(ctx context.Context) (User, bool) {
	u, ok := ctx.Value(ctxKey{}).(User)
	return u, ok
}

// MustUser is for handlers mounted behind Required.
func MustUser(ctx context.Context) User {
	u, _ := UserFrom(ctx)
	return u
}

// ---------- session plumbing ----------

func newToken() (raw string, hash []byte, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", nil, err
	}
	raw = base64.RawURLEncoding.EncodeToString(b)
	sum := sha256.Sum256([]byte(raw))
	return raw, sum[:], nil
}

func hashToken(raw string) []byte {
	sum := sha256.Sum256([]byte(raw))
	return sum[:]
}

func (s *Service) issueSession(ctx context.Context, w http.ResponseWriter, userID string) error {
	raw, hash, err := newToken()
	if err != nil {
		return err
	}
	expires := time.Now().Add(sessionTTL)
	_, err = s.pool.Exec(ctx,
		`insert into sessions (token_hash, user_id, expires_at) values ($1, $2, $3)`,
		hash, userID, expires)
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    raw,
		Path:     "/",
		Expires:  expires,
		MaxAge:   int(sessionTTL.Seconds()),
		HttpOnly: true,
		Secure:   s.secure,
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}

func (s *Service) clearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   s.secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Service) userForToken(ctx context.Context, raw string) (User, error) {
	var u User
	err := s.pool.QueryRow(ctx, `
		select u.id, u.email, u.display_name, u.bodyweight_kg
		from sessions s
		join users u on u.id = s.user_id
		where s.token_hash = $1 and s.expires_at > now()`,
		hashToken(raw)).Scan(&u.ID, &u.Email, &u.DisplayName, &u.BodyweightKg)
	return u, err
}

// Required rejects anonymous requests and attaches the user to the context.
func (s *Service) Required(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(cookieName)
		if err != nil || c.Value == "" {
			httpx.Fail(w, http.StatusUnauthorized, "Sign in to continue.")
			return
		}
		u, err := s.userForToken(r.Context(), c.Value)
		if err != nil {
			s.clearCookie(w)
			httpx.Fail(w, http.StatusUnauthorized, "That session has expired. Sign in again.")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, u)))
	})
}

// RequiredFunc is the handler-function form of Required.
func (s *Service) RequiredFunc(h http.HandlerFunc) http.Handler {
	return s.Required(h)
}

// ---------- handlers ----------

type credentials struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
}

func (s *Service) Register(w http.ResponseWriter, r *http.Request) {
	var in credentials
	if !httpx.Decode(w, r, &in) {
		return
	}
	in.Email = strings.TrimSpace(in.Email)
	if _, err := mail.ParseAddress(in.Email); err != nil {
		httpx.Fail(w, http.StatusBadRequest, "Enter a valid email address.")
		return
	}
	if len(in.Password) < minPassLen {
		httpx.Fail(w, http.StatusBadRequest, "Use a password of at least 10 characters.")
		return
	}
	if strings.TrimSpace(in.DisplayName) == "" {
		in.DisplayName = strings.SplitN(in.Email, "@", 2)[0]
	}

	hash, err := HashPassword(in.Password)
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Couldn't create the account. Try again.")
		return
	}

	var u User
	err = s.pool.QueryRow(r.Context(), `
		insert into users (email, password_hash, display_name)
		values ($1, $2, $3)
		returning id, email, display_name, bodyweight_kg`,
		in.Email, hash, strings.TrimSpace(in.DisplayName),
	).Scan(&u.ID, &u.Email, &u.DisplayName, &u.BodyweightKg)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			httpx.Fail(w, http.StatusConflict, "An account already uses that email. Sign in instead.")
			return
		}
		httpx.Fail(w, http.StatusInternalServerError, "Couldn't create the account. Try again.")
		return
	}

	if err := s.issueSession(r.Context(), w, u.ID); err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Account created, but signing in failed. Try signing in.")
		return
	}
	httpx.JSON(w, http.StatusCreated, u)
}

func (s *Service) Login(w http.ResponseWriter, r *http.Request) {
	var in credentials
	if !httpx.Decode(w, r, &in) {
		return
	}

	var (
		u    User
		hash string
	)
	err := s.pool.QueryRow(r.Context(), `
		select id, email, display_name, bodyweight_kg, password_hash
		from users where lower(email) = lower($1)`,
		strings.TrimSpace(in.Email),
	).Scan(&u.ID, &u.Email, &u.DisplayName, &u.BodyweightKg, &hash)

	if errors.Is(err, pgx.ErrNoRows) {
		// Spend comparable time so a missing account isn't distinguishable by timing.
		_, _ = HashPassword(in.Password)
		httpx.Fail(w, http.StatusUnauthorized, "That email and password don't match.")
		return
	}
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Couldn't sign you in. Try again.")
		return
	}

	ok, err := VerifyPassword(in.Password, hash)
	if err != nil || !ok {
		httpx.Fail(w, http.StatusUnauthorized, "That email and password don't match.")
		return
	}
	if err := s.issueSession(r.Context(), w, u.ID); err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Couldn't start a session. Try again.")
		return
	}
	httpx.JSON(w, http.StatusOK, u)
}

func (s *Service) Logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(cookieName); err == nil && c.Value != "" {
		_, _ = s.pool.Exec(r.Context(), `delete from sessions where token_hash = $1`, hashToken(c.Value))
	}
	s.clearCookie(w)
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "signed out"})
}

func (s *Service) Me(w http.ResponseWriter, r *http.Request) {
	httpx.JSON(w, http.StatusOK, MustUser(r.Context()))
}

type profileUpdate struct {
	DisplayName  *string  `json:"display_name"`
	BodyweightKg *float64 `json:"bodyweight_kg"`
}

func (s *Service) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	var in profileUpdate
	if !httpx.Decode(w, r, &in) {
		return
	}
	me := MustUser(r.Context())

	var u User
	err := s.pool.QueryRow(r.Context(), `
		update users
		set display_name  = coalesce($2, display_name),
		    bodyweight_kg = coalesce($3, bodyweight_kg)
		where id = $1
		returning id, email, display_name, bodyweight_kg`,
		me.ID, in.DisplayName, in.BodyweightKg,
	).Scan(&u.ID, &u.Email, &u.DisplayName, &u.BodyweightKg)
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Couldn't save your profile. Try again.")
		return
	}
	httpx.JSON(w, http.StatusOK, u)
}

// PurgeExpiredSessions is run periodically from main.
func (s *Service) PurgeExpiredSessions(ctx context.Context) {
	if _, err := s.pool.Exec(ctx, `delete from sessions where expires_at < now()`); err != nil {
		// Non-fatal: expired sessions are already rejected on lookup.
		return
	}
}
