package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalURL(t *testing.T) {
	good := "http://127.0.0.1:12345/#" + strings.Repeat("a", 64)
	if _, e := localURL(good); e != nil {
		t.Fatal(e)
	}
	for _, bad := range []string{
		strings.Replace(good, "127.0.0.1", "localhost", 1), strings.Replace(good, "127.0.0.1", "evil.example", 1), strings.Replace(good, "127.0.0.1", "[::1]", 1), strings.Replace(good, "http:", "https:", 1), strings.Replace(good, "http://", "http://user:pass@", 1), strings.Replace(good, "/#", "/?q=x#", 1), strings.Replace(good, "/#", "/other#", 1), strings.Replace(good, ":12345", ":0", 1), strings.Replace(good, ":12345", ":012345", 1), strings.Replace(good, ":12345", ":65536", 1), strings.Replace(good, "/#", "/%2f#", 1), "file:///tmp/file#" + strings.Repeat("a", 64), good + "\nhttp://evil.example",
		"http://127.0.0.1:12345/#" + strings.Repeat("%61", 64),
		"http://127.0.0.1:12345/#" + strings.Repeat("A", 64), good[:len(good)-1], strings.Replace(good, "/#", "/?#", 1),
	} {
		t.Run(bad, func(t *testing.T) {
			if _, e := localURL(bad); e == nil {
				t.Fatal("accepted unsafe URL")
			}
		})
	}
}
func TestSessionNoRedirectOrProxy(t *testing.T) {
	contacted := false
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { contacted = true; w.Write([]byte(`{}`)) }))
	defer destination.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, destination.URL, 302) }))
	defer redirect.Close()
	t.Setenv("HTTP_PROXY", destination.URL)
	t.Setenv("HTTPS_PROXY", destination.URL)
	t.Setenv("ALL_PROXY", destination.URL)
	var state bootState
	if e := sessionJSON(redirect.URL+"/#"+strings.Repeat("a", 64), "/api/status", &state); e == nil {
		t.Fatal("redirect accepted")
	}
	if contacted {
		t.Fatal("followed redirect/proxy")
	}
}
func TestManagerIdentity(t *testing.T) {
	root := t.TempDir()
	tok := strings.Repeat("b", 64)
	gotAuth := false
	badRoot := false
	badVersion := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization") == "Bearer "+tok
		v := currentVersion
		if badVersion {
			v = "0.0.1"
		}
		rr := root
		if badRoot {
			rr += "-other"
		}
		json.NewEncoder(w).Encode(map[string]any{"root": rr, "data": filepath.Join(root, "userdata"), "version": v})
	}))
	defer srv.Close()
	os.MkdirAll(filepath.Join(root, "userdata"), 0700)
	os.WriteFile(filepath.Join(root, "userdata", "open-launcher.txt"), []byte(srv.URL+"/#"+tok), 0600)
	if _, e := managerSession(root, currentVersion); e != nil {
		t.Fatal(e)
	}
	if !gotAuth {
		t.Fatal("missing authentication")
	}
	badRoot = true
	if _, e := managerSession(root, currentVersion); e == nil {
		t.Fatal("wrong root accepted")
	}
	badRoot = false
	badVersion = true
	if _, e := managerSession(root, currentVersion); e == nil {
		t.Fatal("wrong version accepted")
	}
}
func TestSessionFileRejectsSymlink(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target")
	os.WriteFile(target, []byte("http://127.0.0.1:12345/#"+strings.Repeat("a", 64)), 0600)
	link := filepath.Join(root, "link")
	if e := os.Symlink(target, link); e != nil {
		t.Skip("symlink unavailable")
	}
	if _, e := readSession(link); e == nil {
		t.Fatal("symlink accepted")
	}
}
func TestStartupAPI(t *testing.T) {
	s := &startup{host: "127.0.0.1:12345", token: strings.Repeat("a", 64), state: bootState{Kind: "moo2-private-desktop", Version: currentVersion, Root: "private", Busy: true}}
	for _, c := range []struct {
		name, path, method, host, auth, origin string
		want                                   int
	}{
		{"no auth", "/api/status", "GET", s.host, "", "", 401}, {"wrong host", "/api/status", "GET", "evil", s.token, "", 403}, {"cross origin", "/api/status", "GET", s.host, s.token, "http://evil", 403}, {"status", "/api/status", "GET", s.host, s.token, "", 200}, {"status method", "/api/status", "POST", s.host, s.token, "", 405}, {"retry method", "/api/retry", "GET", s.host, s.token, "", 405}, {"busy retry", "/api/retry", "POST", s.host, s.token, "", 409}, {"busy exit", "/api/exit", "POST", s.host, s.token, "", 409}, {"static", "/", "GET", s.host, "", "", 200}, {"missing", "/etc/passwd", "GET", s.host, "", "", 404},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := httptest.NewRequest(c.method, "http://"+s.host+c.path, nil)
			r.Host = c.host
			if c.auth != "" {
				r.Header.Set("Authorization", "Bearer "+c.auth)
			}
			if c.origin != "" {
				r.Header.Set("Origin", c.origin)
			}
			w := httptest.NewRecorder()
			s.handle(w, r)
			if w.Code != c.want {
				t.Fatalf("got %d want %d", w.Code, c.want)
			}
			if strings.Contains(w.Body.String(), s.token) {
				t.Fatal("token echoed")
			}
		})
	}
}

func TestInitialDeferralIsNotHeadlessPolicy(t *testing.T) {
	for _, headless := range []bool{false, true} {
		args := launcherArgs("private-root", headless)
		joined := " " + strings.Join(args, " ") + " "
		if strings.Contains(joined, " --no-browser ") != headless {
			t.Fatal("explicit headless policy lost")
		}
		if strings.Contains(joined, " --defer-browser-once ") == headless {
			t.Fatal("initial deferral mixed with headless")
		}
		for _, flag := range []string{"--no-shortcut", "--private-url-file-only", "--desktop-gui"} {
			if !strings.Contains(joined, " "+flag+" ") {
				t.Fatal("private policy missing", flag)
			}
		}
	}
}
