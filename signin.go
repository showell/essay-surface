package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"html"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// The sign-in: one password, kept in a file outside any repo, in front of
// every page and every comment, so the server can listen on a public
// address. It is light on purpose (the notes are private, not secret):
// plain HTTP, no accounts, no sessions to store.
//
// The cookie is the password's HMAC of a fixed message, so it survives a
// restart (a release swaps servers) and a new password signs every browser
// out. A request from the box itself (a loopback address) needs no cookie:
// the release's checks, check_note and the agent read the server that way.
// That trust is why the server must never sit behind a proxy on the same
// box, which would make every visitor look local.

const signinCookie = "essay_signin"

// signinYear is how long a browser stays signed in.
const signinYear = 365 * 24 * time.Hour

// signinWait is how long a wrong password waits for its answer, so guessing
// is slow.
var signinWait = time.Second

type signin struct {
	cookie string // the value a signed-in browser holds
	pass   []byte
}

// newSignin reads the password from path; a missing or empty file is an
// error, never an open server.
func newSignin(path string) (*signin, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	pass := bytes.TrimSpace(b)
	if len(pass) == 0 {
		return nil, fmt.Errorf("%s holds no password", path)
	}
	mac := hmac.New(sha256.New, pass)
	mac.Write([]byte("essay-surface: signed in"))
	return &signin{cookie: hex.EncodeToString(mac.Sum(nil)), pass: pass}, nil
}

// fromBox says whether a request came over loopback.
func fromBox(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func (g *signin) signedIn(r *http.Request) bool {
	c, err := r.Cookie(signinCookie)
	return err == nil && subtle.ConstantTimeCompare([]byte(c.Value), []byte(g.cookie)) == 1
}

// wrap puts the sign-in in front of h. A page asked for while signed out
// goes to the sign-in page, which comes back to it; anything else (a
// comment's POST) is refused with 401.
func (g *signin) wrap(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/signin" {
			g.serveSignin(w, r)
			return
		}
		if fromBox(r) || g.signedIn(r) {
			h.ServeHTTP(w, r)
			return
		}
		if r.Method == http.MethodGet || r.Method == http.MethodHead {
			http.Redirect(w, r, "/signin?next="+url.QueryEscape(r.URL.RequestURI()), http.StatusSeeOther)
			return
		}
		http.Error(w, "sign in first, at /signin", http.StatusUnauthorized)
	})
}

// nextPage is where a sign-in goes on to: a path on this server, never
// another site (`//host` or `/\host` would leave it).
func nextPage(next string) string {
	if !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") || strings.HasPrefix(next, "/\\") {
		return "/"
	}
	return next
}

func (g *signin) serveSignin(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet, http.MethodHead:
		signinPage(w, http.StatusOK, r.URL.Query().Get("next"), "")
	case http.MethodPost:
		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		if err := r.ParseForm(); err != nil {
			http.Error(w, "a sign-in is a short form", http.StatusBadRequest)
			return
		}
		next := r.PostForm.Get("next")
		if subtle.ConstantTimeCompare([]byte(r.PostForm.Get("password")), g.pass) != 1 {
			time.Sleep(signinWait)
			signinPage(w, http.StatusUnauthorized, next, "That password is wrong.")
			return
		}
		http.SetCookie(w, &http.Cookie{
			Name:     signinCookie,
			Value:    g.cookie,
			Path:     "/",
			MaxAge:   int(signinYear / time.Second),
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})
		http.Redirect(w, r, nextPage(next), http.StatusSeeOther)
	default:
		http.Error(w, "GET or POST", http.StatusMethodNotAllowed)
	}
}

func signinPage(w http.ResponseWriter, code int, next, problem string) {
	var b bytes.Buffer
	pageHead(&b, "Sign in")
	b.WriteString("<h1>Notes</h1>\n")
	if problem != "" {
		b.WriteString(`<p class="date">` + html.EscapeString(problem) + "</p>\n")
	}
	b.WriteString(`<form method="post" action="/signin">` + "\n")
	b.WriteString(`<input type="hidden" name="next" value="` + html.EscapeString(nextPage(next)) + `">` + "\n")
	b.WriteString(`<p><label>Password <input type="password" name="password" autocomplete="current-password" autofocus required></label></p>` + "\n")
	b.WriteString(`<p><button type="submit">Sign in</button></p>` + "\n</form>\n</main></body></html>\n")
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	w.Write(b.Bytes())
}

// checkOpen refuses an address past loopback without a password: a typo in
// a flag must never serve the notes to everyone.
func checkOpen(addr, passwordFile string) error {
	if passwordFile != "" {
		return nil
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return err
	}
	ip := net.ParseIP(host)
	if host == "localhost" || (ip != nil && ip.IsLoopback()) {
		return nil
	}
	return errors.New("listening on " + addr + " needs -password-file: only a loopback address may go without the sign-in")
}
