package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// testAccounts is a fresh accounts folder in front of a handler that
// answers "the notes", with hashing and failed log-ins made quick.
func testAccounts(t *testing.T) (*accounts, http.Handler, string) {
	t.Helper()
	loginWait = 0
	hashRounds = 1000
	dir := filepath.Join(t.TempDir(), "accounts")
	a, err := openAccounts(dir)
	if err != nil {
		t.Fatal(err)
	}
	notes := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("the notes")) })
	return a, a.wrap(notes), dir
}

// ask sends a request from far (a public address) unless remote is given.
func ask(h http.Handler, method, path, remote string, form url.Values, cookie *http.Cookie) *httptest.ResponseRecorder {
	var req *http.Request
	if form != nil {
		req = httptest.NewRequest(method, path, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	if remote == "" {
		remote = "203.0.113.7:51000"
	}
	req.RemoteAddr = remote
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func sessionOf(t *testing.T, rec *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookie && c.MaxAge > 0 {
			return c
		}
	}
	t.Fatalf("no session cookie: %d %s", rec.Code, rec.Body.String())
	return nil
}

func signUp(t *testing.T, h http.Handler) *http.Cookie {
	t.Helper()
	rec := ask(h, "POST", "/signup", "", url.Values{"username": {"steve"}, "password": {"a-generated-password-1"}, "next": {"/notes/a.note"}}, nil)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/notes/a.note" {
		t.Fatalf("sign-up: %d to %q: %s", rec.Code, rec.Header().Get("Location"), rec.Body.String())
	}
	return sessionOf(t, rec)
}

func TestAccountsLoggedOutPageGoesToLogin(t *testing.T) {
	_, h, _ := testAccounts(t)
	rec := ask(h, "GET", "/notes/a.note?x=1", "", nil, nil)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/login?next=%2Fnotes%2Fa.note%3Fx%3D1" {
		t.Fatalf("got %d to %q", rec.Code, rec.Header().Get("Location"))
	}
	if strings.Contains(rec.Body.String(), "the notes") {
		t.Fatal("a logged-out page showed the notes")
	}
}

func TestAccountsLoggedOutCommentRefused(t *testing.T) {
	_, h, _ := testAccounts(t)
	rec := ask(h, "POST", "/notes/a.note/comments", "", url.Values{"text": {"hi"}}, nil)
	if rec.Code != http.StatusUnauthorized || strings.Contains(rec.Body.String(), "the notes") {
		t.Fatalf("got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAccountsLoginGoesToSignupUntilTheFirstAccount(t *testing.T) {
	_, h, _ := testAccounts(t)
	if rec := ask(h, "GET", "/login?next=/notes/a.note", "", nil, nil); rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/signup?next=%2Fnotes%2Fa.note" {
		t.Fatalf("no account yet: %d to %q", rec.Code, rec.Header().Get("Location"))
	}
	signUp(t, h)
	if rec := ask(h, "GET", "/login", "", nil, nil); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `autocomplete="current-password"`) {
		t.Fatalf("an account: %d", rec.Code)
	}
}

func TestAccountsSignupFormAsksTheBrowserForANewPassword(t *testing.T) {
	_, h, _ := testAccounts(t)
	rec := ask(h, "GET", "/signup", "", nil, nil)
	body := rec.Body.String()
	for _, want := range []string{`autocomplete="username"`, `autocomplete="new-password"`, `minlength="12"`, `method="post"`} {
		if !strings.Contains(body, want) {
			t.Errorf("the sign-up form lacks %s", want)
		}
	}
}

func TestAccountsSignupThenTheCookieServes(t *testing.T) {
	_, h, dir := testAccounts(t)
	c := signUp(t, h)
	if !c.HttpOnly || c.SameSite != http.SameSiteLaxMode {
		t.Fatalf("cookie: %+v", c)
	}
	if got := ask(h, "GET", "/", "", nil, c); got.Code != http.StatusOK || got.Body.String() != "the notes" {
		t.Fatalf("with the cookie: %d %q", got.Code, got.Body.String())
	}
	if got := ask(h, "POST", "/notes/a.note/comments", "", url.Values{"text": {"hi"}}, c); got.Code != http.StatusOK {
		t.Fatalf("a comment with the cookie: %d", got.Code)
	}
	b, err := os.ReadFile(filepath.Join(dir, "users.json"))
	if err != nil || !strings.Contains(string(b), `"steve"`) || strings.Contains(string(b), "a-generated-password-1") {
		t.Fatalf("users.json: %v %s", err, b)
	}
	for _, f := range []string{"users.json", "session_key"} {
		st, err := os.Stat(filepath.Join(dir, f))
		if err != nil || st.Mode().Perm() != 0o600 {
			t.Errorf("%s: %v %v", f, err, st)
		}
	}
}

func TestAccountsSignupClosedAfterTheFirst(t *testing.T) {
	_, h, _ := testAccounts(t)
	signUp(t, h)
	rec := ask(h, "POST", "/signup", "", url.Values{"username": {"other"}, "password": {"another-password-2"}}, nil)
	if rec.Code != http.StatusForbidden || len(rec.Result().Cookies()) != 0 {
		t.Fatalf("a second sign-up: %d", rec.Code)
	}
	if got := ask(h, "GET", "/signup", "", nil, nil); got.Code != http.StatusForbidden {
		t.Fatalf("the sign-up page after: %d", got.Code)
	}
}

func TestAccountsSignupRefusesBadNamesAndShortPasswords(t *testing.T) {
	_, h, _ := testAccounts(t)
	for _, f := range []url.Values{
		{"username": {"steve howell"}, "password": {"a-generated-password-1"}},
		{"username": {""}, "password": {"a-generated-password-1"}},
		{"username": {"steve"}, "password": {"short"}},
	} {
		if rec := ask(h, "POST", "/signup", "", f, nil); rec.Code != http.StatusBadRequest || len(rec.Result().Cookies()) != 0 {
			t.Errorf("%v: %d", f, rec.Code)
		}
	}
	signUp(t, h) // still open: none of those made an account
}

func TestAccountsLogin(t *testing.T) {
	_, h, _ := testAccounts(t)
	signUp(t, h)
	bad := ask(h, "POST", "/login", "", url.Values{"username": {"steve"}, "password": {"a-generated-password-2"}}, nil)
	if bad.Code != http.StatusUnauthorized || len(bad.Result().Cookies()) != 0 {
		t.Fatalf("a wrong password: %d", bad.Code)
	}
	nobody := ask(h, "POST", "/login", "", url.Values{"username": {"nobody"}, "password": {"a-generated-password-1"}}, nil)
	if nobody.Code != http.StatusUnauthorized {
		t.Fatalf("no such account: %d", nobody.Code)
	}
	good := ask(h, "POST", "/login", "", url.Values{"username": {"Steve"}, "password": {"a-generated-password-1"}, "next": {"//evil.example/"}}, nil)
	if good.Code != http.StatusSeeOther || good.Header().Get("Location") != "/" {
		t.Fatalf("the right password: %d to %q", good.Code, good.Header().Get("Location"))
	}
	if got := ask(h, "GET", "/", "", nil, sessionOf(t, good)); got.Code != http.StatusOK {
		t.Fatalf("after logging in: %d", got.Code)
	}
}

func TestAccountsForgedExpiredAndLoggedOutCookies(t *testing.T) {
	a, h, _ := testAccounts(t)
	c := signUp(t, h)
	forged := &http.Cookie{Name: sessionCookie, Value: c.Value[:len(c.Value)-1] + "0"}
	if strings.HasSuffix(c.Value, "0") {
		forged.Value = c.Value[:len(c.Value)-1] + "1"
	}
	expired := &http.Cookie{Name: sessionCookie, Value: a.sessionFor("steve", time.Now().Add(-time.Minute))}
	stranger := &http.Cookie{Name: sessionCookie, Value: a.sessionFor("nobody", time.Now().Add(time.Hour))}
	for name, ck := range map[string]*http.Cookie{"forged": forged, "expired": expired, "no such account": stranger} {
		if got := ask(h, "GET", "/", "", nil, ck); got.Code != http.StatusSeeOther {
			t.Errorf("%s: %d", name, got.Code)
		}
	}
	out := ask(h, "GET", "/logout", "", nil, c)
	cleared := false
	for _, k := range out.Result().Cookies() {
		if k.Name == sessionCookie && k.MaxAge < 0 {
			cleared = true
		}
	}
	if out.Code != http.StatusSeeOther || !cleared {
		t.Fatalf("log-out: %d, cleared %v", out.Code, cleared)
	}
}

func TestAccountsSurviveARestart(t *testing.T) {
	_, h, dir := testAccounts(t)
	c := signUp(t, h)
	again, err := openAccounts(dir)
	if err != nil {
		t.Fatal(err)
	}
	h2 := again.wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("the notes")) }))
	if got := ask(h2, "GET", "/", "", nil, c); got.Code != http.StatusOK {
		t.Fatalf("the old cookie after a restart: %d", got.Code)
	}
	if got := ask(h2, "GET", "/signup", "", nil, nil); got.Code != http.StatusForbidden {
		t.Fatalf("sign-up after a restart: %d", got.Code)
	}
}

func TestAccountsLoopbackNeedsNone(t *testing.T) {
	_, h, _ := testAccounts(t)
	for _, remote := range []string{"127.0.0.1:40000", "[::1]:40000"} {
		if got := ask(h, "GET", "/", remote, nil, nil); got.Code != http.StatusOK || got.Body.String() != "the notes" {
			t.Fatalf("from %s: %d", remote, got.Code)
		}
	}
}

func TestNextPageStaysOnThisServer(t *testing.T) {
	for next, want := range map[string]string{
		"/notes/a.note":   "/notes/a.note",
		"":                "/",
		"//evil.example/": "/",
		"/\\evil.example": "/",
		"https://evil/":   "/",
	} {
		if got := nextPage(next); got != want {
			t.Errorf("nextPage(%q) = %q, want %q", next, got, want)
		}
	}
}

func TestCheckOpen(t *testing.T) {
	for _, c := range []struct {
		addr, dir string
		ok        bool
	}{
		{"127.0.0.1:9301", "", true},
		{"localhost:9301", "", true},
		{"[::1]:9301", "", true},
		{"0.0.0.0:9301", "", false},
		{":9301", "", false},
		{"203.0.113.9:9301", "", false},
		{"0.0.0.0:9301", "/data/accounts", true},
	} {
		if err := checkOpen(c.addr, c.dir); (err == nil) != c.ok {
			t.Errorf("checkOpen(%q, %q): %v", c.addr, c.dir, err)
		}
	}
}

// A logged-in reply is signed by its account, whatever name the page sends;
// over loopback, without an account, the typed name stands.
func TestAccountsReplySignedByTheAccount(t *testing.T) {
	loginWait = 0
	hashRounds = 1000
	dir := t.TempDir()
	writeNote(t, dir, "a.md", commentNote, time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC))
	a, err := openAccounts(filepath.Join(dir, "accounts"))
	if err != nil {
		t.Fatal(err)
	}
	h := a.wrap(testStore(t, dir).handler())
	c := signUp(t, h)
	page := ask(h, "GET", "/notes/a.md", "", nil, c)
	hs := hashesOf(t, page.Body.String())
	send := func(remote string, cookie *http.Cookie, name string) map[string]any {
		req := httptest.NewRequest("POST", "/notes/a.md/comments", strings.NewReader(commentJSON(2, hs[2], name, "a reply")))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = remote
		if cookie != nil {
			req.AddCookie(cookie)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("post from %s: %d %s", remote, rec.Code, rec.Body.String())
		}
		var got map[string]any
		json.Unmarshal(rec.Body.Bytes(), &got)
		return got
	}
	if got := send("203.0.113.7:51000", c, "Mallory"); got["name"] != "steve" {
		t.Errorf("a logged-in reply is signed %v", got["name"])
	}
	if got := send("127.0.0.1:40000", nil, "Agent"); got["name"] != "Agent" {
		t.Errorf("a loopback reply is signed %v", got["name"])
	}
}

func TestAccountsWhoami(t *testing.T) {
	_, h, _ := testAccounts(t)
	c := signUp(t, h)
	for _, k := range []struct {
		remote string
		cookie *http.Cookie
		want   string
	}{
		{"", c, `{"name":"steve"}`},
		{"", nil, `{"name":""}`},
		{"127.0.0.1:40000", nil, `{"name":""}`},
	} {
		rec := ask(h, "GET", "/whoami", k.remote, nil, k.cookie)
		if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != k.want {
			t.Errorf("whoami from %q: %d %s", k.remote, rec.Code, rec.Body.String())
		}
	}
}
