package handler

import "testing"

func TestValidate(t *testing.T) {
	ok := map[string]string{}
	cases := []struct {
		r        request
		needData bool
		valid    bool
	}{
		{request{Username: "mimi_01", Code: "123456", Data: nil}, false, true},
		{request{Username: "ab", Code: "123456", Data: nil}, false, false},
		{request{Username: "Mimi", Code: "123456", Data: nil}, false, false},
		{request{Username: "mimi", Code: "12345", Data: nil}, false, false},
		{request{Username: "mimi", Code: "12345a", Data: nil}, false, false},
		{request{Username: "mimi", Code: "123456", Data: nil}, true, false},
		{request{Username: "mimi", Code: "123456", Data: ok}, true, true},
	}
	for i, c := range cases {
		if got := validate(c.r, c.needData) == ""; got != c.valid {
			t.Errorf("case %d: valid=%v want %v", i, got, c.valid)
		}
	}
}

func TestAllowOrigin(t *testing.T) {
	const allowed = "https://imiah.vercel.app/, https://x.github.io"
	cases := []struct{ origin, allowed, want string }{
		{"https://imiah.vercel.app", allowed, "https://imiah.vercel.app"},
		{"https://x.github.io", allowed, "https://x.github.io"},
		{"http://localhost:5173", allowed, "http://localhost:5173"},
		{"http://127.0.0.1:3000", allowed, "http://127.0.0.1:3000"},
		{"https://evil.com", allowed, ""},
		{"https://anything.com", "", "*"},
	}
	for _, c := range cases {
		if got := allowOrigin(c.origin, c.allowed); got != c.want {
			t.Errorf("allowOrigin(%q) = %q, want %q", c.origin, got, c.want)
		}
	}
}
