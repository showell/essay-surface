package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// testGate is a sign-in with the password "open sesame" in front of a
// handler that answers "the notes".
func testGate(t *testing.T) (*signin, http.Handler) {
	t.Helper()
	signinWait = 0
	p := filepath.Join(t.TempDir(), "password")
	if err := os.WriteFile(p, []byte("open sesame\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	g, err := newSignin(p)
	if err != nil {
		t.Fatal(err)
	}
	notes := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("the notes")) })
	return g, g.wrap(notes)
}

// ask sends a request from far (a public address) unless remote is given.
func ask(h http.Handler, method, path, remote string, body url.Values, cookie *http.Cookie) *httptest.ResponseRecorder {
	var req *http.Request
	if body != nil {
		req = httptest.NewRequest(method, path, strings.NewReader(body.Encode()))
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

func TestSigninSignedOutPageGoesToSignin(t *testing.T) {
	_, h := testGate(t)
	rec := ask(h, "GET", "/notes/a.note?x=1", "", nil, nil)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/signin?next=%2Fnotes%2Fa.note%3Fx%3D1" {
		t.Fatalf("got %d to %q", rec.Code, rec.Header().Get("Location"))
	}
	if strings.Contains(rec.Body.String(), "the notes") {
		t.Fatal("a signed-out page showed the notes")
	}
}

func TestSigninSignedOutCommentRefused(t *testing.T) {
	_, h := testGate(t)
	rec := ask(h, "POST", "/notes/a.note/comments", "", url.Values{"text": {"hi"}}, nil)
	if rec.Code != http.StatusUnauthorized || strings.Contains(rec.Body.String(), "the notes") {
		t.Fatalf("got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestSigninWrongPassword(t *testing.T) {
	_, h := testGate(t)
	rec := ask(h, "POST", "/signin", "", url.Values{"password": {"open sesam"}, "next": {"/"}}, nil)
	if rec.Code != http.StatusUnauthorized || len(rec.Result().Cookies()) != 0 || !strings.Contains(rec.Body.String(), "wrong") {
		t.Fatalf("got %d, %d cookies", rec.Code, len(rec.Result().Cookies()))
	}
}

func TestSigninRightPasswordThenTheCookieServes(t *testing.T) {
	g, h := testGate(t)
	rec := ask(h, "POST", "/signin", "", url.Values{"password": {"open sesame"}, "next": {"/notes/a.note"}}, nil)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/notes/a.note" {
		t.Fatalf("got %d to %q", rec.Code, rec.Header().Get("Location"))
	}
	cs := rec.Result().Cookies()
	if len(cs) != 1 || cs[0].Name != signinCookie || cs[0].Value != g.cookie || !cs[0].HttpOnly || cs[0].SameSite != http.SameSiteLaxMode {
		t.Fatalf("cookies: %+v", cs)
	}
	if got := ask(h, "GET", "/", "", nil, cs[0]); got.Code != http.StatusOK || got.Body.String() != "the notes" {
		t.Fatalf("with the cookie: %d %q", got.Code, got.Body.String())
	}
	if got := ask(h, "POST", "/notes/a.note/comments", "", url.Values{"text": {"hi"}}, cs[0]); got.Code != http.StatusOK {
		t.Fatalf("a comment with the cookie: %d", got.Code)
	}
	forged := &http.Cookie{Name: signinCookie, Value: strings.Repeat("0", len(g.cookie))}
	if got := ask(h, "GET", "/", "", nil, forged); got.Code != http.StatusSeeOther {
		t.Fatalf("a forged cookie: %d", got.Code)
	}
}

func TestSigninLoopbackNeedsNone(t *testing.T) {
	_, h := testGate(t)
	for _, remote := range []string{"127.0.0.1:40000", "[::1]:40000"} {
		if got := ask(h, "GET", "/", remote, nil, nil); got.Code != http.StatusOK || got.Body.String() != "the notes" {
			t.Fatalf("from %s: %d", remote, got.Code)
		}
	}
}

func TestSigninNextStaysOnThisServer(t *testing.T) {
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

func TestSigninEmptyPasswordFileRefused(t *testing.T) {
	p := filepath.Join(t.TempDir(), "password")
	if err := os.WriteFile(p, []byte(" \n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := newSignin(p); err == nil {
		t.Fatal("an empty password file gave a sign-in")
	}
	if _, err := newSignin(filepath.Join(t.TempDir(), "none")); err == nil {
		t.Fatal("a missing password file gave a sign-in")
	}
}

func TestCheckOpen(t *testing.T) {
	for _, c := range []struct {
		addr, file string
		ok         bool
	}{
		{"127.0.0.1:9301", "", true},
		{"localhost:9301", "", true},
		{"[::1]:9301", "", true},
		{"0.0.0.0:9301", "", false},
		{":9301", "", false},
		{"203.0.113.9:9301", "", false},
		{"0.0.0.0:9301", "/data/password", true},
	} {
		if err := checkOpen(c.addr, c.file); (err == nil) != c.ok {
			t.Errorf("checkOpen(%q, %q): %v", c.addr, c.file, err)
		}
	}
}
