package main

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	d "moo2manager/shared/distribution"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Both recovery files and every response are untrusted until checked. Do not use
// environment proxies, DNS names, redirects, query tokens, or arbitrary URLs.
func localURL(raw string) (*url.URL, error) {
	u, e := url.Parse(strings.TrimSpace(raw))
	if e != nil {
		return nil, errors.New("invalid local session URL")
	}
	if u.Scheme != "http" || u.Hostname() != "127.0.0.1" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Path != "/" || u.RawPath != "" || u.Opaque != "" || u.RawFragment != "" {
		return nil, errors.New("unsafe local session URL refused")
	}
	port, e := strconv.Atoi(u.Port())
	if e != nil || port < 1 || port > 65535 || u.Host != "127.0.0.1:"+strconv.Itoa(port) {
		return nil, errors.New("invalid local session port")
	}
	secret, e := hex.DecodeString(u.Fragment)
	if e != nil || len(secret) != 32 || len(u.Fragment) != 64 || strings.ToLower(u.Fragment) != u.Fragment {
		return nil, errors.New("invalid local session credential")
	}
	return u, nil
}
func loopbackClient() *http.Client {
	return &http.Client{Timeout: 2 * time.Second, Transport: &http.Transport{Proxy: nil}, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("local session redirect refused") }}
}
func sessionJSON(raw, endpoint string, v any) error {
	u, e := localURL(raw)
	if e != nil {
		return e
	}
	secret := u.Fragment
	u.Fragment = ""
	u.Path = endpoint
	req, e := http.NewRequest("GET", u.String(), nil)
	if e != nil {
		return e
	}
	req.Header.Set("Authorization", "Bearer "+secret)
	client := loopbackClient()
	defer client.CloseIdleConnections()
	resp, e := client.Do(req)
	if e != nil {
		return errors.New("local launcher is not responding")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("local launcher refused authentication (%d)", resp.StatusCode)
	}
	decoder := json.NewDecoder(io.LimitReader(resp.Body, 2<<20))
	return decoder.Decode(v)
}
func readSession(path string) (string, error) {
	if e := d.NoLinks(path); e != nil {
		return "", e
	}
	st, e := os.Stat(path)
	if e != nil {
		return "", e
	}
	if !st.Mode().IsRegular() || st.Size() > 4096 {
		return "", errors.New("invalid local session file")
	}
	b, e := os.ReadFile(path)
	if e != nil {
		return "", e
	}
	raw := strings.TrimSpace(string(b))
	_, e = localURL(raw)
	return raw, e
}
func managerSession(root, version string) (string, error) {
	raw, e := readSession(filepath.Join(root, "userdata", "open-launcher.txt"))
	if e != nil {
		return "", e
	}
	var state struct{ Root, Data, Version string }
	if e = sessionJSON(raw, "/api/identity", &state); e != nil {
		return "", e
	}
	if state.Version != version || filepath.Clean(state.Root) != filepath.Clean(root) || filepath.Clean(state.Data) != filepath.Join(root, "userdata") {
		return "", errors.New("local launcher identity does not match this installation")
	}

	return raw, nil
}
func bootSession(root string) (string, error) {
	raw, e := readSession(filepath.Join(root, "desktop-session", "open.txt"))
	if e != nil {
		return "", e
	}
	var state bootState
	if e = sessionJSON(raw, "/api/status", &state); e != nil {
		return "", e
	}
	if state.Root != root || state.Version != currentVersion || state.Kind != "moo2-private-desktop" {
		return "", errors.New("local startup identity does not match this installation")
	}
	return raw, nil
}
