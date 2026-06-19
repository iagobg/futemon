package app

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	sessionCookieName    = "futemon_session"
	oauthStateCookieName = "futemon_oauth_state"
)

type GoogleOAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

type GoogleProfile struct {
	GoogleID    string
	DisplayName string
	Email       string
	PictureURL  string
}

func loadGoogleOAuthConfig() GoogleOAuthConfig {
	return GoogleOAuthConfig{
		ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		RedirectURL:  os.Getenv("GOOGLE_REDIRECT_URL"),
	}
}

func (c GoogleOAuthConfig) configured(r *http.Request) bool {
	return c.ClientID != "" && c.ClientSecret != "" && c.redirectURL(r) != ""
}

func (c GoogleOAuthConfig) redirectURL(r *http.Request) string {
	if c.RedirectURL != "" {
		return c.RedirectURL
	}
	if r == nil {
		return ""
	}
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	return scheme + "://" + r.Host + "/auth/google/callback"
}

func (s *Server) sessionSecret() []byte {
	if len(s.sessionKey) > 0 {
		return s.sessionKey
	}
	return []byte("futemon-dev-session-secret-change-me")
}

func randomToken(bytes int) string {
	buf := make([]byte, bytes)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}

func (s *Server) signSession(userID string, expires time.Time) string {
	payload := userID + "|" + strconv.FormatInt(expires.Unix(), 10)
	mac := hmac.New(sha256.New, s.sessionSecret())
	mac.Write([]byte(payload))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." + sig
}

func (s *Server) verifySession(value string) (string, bool) {
	parts := strings.Split(value, ".")
	if len(parts) != 2 {
		return "", false
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", false
	}
	payload := string(payloadBytes)
	mac := hmac.New(sha256.New, s.sessionSecret())
	mac.Write([]byte(payload))
	expected := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(parts[1])) {
		return "", false
	}
	payloadParts := strings.Split(payload, "|")
	if len(payloadParts) != 2 {
		return "", false
	}
	expiresUnix, err := strconv.ParseInt(payloadParts[1], 10, 64)
	if err != nil || time.Now().After(time.Unix(expiresUnix, 0)) {
		return "", false
	}
	return payloadParts[0], true
}

func (s *Server) setSession(w http.ResponseWriter, userID string) {
	expires := time.Now().Add(14 * 24 * time.Hour)
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    s.signSession(userID, expires),
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func clearCookie(w http.ResponseWriter, name string) {
	http.SetCookie(w, &http.Cookie{Name: name, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode})
}

func (s *Server) currentUser(r *http.Request) (User, bool) {
	if s.authMode == "local" {
		return s.store.CurrentUser()
	}
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		if userID, ok := s.verifySession(cookie.Value); ok {
			if user, ok := s.store.UserByID(userID); ok {
				return user, true
			}
		}
	}
	return User{}, false
}

func (s *Server) handleGoogleLogin(w http.ResponseWriter, r *http.Request) {
	if s.authMode == "local" {
		http.Redirect(w, r, "/teams", http.StatusSeeOther)
		return
	}
	if !s.googleOAuth.configured(r) {
		http.Error(w, "Google OAuth nao configurado", http.StatusServiceUnavailable)
		return
	}
	state := randomToken(16)
	http.SetCookie(w, &http.Cookie{Name: oauthStateCookieName, Value: state, Path: "/", MaxAge: 300, HttpOnly: true, SameSite: http.SameSiteLaxMode})
	query := url.Values{}
	query.Set("client_id", s.googleOAuth.ClientID)
	query.Set("redirect_uri", s.googleOAuth.redirectURL(r))
	query.Set("response_type", "code")
	query.Set("scope", "openid email profile")
	query.Set("state", state)
	http.Redirect(w, r, "https://accounts.google.com/o/oauth2/v2/auth?"+query.Encode(), http.StatusFound)
}

func (s *Server) handleGoogleCallback(w http.ResponseWriter, r *http.Request) {
	if s.authMode == "local" {
		http.Redirect(w, r, "/teams", http.StatusSeeOther)
		return
	}
	stateCookie, err := r.Cookie(oauthStateCookieName)
	if err != nil || stateCookie.Value == "" || stateCookie.Value != r.URL.Query().Get("state") {
		http.Error(w, "estado OAuth invalido", http.StatusBadRequest)
		return
	}
	clearCookie(w, oauthStateCookieName)
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "codigo OAuth ausente", http.StatusBadRequest)
		return
	}
	profile, err := s.fetchGoogleProfile(r, code)
	if err != nil {
		http.Error(w, "falha ao autenticar com Google", http.StatusBadGateway)
		return
	}
	user, err := s.store.UpsertGoogleUser(profile)
	if err != nil {
		http.Error(w, "falha ao salvar usuario", http.StatusInternalServerError)
		return
	}
	s.setSession(w, user.ID)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if s.authMode == "local" {
		http.Redirect(w, r, "/teams", http.StatusSeeOther)
		return
	}
	clearCookie(w, sessionCookieName)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) fetchGoogleProfile(r *http.Request, code string) (GoogleProfile, error) {
	form := url.Values{}
	form.Set("code", code)
	form.Set("client_id", s.googleOAuth.ClientID)
	form.Set("client_secret", s.googleOAuth.ClientSecret)
	form.Set("redirect_uri", s.googleOAuth.redirectURL(r))
	form.Set("grant_type", "authorization_code")
	resp, err := http.PostForm("https://oauth2.googleapis.com/token", form)
	if err != nil {
		return GoogleProfile{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return GoogleProfile{}, errors.New("token endpoint rejected request")
	}
	var token struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return GoogleProfile{}, err
	}
	if token.AccessToken == "" {
		return GoogleProfile{}, errors.New("empty access token")
	}
	req, _ := http.NewRequestWithContext(r.Context(), http.MethodGet, "https://www.googleapis.com/oauth2/v2/userinfo", nil)
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	userinfo, err := http.DefaultClient.Do(req)
	if err != nil {
		return GoogleProfile{}, err
	}
	defer userinfo.Body.Close()
	if userinfo.StatusCode < 200 || userinfo.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, userinfo.Body)
		return GoogleProfile{}, errors.New("userinfo endpoint rejected request")
	}
	var payload struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		Email   string `json:"email"`
		Picture string `json:"picture"`
	}
	if err := json.NewDecoder(userinfo.Body).Decode(&payload); err != nil {
		return GoogleProfile{}, err
	}
	if payload.ID == "" || payload.Email == "" {
		return GoogleProfile{}, errors.New("incomplete google profile")
	}
	if payload.Name == "" {
		payload.Name = payload.Email
	}
	return GoogleProfile{GoogleID: payload.ID, DisplayName: payload.Name, Email: payload.Email, PictureURL: payload.Picture}, nil
}
