package helper

import (
	"errors"
	"github.com/golang-jwt/jwt/v5"
	"siakad-uts/app/model"
	"strconv"
	"time"
)

type Claims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}
type JWT struct {
	secret []byte
	ttl    time.Duration
}

func NewJWT(secret string, ttl time.Duration) *JWT { return &JWT{[]byte(secret), ttl} }
func (j *JWT) TTL() int                            { return int(j.ttl.Seconds()) }
func (j *JWT) Issue(u model.User) (string, error) {
	now := time.Now()
	c := Claims{Role: u.Role, RegisteredClaims: jwt.RegisteredClaims{Subject: strconv.Itoa(u.ID), IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(j.ttl)), Issuer: "siakad-uts"}}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(j.secret)
}
func (j *JWT) Parse(raw string) (int, error) {
	c := &Claims{}
	t, e := jwt.ParseWithClaims(raw, c, func(t *jwt.Token) (any, error) {
		if t.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, errors.New("unexpected signing algorithm")
		}
		return j.secret, nil
	}, jwt.WithIssuer("siakad-uts"), jwt.WithExpirationRequired(), jwt.WithValidMethods([]string{"HS256"}))
	if e != nil || !t.Valid {
		return 0, errors.New("token tidak valid atau kedaluwarsa")
	}
	id, e := strconv.Atoi(c.Subject)
	if e != nil || id <= 0 {
		return 0, errors.New("subject tidak valid")
	}
	return id, nil
}
