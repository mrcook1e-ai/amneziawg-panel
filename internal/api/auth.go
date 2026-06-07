package api

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"sync"
	"time"
)

const sessionCookie = "awgp_sid"

type session struct {
	expires time.Time
}

type Auth struct {
	password string
	mu       sync.Mutex
	sess     map[string]session
}

func NewAuth(password string) *Auth {
	return &Auth{password: password, sess: map[string]session{}}
}

func (a *Auth) Required() bool { return a.password != "" }

func (a *Auth) Login(password string) (string, bool) {
	if a.password == "" || subtle.ConstantTimeCompare([]byte(password), []byte(a.password)) != 1 {
		return "", false
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", false
	}
	tok := hex.EncodeToString(b)
	a.mu.Lock()
	a.sess[tok] = session{expires: time.Now().Add(7 * 24 * time.Hour)}
	a.mu.Unlock()
	return tok, true
}

func (a *Auth) Logout(tok string) {
	a.mu.Lock()
	delete(a.sess, tok)
	a.mu.Unlock()
}

func (a *Auth) Valid(tok string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	s, ok := a.sess[tok]
	if !ok {
		return false
	}
	if time.Now().After(s.expires) {
		delete(a.sess, tok)
		return false
	}
	return true
}

func (a *Auth) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !a.Required() {
			next.ServeHTTP(w, r)
			return
		}
		c, err := r.Cookie(sessionCookie)
		if err != nil || !a.Valid(c.Value) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Not Logged In"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   7 * 24 * 3600,
	})
}

func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: "", Path: "/", MaxAge: -1,
	})
}
