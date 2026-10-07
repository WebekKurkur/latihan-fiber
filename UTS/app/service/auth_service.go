package service

import (
	"context"
	"errors"
	"siakad-uts/app/model"
	"siakad-uts/app/repository"
	"siakad-uts/helper"
	"strings"
)

type AuthService struct {
	Users repository.UserRepository
	JWT   *helper.JWT
}

func (s AuthService) Login(ctx context.Context, r model.LoginRequest) (any, error) {
	r.Email = strings.TrimSpace(r.Email)
	if e := helper.ValidateStruct(r); e != nil {
		return nil, e
	}
	u, e := s.Users.ByEmail(ctx, r.Email)
	if e != nil {
		if !errors.Is(e, repository.ErrNotFound) {
			return nil, helper.Internal(e)
		}
		return nil, helper.Error(401, "Email atau password salah")
	}
	if !helper.VerifyPassword(u.Password, r.Password) {
		return nil, helper.Error(401, "Email atau password salah")
	}
	token, e := s.JWT.Issue(u)
	if e != nil {
		return nil, helper.Internal(e)
	}
	return map[string]any{"access_token": token, "token_type": "Bearer", "expires_in": s.JWT.TTL(), "user": u}, nil
}
func (s AuthService) Me(ctx context.Context, id int) (any, error) {
	u, e := s.Users.ByID(ctx, id)
	if errors.Is(e, repository.ErrNotFound) {
		return nil, helper.Error(401, "Akun tidak aktif")
	}
	if e != nil {
		return nil, helper.Internal(e)
	}
	if u.StudentID == nil {
		return map[string]any{"user": u}, nil
	}
	return map[string]any{"user": u}, nil
}
