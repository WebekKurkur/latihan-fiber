package middleware

import (
	"siakad-uts/helper"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
)

type loginAttempt struct {
	Count int
	Until time.Time
}
type LoginLimiter struct {
	mu       sync.Mutex
	failures map[string]loginAttempt
}

func NewLoginLimiter() *LoginLimiter { return &LoginLimiter{failures: make(map[string]loginAttempt)} }
func (l *LoginLimiter) Guard(c *fiber.Ctx) error {
	ip := c.IP()
	l.mu.Lock()
	v := l.failures[ip]
	if time.Now().After(v.Until) {
		v = loginAttempt{}
		delete(l.failures, ip)
	}
	l.mu.Unlock()
	if v.Count >= 5 {
		return helper.Error(429, "Terlalu banyak gagal login; coba lagi dalam satu menit")
	}
	return c.Next()
}
func (l *LoginLimiter) Record(ip string, failed bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !failed {
		delete(l.failures, ip)
		return
	}
	v := l.failures[ip]
	if time.Now().After(v.Until) {
		v = loginAttempt{Until: time.Now().Add(time.Minute)}
	}
	v.Count++
	l.failures[ip] = v
}
