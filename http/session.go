package http

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Session represents per-client state stored in an encrypted cookie.
//
// Phase 1 implements a secure cookie-backed session (no server storage).
type Session struct {
	Values   map[string]any `json:"values"`
	Flashes  map[string]any `json:"flashes,omitempty"`
	CSRF     string         `json:"csrf"`
	IssuedAt int64          `json:"iat"`

	dirty bool `json:"-"`
}

func newSession() *Session {
	return &Session{
		Values:   make(map[string]any),
		Flashes:  make(map[string]any),
		IssuedAt: time.Now().Unix(),
		dirty:    true,
	}
}

// Get returns a value from the session.
func (s *Session) Get(key string) any {
	if s == nil {
		return nil
	}
	return s.Values[key]
}

// Put sets a value in the session.
func (s *Session) Put(key string, value any) {
	if s == nil {
		return
	}
	if s.Values == nil {
		s.Values = make(map[string]any)
	}
	s.Values[key] = value
	s.dirty = true
}

// Flash sets a value that is meant to be used once.
func (s *Session) Flash(key string, value any) {
	if s == nil {
		return
	}
	if s.Flashes == nil {
		s.Flashes = make(map[string]any)
	}
	s.Flashes[key] = value
	s.dirty = true
}

// PullFlash reads and removes a flash value.
func (s *Session) PullFlash(key string) any {
	if s == nil {
		return nil
	}
	v := s.Flashes[key]
	delete(s.Flashes, key)
	s.dirty = true
	return v
}

// SessionManager controls cookie session behavior.
type SessionManager struct {
	CookieName string
	Key        []byte
	Path       string
	Domain     string
	Secure     bool
	HTTPOnly   bool
	SameSite   http.SameSite
	MaxAge     time.Duration
}

func NewSessionManager(appKey string) (*SessionManager, error) {
	key, err := deriveKey(appKey)
	if err != nil {
		return nil, err
	}
	return &SessionManager{
		CookieName: "jimo_session",
		Key:        key,
		Path:       "/",
		Secure:     false,
		HTTPOnly:   true,
		SameSite:   http.SameSiteLaxMode,
		MaxAge:     14 * 24 * time.Hour,
	}, nil
}

func deriveKey(appKey string) ([]byte, error) {
	appKey = strings.TrimSpace(appKey)
	if appKey == "" {
		return nil, fmt.Errorf("session: APP_KEY is empty")
	}

	if strings.HasPrefix(appKey, "base64:") {
		raw := strings.TrimPrefix(appKey, "base64:")
		b, err := base64.StdEncoding.DecodeString(raw)
		if err != nil {
			return nil, fmt.Errorf("session: invalid base64 APP_KEY")
		}
		sum := sha256.Sum256(b)
		return sum[:], nil
	}

	sum := sha256.Sum256([]byte(appKey))
	return sum[:], nil
}

func (m *SessionManager) load(r *http.Request) *Session {
	c, err := r.Cookie(m.CookieName)
	if err != nil {
		s := newSession()
		ensureCSRF(s)
		return s
	}

	s, err := m.decrypt(c.Value)
	if err != nil {
		s = newSession()
		ensureCSRF(s)
		return s
	}

	if s.Values == nil {
		s.Values = make(map[string]any)
	}
	if s.Flashes == nil {
		s.Flashes = make(map[string]any)
	}
	ensureCSRF(s)
	return s
}

func (m *SessionManager) save(w http.ResponseWriter, s *Session) error {
	if s == nil {
		return nil
	}
	if !s.dirty {
		return nil
	}

	enc, err := m.encrypt(s)
	if err != nil {
		return err
	}

	cookie := &http.Cookie{
		Name:     m.CookieName,
		Value:    enc,
		Path:     m.Path,
		Domain:   m.Domain,
		Secure:   m.Secure,
		HttpOnly: m.HTTPOnly,
		SameSite: m.SameSite,
		Expires:  time.Now().Add(m.MaxAge),
	}
	http.SetCookie(w, cookie)
	return nil
}

func (m *SessionManager) encrypt(s *Session) (string, error) {
	payload, err := json.Marshal(s)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(m.Key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nil, nonce, payload, nil)
	out := append(nonce, ciphertext...)
	return "v1." + base64.RawURLEncoding.EncodeToString(out), nil
}

func (m *SessionManager) decrypt(value string) (*Session, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, fmt.Errorf("session: empty")
	}
	if !strings.HasPrefix(value, "v1.") {
		return nil, fmt.Errorf("session: unsupported version")
	}

	blob, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(value, "v1."))
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(m.Key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(blob) < gcm.NonceSize() {
		return nil, fmt.Errorf("session: invalid payload")
	}

	nonce := blob[:gcm.NonceSize()]
	ciphertext := blob[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	var s Session
	if err := json.Unmarshal(plain, &s); err != nil {
		return nil, err
	}
	s.dirty = false
	return &s, nil
}

func ensureCSRF(s *Session) {
	if s.CSRF != "" {
		return
	}
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	s.CSRF = base64.RawURLEncoding.EncodeToString(b)
	s.dirty = true
}
