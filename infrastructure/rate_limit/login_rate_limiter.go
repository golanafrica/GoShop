package ratelimit

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"Goshop/domain/service"

	"github.com/redis/go-redis/v9"
)

// Limites configurables
const (
	// Par email : 5 tentatives toutes les 15 minutes
	EmailLimit  = 5
	EmailWindow = 15 * time.Minute

	// Par IP : 20 tentatives toutes les 15 minutes
	IPLimit  = 20
	IPWindow = 15 * time.Minute

	// Global : 100 tentatives toutes les 1 minute (anti-DDoS)
	GlobalLimit  = 100
	GlobalWindow = 1 * time.Minute
)

// Erreurs de rate limiting
var (
	ErrEmailRateLimitExceeded  = fmt.Errorf("too many login attempts for this email, please try again later")
	ErrIPRateLimitExceeded     = fmt.Errorf("too many login attempts from this IP, please try again later")
	ErrGlobalRateLimitExceeded = fmt.Errorf("too many login attempts, please try again later")
)

// LoginRateLimiterRedis implémente LoginRateLimiter avec Redis
type LoginRateLimiterRedis struct {
	client *redis.Client
}

// NewLoginRateLimiterRedis crée une nouvelle instance
func NewLoginRateLimiterRedis(client *redis.Client) service.LoginRateLimiter {
	return &LoginRateLimiterRedis{
		client: client,
	}
}

// CheckEmailLimit vérifie la limite par email
func (r *LoginRateLimiterRedis) CheckEmailLimit(ctx context.Context, email string) error {
	if r.client == nil {
		return nil // Redis non disponible, on laisse passer
	}

	// Normaliser l'email (lowercase, trim)
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return nil
	}

	key := fmt.Sprintf("ratelimit:login:email:%s", email)
	count, err := r.client.Get(ctx, key).Int()
	if err == redis.Nil {
		return nil // Pas encore de tentatives
	}
	if err != nil {
		// Erreur Redis, on laisse passer (fail-open pour ne pas bloquer les utilisateurs)
		return nil
	}

	if count >= EmailLimit {
		return ErrEmailRateLimitExceeded
	}

	return nil
}

// CheckIPLimit vérifie la limite par IP
func (r *LoginRateLimiterRedis) CheckIPLimit(ctx context.Context, ip string) error {
	if r.client == nil {
		return nil
	}

	if ip == "" || ip == "unknown" {
		return nil
	}

	key := fmt.Sprintf("ratelimit:login:ip:%s", ip)
	count, err := r.client.Get(ctx, key).Int()
	if err == redis.Nil {
		return nil
	}
	if err != nil {
		return nil
	}

	if count >= IPLimit {
		return ErrIPRateLimitExceeded
	}

	return nil
}

// RecordFailedAttempt enregistre une tentative échouée
func (r *LoginRateLimiterRedis) RecordFailedAttempt(ctx context.Context, email, ip string) error {
	if r.client == nil {
		return nil
	}

	// Normaliser l'email
	email = strings.ToLower(strings.TrimSpace(email))

	// Pipeline pour exécuter toutes les commandes atomiquement
	pipe := r.client.Pipeline()

	// Incrémenter le compteur email
	if email != "" {
		emailKey := fmt.Sprintf("ratelimit:login:email:%s", email)
		pipe.Incr(ctx, emailKey)
		pipe.Expire(ctx, emailKey, EmailWindow)
	}

	// Incrémenter le compteur IP
	if ip != "" && ip != "unknown" {
		ipKey := fmt.Sprintf("ratelimit:login:ip:%s", ip)
		pipe.Incr(ctx, ipKey)
		pipe.Expire(ctx, ipKey, IPWindow)
	}

	// Incrémenter le compteur global
	globalKey := "ratelimit:login:global"
	pipe.Incr(ctx, globalKey)
	pipe.Expire(ctx, globalKey, GlobalWindow)

	_, err := pipe.Exec(ctx)
	return err
}

// ResetOnSuccess réinitialise les compteurs après une connexion réussie
func (r *LoginRateLimiterRedis) ResetOnSuccess(ctx context.Context, email, ip string) error {
	if r.client == nil {
		return nil
	}

	email = strings.ToLower(strings.TrimSpace(email))

	pipe := r.client.Pipeline()

	if email != "" {
		emailKey := fmt.Sprintf("ratelimit:login:email:%s", email)
		pipe.Del(ctx, emailKey)
	}

	if ip != "" && ip != "unknown" {
		ipKey := fmt.Sprintf("ratelimit:login:ip:%s", ip)
		pipe.Del(ctx, ipKey)
	}

	_, err := pipe.Exec(ctx)
	return err
}

// ============================================================
// Fallback : Implémentation en mémoire (si Redis down)
// ============================================================

// LoginRateLimiterMemory implémente LoginRateLimiter en mémoire
// Utilisé comme fallback si Redis n'est pas disponible
type LoginRateLimiterMemory struct {
	mu     sync.RWMutex
	email  map[string]*bucket
	ip     map[string]*bucket
	global *bucket
}

type bucket struct {
	count   int
	expires time.Time
}

// NewLoginRateLimiterMemory crée une nouvelle instance en mémoire
func NewLoginRateLimiterMemory() service.LoginRateLimiter {
	return &LoginRateLimiterMemory{
		email:  make(map[string]*bucket),
		ip:     make(map[string]*bucket),
		global: &bucket{},
	}
}

func (m *LoginRateLimiterMemory) CheckEmailLimit(ctx context.Context, email string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return nil
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	b, exists := m.email[email]
	if !exists || time.Now().After(b.expires) {
		return nil
	}

	if b.count >= EmailLimit {
		return ErrEmailRateLimitExceeded
	}

	return nil
}

func (m *LoginRateLimiterMemory) CheckIPLimit(ctx context.Context, ip string) error {
	if ip == "" || ip == "unknown" {
		return nil
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	b, exists := m.ip[ip]
	if !exists || time.Now().After(b.expires) {
		return nil
	}

	if b.count >= IPLimit {
		return ErrIPRateLimitExceeded
	}

	return nil
}

func (m *LoginRateLimiterMemory) RecordFailedAttempt(ctx context.Context, email, ip string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	now := time.Now()

	m.mu.Lock()
	defer m.mu.Unlock()

	// Email
	if email != "" {
		b, exists := m.email[email]
		if !exists || now.After(b.expires) {
			m.email[email] = &bucket{count: 1, expires: now.Add(EmailWindow)}
		} else {
			b.count++
		}
	}

	// IP
	if ip != "" && ip != "unknown" {
		b, exists := m.ip[ip]
		if !exists || now.After(b.expires) {
			m.ip[ip] = &bucket{count: 1, expires: now.Add(IPWindow)}
		} else {
			b.count++
		}
	}

	// Global
	if now.After(m.global.expires) {
		m.global = &bucket{count: 1, expires: now.Add(GlobalWindow)}
	} else {
		m.global.count++
	}

	return nil
}

func (m *LoginRateLimiterMemory) ResetOnSuccess(ctx context.Context, email, ip string) error {
	email = strings.ToLower(strings.TrimSpace(email))

	m.mu.Lock()
	defer m.mu.Unlock()

	if email != "" {
		delete(m.email, email)
	}

	if ip != "" && ip != "unknown" {
		delete(m.ip, ip)
	}

	return nil
}
