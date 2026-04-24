package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// RateLimiter is a fixed-window Redis-backed limiter sized for go-live: it's
// not the most sophisticated algorithm in the world, but it's a few hundred
// lines of correctness instead of a few thousand, and it's enough to keep the
// public auth + webhook endpoints from being open-sesame for fraud bots and
// password-reset spammers.
//
// Each call to Limit() takes a key (e.g. "ip:1.2.3.4") and bumps a counter
// in Redis with a TTL equal to `window`. If the counter goes above `max`,
// the request gets a 429 with a Retry-After header.
//
// The "fixed window" model means a burst at the very end of one window plus
// a burst at the very start of the next can briefly double the effective
// rate. For our use cases (signup ~5/15min, login ~10/15min, password reset
// ~3/hour, webhook ~60/min) that's fine — we're protecting from order-of-
// magnitude abuse, not nudging a knob.
type RateLimiter struct {
	rdb    *redis.Client
	prefix string
}

// NewRateLimiter returns a limiter rooted under a Redis key prefix so two
// API instances sharing Redis don't trample each other's counters.
func NewRateLimiter(rdb *redis.Client) *RateLimiter {
	return &RateLimiter{rdb: rdb, prefix: "rl:"}
}

// Limit returns middleware that enforces `max` requests per `window` per
// `keyFn(r)`. keyFn returns the bucket identity (typically "ip:..." or
// "email:hash..."); return "" to skip rate-limiting entirely (e.g. when the
// request can't be keyed safely).
//
// We INTENTIONALLY don't peek the JSON body to extract emails for keying —
// that would mean reading + replaying the body on every request just to
// rate-limit the unauthenticated path. The IP key catches spray attacks;
// the email-aware path is a separate per-route helper (LimitByEmail) used
// only where we already need to parse the body.
func (rl *RateLimiter) Limit(max int, window time.Duration, keyFn func(r *http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := keyFn(r)
			if key == "" {
				next.ServeHTTP(w, r)
				return
			}

			// Bucket the window so all clients share the same fixed-window
			// boundary — simpler reasoning than a sliding window and the
			// only data Redis needs to keep is the integer + TTL.
			bucket := time.Now().Truncate(window).Unix()
			redisKey := fmt.Sprintf("%s%s:%d", rl.prefix, key, bucket)

			ctx, cancel := context.WithTimeout(r.Context(), 200*time.Millisecond)
			defer cancel()

			pipe := rl.rdb.TxPipeline()
			incr := pipe.Incr(ctx, redisKey)
			pipe.Expire(ctx, redisKey, window+time.Second) // +1s slack so we never expire mid-bucket
			if _, err := pipe.Exec(ctx); err != nil {
				// Fail OPEN on Redis blips — better to serve a request than
				// to take down auth because the cache hiccupped.
				next.ServeHTTP(w, r)
				return
			}

			count := incr.Val()
			if count > int64(max) {
				retryAfter := window - time.Since(time.Unix(bucket, 0))
				if retryAfter < time.Second {
					retryAfter = time.Second
				}
				w.Header().Set("Retry-After", fmt.Sprintf("%d", int(retryAfter.Seconds())))
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte(`{"error":"rate_limited","message":"Too many requests. Please try again shortly."}`))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// LimitByEmail rate-limits POST endpoints that key off an "email" field in
// the JSON body — used for /forgot-password to stop reset-email spam at any
// specific user, regardless of attacker IP rotation.
//
// We read + restore the body so the downstream handler still gets it. Body
// is capped at 4KB before we parse it — these endpoints don't need more.
func (rl *RateLimiter) LimitByEmail(max int, window time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, err := io.ReadAll(io.LimitReader(r.Body, 4096))
			if err != nil {
				next.ServeHTTP(w, r) // fail open on a body-read blip
				return
			}
			r.Body = io.NopCloser(strings.NewReader(string(body)))

			var probe struct {
				Email string `json:"email"`
			}
			_ = json.Unmarshal(body, &probe)
			email := strings.ToLower(strings.TrimSpace(probe.Email))
			if email == "" {
				// No email to key on; fall through. The IP-based limiter
				// applied alongside this one still protects the route.
				next.ServeHTTP(w, r)
				return
			}

			// Hash the email so we don't store PII in Redis keys.
			sum := sha256.Sum256([]byte(email))
			key := "email:" + hex.EncodeToString(sum[:8]) // 8 bytes = 16 hex chars; collisions don't matter here

			rl.Limit(max, window, func(r *http.Request) string { return key })(next).ServeHTTP(w, r)
		})
	}
}

// IPKey returns the request's real IP for use as a rate-limit bucket. Honors
// X-Forwarded-For (chi's RealIP middleware sets RemoteAddr correctly when
// chained after RealIP, but we don't depend on that being installed).
func IPKey(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		// First value is the original client; the rest are proxies.
		if idx := strings.IndexByte(fwd, ','); idx > 0 {
			return "ip:" + strings.TrimSpace(fwd[:idx])
		}
		return "ip:" + strings.TrimSpace(fwd)
	}
	addr := r.RemoteAddr
	// Strip the port — bucket by IP, not by ephemeral source port.
	if idx := strings.LastIndexByte(addr, ':'); idx > 0 {
		addr = addr[:idx]
	}
	return "ip:" + addr
}
