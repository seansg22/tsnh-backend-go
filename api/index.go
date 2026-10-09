package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const maxBody = 4 << 20 // Vercel's request limit is 4.5 MB

var (
	pool    *pgxpool.Pool
	once    sync.Once
	poolErr error

	usernameRe = regexp.MustCompile(`^[a-z0-9_-]{3,32}$`)
	codeRe     = regexp.MustCompile(`^\d{6}$`)
)

// ponytail: lazy global pool, reused across warm invocations; no ORM/migrations.
func db(ctx context.Context) (*pgxpool.Pool, error) {
	once.Do(func() { pool, poolErr = pgxpool.New(ctx, os.Getenv("DATABASE_URL")) })
	return pool, poolErr
}

type request struct {
	Username string            `json:"username"`
	Code     string            `json:"code"`
	Data     map[string]string `json:"data"` // raw localStorage key -> raw string value
}

// validate checks the trust-boundary inputs; returns "" when ok.
func validate(r request, needData bool) string {
	switch {
	case !usernameRe.MatchString(r.Username):
		return "username must match [a-z0-9_-]{3,32}"
	case !codeRe.MatchString(r.Code):
		return "code must be 6 digits"
	case needData && r.Data == nil:
		return "data must be an object of string values"
	}
	return ""
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func fail(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func Handler(w http.ResponseWriter, r *http.Request) {
	origin := os.Getenv("ALLOWED_ORIGIN")
	if origin == "" {
		origin = "*"
	}
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	switch r.URL.Path {
	case "/", "/health":
		fmt.Fprint(w, "ok")
	case "/db":
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		p, err := db(ctx)
		var now time.Time
		if err == nil {
			err = p.QueryRow(ctx, "select now()").Scan(&now)
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		fmt.Fprint(w, now)
	case "/register", "/login", "/sync/push", "/sync/pull":
		api(w, r)
	default:
		http.NotFound(w, r)
	}
}

func api(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		fail(w, http.StatusMethodNotAllowed, "POST only")
		return
	}
	var req request
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBody)).Decode(&req); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			fail(w, http.StatusRequestEntityTooLarge, "body too large")
			return
		}
		fail(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if msg := validate(req, r.URL.Path == "/sync/push"); msg != "" {
		fail(w, http.StatusBadRequest, msg)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	p, err := db(ctx)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}

	if r.URL.Path == "/register" {
		tag, err := p.Exec(ctx, `insert into users (username, code) values ($1, $2) on conflict do nothing`, req.Username, req.Code)
		switch {
		case err != nil:
			fail(w, http.StatusInternalServerError, err.Error())
		case tag.RowsAffected() == 0:
			fail(w, http.StatusConflict, "username already taken")
		default:
			writeJSON(w, http.StatusCreated, map[string]string{"username": req.Username})
		}
		return
	}

	// login / push / pull: stateless auth, username+code checked on every call.
	var code string
	var data []byte
	var updatedAt time.Time
	err = p.QueryRow(ctx, `select code, data, updated_at from users where username = $1`, req.Username).Scan(&code, &data, &updatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, http.StatusNotFound, "user not found")
		return
	}
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if code != req.Code {
		fail(w, http.StatusUnauthorized, "wrong code")
		return
	}

	switch r.URL.Path {
	case "/login":
		writeJSON(w, http.StatusOK, map[string]string{"username": req.Username})
	case "/sync/pull":
		writeJSON(w, http.StatusOK, map[string]any{"data": json.RawMessage(data), "updated_at": updatedAt})
	case "/sync/push":
		// ponytail: last-write-wins whole snapshot, no merge/versioning.
		err = p.QueryRow(ctx, `update users set data = $2, updated_at = now() where username = $1 returning updated_at`, req.Username, req.Data).Scan(&updatedAt)
		if err != nil {
			fail(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"updated_at": updatedAt})
	}
}
