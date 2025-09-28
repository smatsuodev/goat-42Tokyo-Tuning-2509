package service

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"log"
	"time"

	cache "backend/internal"
	"backend/internal/repository"
	"backend/internal/utils"

	"go.opentelemetry.io/otel"
)

var (
	ErrUserNotFound    = errors.New("user not found")
	ErrInvalidPassword = errors.New("invalid password")
	ErrInternalServer  = errors.New("internal server error")
)

type AuthService struct {
	store *repository.Store
}

func NewAuthService(store *repository.Store) *AuthService {
	return &AuthService{store: store}
}

func (s *AuthService) Login(ctx context.Context, userName, password string) (string, time.Time, error) {
	ctx, span := otel.Tracer("service.auth").Start(ctx, "AuthService.Login")
	defer span.End()

	var sessionID string
	var expiresAt time.Time
	user, err := s.store.UserRepo.FindByUserName(ctx, userName)
	if err != nil {
		log.Printf("[Login] ユーザー検索失敗(userName: %s): %v", userName, err)
		if errors.Is(err, sql.ErrNoRows) {
			return "", time.Time{}, ErrUserNotFound
		}
		return "", time.Time{}, ErrInternalServer

	}
	if !cache.Cache.IsHashed[user.UserID] {
		err = utils.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
		if err != nil {
			log.Printf("[Login] パスワード検証失敗: %v", err)
			span.RecordError(err)
			return "", time.Time{}, ErrInvalidPassword
		}
		cache.Cache.Password[user.UserID] = sha256.Sum256([]byte(password))
		cache.Cache.IsHashed[user.UserID] = true
	} else {
		if cache.Cache.Password[user.UserID] != sha256.Sum256([]byte(password)) {
			log.Printf("[Login] パスワード検証失敗")
			return "", time.Time{}, ErrInvalidPassword
		}
	}

	sessionDuration := 24 * time.Hour
	sessionID, expiresAt, err = s.store.SessionRepo.Create(ctx, user.UserID, sessionDuration)
	if err != nil {
		log.Printf("[Login] セッション生成失敗: %v", err)
		return "", time.Time{}, ErrInternalServer
	}

	log.Printf("Login successful for UserName '%s', session created.", userName)
	return sessionID, expiresAt, nil
}
