package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"
)

type ctxKey string

const ctxDeviceID ctxKey = "device_id"

// GenerateToken returns a 32-byte random hex string (64 chars).
func GenerateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// GeneratePairingCode returns a 6-digit numeric code (zero-padded).
func GeneratePairingCode() (string, error) {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	n := uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
	code := int(n % 1000000)
	out := make([]byte, 6)
	for i := 5; i >= 0; i-- {
		out[i] = byte('0' + code%10)
		code /= 10
	}
	return string(out), nil
}

// HashToken returns SHA-256 of the bearer token. Tokens are stored hashed so a DB leak
// doesn't grant immediate device access.
func HashToken(t string) string {
	h := sha256.Sum256([]byte(t))
	return hex.EncodeToString(h[:])
}

// Middleware validates the Authorization: Bearer <token> header against the devices table.
// Sets the device ID into request context on success.
func Middleware(db *sql.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tok, err := extractBearer(r)
			if err != nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			var deviceID string
			var revoked sql.NullInt64
			err = db.QueryRowContext(r.Context(),
				`SELECT id, revoked_at FROM devices WHERE token_hash = ?`,
				HashToken(tok)).Scan(&deviceID, &revoked)
			if err != nil || (revoked.Valid && revoked.Int64 > 0) {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			_, _ = db.ExecContext(r.Context(),
				`UPDATE devices SET last_seen = ? WHERE id = ?`, time.Now().Unix(), deviceID)
			ctx := context.WithValue(r.Context(), ctxDeviceID, deviceID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// DeviceID pulls the authenticated device ID out of the request context.
func DeviceID(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(ctxDeviceID).(string)
	return v, ok
}

func extractBearer(r *http.Request) (string, error) {
	h := r.Header.Get("Authorization")
	if h == "" {
		return "", errors.New("no auth header")
	}
	parts := strings.SplitN(h, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", errors.New("malformed auth header")
	}
	return parts[1], nil
}
