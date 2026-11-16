package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Accounts: sign up, log in, log out, in front of every page and every
// comment, so the server can listen on a public address. It follows the
// old essay-repl-server's auth.py: a username and a password on an
// ordinary form (so the browser offers to generate a strong password and
// saves it), salted password hashes in the data folder, a signed session
// cookie. No email, no reset.
//
// The notes are private, so sign-up is open only until the first account
// exists; after that the sign-up page says it is closed. Another account
// is a line added to users.json by hand.
//
// A request from the box itself (a loopback address) needs no session:
// the release's checks, check_note and the agent read the server that way.
// That trust is why the server must never sit behind a proxy on the same
// box, which would make every visitor look local.
//
// The accounts folder holds users.json (the accounts) and session_key (the
// cookie's signing key, made on first use; deleting it logs everyone out
// and nothing else). Both are 0600 in a 0700 folder.

const sessionCookie = "essay_session"

// sessionLife is how long a browser stays logged in.
const sessionLife = 365 * 24 * time.Hour

// loginWait is how long a failed log-in waits for its answer, so guessing
// is slow.
var loginWait = time.Second

// hashRounds is PBKDF2-SHA256's iteration count, OWASP's for it; a test
// lowers it.
var hashRounds = 600_000

// minPassword is the shortest password sign-up takes; a browser's
// generated one is longer.
const minPassword = 12

var usernameRE = regexp.MustCompile(`^[A-Za-z0-9_-]{1,32}$`)

type account struct {
	Name    string `json:"name"`
	Hash    string `json:"hash"`
	Created string `json:"created"`
}

type accounts struct {
	dir string
	key []byte

	mu    sync.Mutex
	users []account
}

// openAccounts reads the accounts folder, making it and its key when they
// are not there yet.
func openAccounts(dir string) (*accounts, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	a := &accounts{dir: dir}
	keyPath := filepath.Join(dir, "session_key")
	key, err := os.ReadFile(keyPath)
	if errors.Is(err, os.ErrNotExist) {
		key = make([]byte, 32)
		rand.Read(key)
		if err := os.WriteFile(keyPath, key, 0o600); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}
	if len(key) < 32 {
		return nil, fmt.Errorf("%s is %d bytes, not 32: delete it to make another (that logs everyone out)", keyPath, len(key))
	}
	a.key = key
	b, err := os.ReadFile(filepath.Join(dir, "users.json"))
	if err == nil {
		if err := json.Unmarshal(b, &a.users); err != nil {
			return nil, fmt.Errorf("users.json: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	return a, nil
}

func hashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	rand.Read(salt)
	h, err := pbkdf2.Key(sha256.New, password, salt, hashRounds, 32)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("pbkdf2-sha256:%d:%x:%x", hashRounds, salt, h), nil
}

func checkPassword(password, stored string) bool {
	parts := strings.Split(stored, ":")
	if len(parts) != 4 || parts[0] != "pbkdf2-sha256" {
		return false
	}
	rounds, err := strconv.Atoi(parts[1])
	salt, err2 := hex.DecodeString(parts[2])
	want, err3 := hex.DecodeString(parts[3])
	if err != nil || err2 != nil || err3 != nil {
		return false
	}
	got, err := pbkdf2.Key(sha256.New, password, salt, rounds, len(want))
	return err == nil && subtle.ConstantTimeCompare(got, want) == 1
}

func (a *accounts) count() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.users)
}

func (a *accounts) find(name string) (account, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, u := range a.users {
		if strings.EqualFold(u.Name, name) {
			return u, true
		}
	}
	return account{}, false
}

// addFirst makes the first account; once there is one, sign-up is closed.
func (a *accounts) addFirst(name, password string) error {
	h, err := hashPassword(password)
	if err != nil {
		return err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.users) > 0 {
		return errSignupClosed
	}
	users := append(a.users, account{Name: name, Hash: h, Created: time.Now().UTC().Format(time.RFC3339)})
	b, err := json.MarshalIndent(users, "", "  ")
	if err != nil {
		return err
	}
	tmp := filepath.Join(a.dir, "users.json.new")
	if err := os.WriteFile(tmp, append(b, '\n'), 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, filepath.Join(a.dir, "users.json")); err != nil {
		return err
	}
	a.users = users
	return nil
}

var errSignupClosed = errors.New("sign-up is closed: this server has its account")

// The session cookie is `name.expiry.mac`, the mac the key's HMAC of the
// rest.
func (a *accounts) sessionFor(name string, until time.Time) string {
	body := name + "." + strconv.FormatInt(until.Unix(), 10)
	mac := hmac.New(sha256.New, a.key)
	mac.Write([]byte(body))
	return body + "." + hex.EncodeToString(mac.Sum(nil))
}

// sessionUser is the account a request's cookie names, or "".
func (a *accounts) sessionUser(r *http.Request) string {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return ""
	}
	parts := strings.Split(c.Value, ".")
	if len(parts) != 3 {
		return ""
	}
	secs, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || time.Now().After(time.Unix(secs, 0)) {
		return ""
	}
	if !hmac.Equal([]byte(a.sessionFor(parts[0], time.Unix(secs, 0))), []byte(c.Value)) {
		return ""
	}
	if _, ok := a.find(parts[0]); !ok {
		return ""
	}
	return parts[0]
}

func (a *accounts) startSession(w http.ResponseWriter, name string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    a.sessionFor(name, time.Now().Add(sessionLife)),
		Path:     "/",
		MaxAge:   int(sessionLife / time.Second),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

type accountKey struct{}

// accountOf is the account a request is logged in as, or "" (logged out,
// over loopback, or a server without accounts).
func accountOf(r *http.Request) string {
	name, _ := r.Context().Value(accountKey{}).(string)
	return name
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

// wrap puts the accounts in front of h. A page asked for while logged out
// goes to the log-in page (the sign-up page while there is no account),
// which comes back to it; anything else (a comment's POST) is refused with
// 401.
func (a *accounts) wrap(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login":
			a.serveLogin(w, r)
			return
		case "/signup":
			a.serveSignup(w, r)
			return
		case "/logout":
			http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode})
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		user := a.sessionUser(r)
		if r.URL.Path == "/whoami" {
			// The page asks, to sign a reply with the account instead of a
			// typed name.
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Cache-Control", "no-store")
			json.NewEncoder(w).Encode(map[string]string{"name": user})
			return
		}
		if user != "" {
			h.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), accountKey{}, user)))
			return
		}
		if fromBox(r) {
			h.ServeHTTP(w, r)
			return
		}
		if r.Method == http.MethodGet || r.Method == http.MethodHead {
			http.Redirect(w, r, "/login?next="+url.QueryEscape(r.URL.RequestURI()), http.StatusSeeOther)
			return
		}
		http.Error(w, "log in first, at /login", http.StatusUnauthorized)
	})
}

// nextPage is where a log-in goes on to: a path on this server, never
// another site (`//host` or `/\host` would leave it).
func nextPage(next string) string {
	if !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") || strings.HasPrefix(next, "/\\") {
		return "/"
	}
	return next
}

func readForm(w http.ResponseWriter, r *http.Request) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "a log-in is a short form", http.StatusBadRequest)
		return false
	}
	return true
}

func (a *accounts) serveLogin(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet, http.MethodHead:
		if a.count() == 0 {
			http.Redirect(w, r, "/signup?next="+url.QueryEscape(r.URL.Query().Get("next")), http.StatusSeeOther)
			return
		}
		authPage(w, http.StatusOK, "Log in", "/login", r.URL.Query().Get("next"), "", "", "current-password")
	case http.MethodPost:
		if !readForm(w, r) {
			return
		}
		name := strings.TrimSpace(r.PostForm.Get("username"))
		next := r.PostForm.Get("next")
		u, ok := a.find(name)
		if !ok || !checkPassword(r.PostForm.Get("password"), u.Hash) {
			time.Sleep(loginWait)
			authPage(w, http.StatusUnauthorized, "Log in", "/login", next, name, "No account has that name and password.", "current-password")
			return
		}
		a.startSession(w, u.Name)
		http.Redirect(w, r, nextPage(next), http.StatusSeeOther)
	default:
		http.Error(w, "GET or POST", http.StatusMethodNotAllowed)
	}
}

func (a *accounts) serveSignup(w http.ResponseWriter, r *http.Request) {
	if a.count() > 0 {
		closedPage(w)
		return
	}
	switch r.Method {
	case http.MethodGet, http.MethodHead:
		authPage(w, http.StatusOK, "Sign up", "/signup", r.URL.Query().Get("next"), "", "", "new-password")
	case http.MethodPost:
		if !readForm(w, r) {
			return
		}
		name := strings.TrimSpace(r.PostForm.Get("username"))
		password := r.PostForm.Get("password")
		next := r.PostForm.Get("next")
		problem := ""
		if !usernameRE.MatchString(name) {
			problem = "A username is 1 to 32 letters, digits, - or _."
		} else if len(password) < minPassword {
			problem = fmt.Sprintf("A password is at least %d characters (let the browser make one).", minPassword)
		}
		if problem != "" {
			authPage(w, http.StatusBadRequest, "Sign up", "/signup", next, name, problem, "new-password")
			return
		}
		if err := a.addFirst(name, password); errors.Is(err, errSignupClosed) {
			closedPage(w)
			return
		} else if err != nil {
			http.Error(w, "the account could not be saved", http.StatusInternalServerError)
			return
		}
		a.startSession(w, name)
		http.Redirect(w, r, nextPage(next), http.StatusSeeOther)
	default:
		http.Error(w, "GET or POST", http.StatusMethodNotAllowed)
	}
}

// authPage is the log-in or sign-up form: a username and a password with
// the autocomplete words that tell the browser to fill a saved password
// (current-password) or offer to generate and save one (new-password).
func authPage(w http.ResponseWriter, code int, title, action, next, name, problem, kind string) {
	var b bytes.Buffer
	pageHead(&b, title)
	b.WriteString("<h1>" + title + "</h1>\n")
	if problem != "" {
		b.WriteString(`<p class="date">` + html.EscapeString(problem) + "</p>\n")
	}
	b.WriteString(`<form method="post" action="` + action + `">` + "\n")
	b.WriteString(`<input type="hidden" name="next" value="` + html.EscapeString(nextPage(next)) + `">` + "\n")
	b.WriteString(`<p><label>Username<br><input name="username" autocomplete="username" autocapitalize="none" required value="` + html.EscapeString(name) + `"` + autofocusIf(name == "") + `></label></p>` + "\n")
	minlength := ""
	if kind == "new-password" {
		minlength = fmt.Sprintf(` minlength="%d"`, minPassword)
	}
	b.WriteString(`<p><label>Password<br><input type="password" name="password" autocomplete="` + kind + `"` + minlength + ` required` + autofocusIf(name != "") + `></label></p>` + "\n")
	b.WriteString(`<p><button type="submit">` + title + `</button></p>` + "\n</form>\n</main></body></html>\n")
	writeAuth(w, code, b.Bytes())
}

func autofocusIf(yes bool) string {
	if yes {
		return " autofocus"
	}
	return ""
}

func closedPage(w http.ResponseWriter) {
	var b bytes.Buffer
	pageHead(&b, "Sign up")
	b.WriteString("<h1>Sign up</h1>\n<p>Sign-up is closed: this server has its account. <a href=\"/login\">Log in</a>.</p>\n</main></body></html>\n")
	writeAuth(w, http.StatusForbidden, b.Bytes())
}

func writeAuth(w http.ResponseWriter, code int, page []byte) {
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	w.Write(page)
}

// checkOpen refuses an address past loopback without accounts: a typo in
// a flag must never serve the notes to everyone.
func checkOpen(addr, accountsDir string) error {
	if accountsDir != "" {
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
	return errors.New("listening on " + addr + " needs -accounts: only a loopback address may go without logging in")
}
