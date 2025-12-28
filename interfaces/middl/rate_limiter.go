package middl

import (
	"Goshop/interfaces/utils"
	"context"
	"net"
	"net/http"
	"sync"
	"time"
)

// LIMIT: max requêtes par fenêtre
// WINDOW: durée de la fenêtre
const (
	RATE_LIMIT = 30
	WINDOW     = 1 * time.Minute
)

// Mémoire locale en fallback
type LocalBucket struct {
	Requests int
	Expires  time.Time
}

var (
	localStore   = make(map[string]*LocalBucket)
	localStoreMu sync.Mutex
)

// -------------------------
// Redis rate limiter
// -------------------------
func checkRedisLimit(ip string) (bool, error) {
	ctx := context.Background()
	key := "rl:" + ip

	// incr
	reqCount, err := utils.Rdb.Incr(ctx, key).Result()
	if err != nil {
		return false, err
	}

	if reqCount == 1 {
		utils.Rdb.Expire(ctx, key, WINDOW)
	}

	if reqCount > RATE_LIMIT {
		return false, nil
	}

	return true, nil
}

// -------------------------
// Fallback: In-memory limiter
// -------------------------
func checkLocalLimit(ip string) bool {
	localStoreMu.Lock()
	defer localStoreMu.Unlock()

	now := time.Now()

	b, exists := localStore[ip]
	if !exists || now.After(b.Expires) {
		localStore[ip] = &LocalBucket{
			Requests: 1,
			Expires:  now.Add(WINDOW),
		}
		return true
	}

	b.Requests++

	if b.Requests > RATE_LIMIT {
		return false
	}

	return true
}

// -------------------------
// Middleware Public
// -------------------------
func RateLimiter(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, _, _ := net.SplitHostPort(r.RemoteAddr)
		if ip == "" {
			ip = "unknown"
		}

		// Try Redis first
		ok, err := checkRedisLimit(ip)
		if err != nil {
			// Redis DOWN → fallback memory
			if !checkLocalLimit(ip) {
				http.Error(w, "Too Many Requests (fallback)", http.StatusTooManyRequests)
				return
			}
		} else if !ok {
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}
