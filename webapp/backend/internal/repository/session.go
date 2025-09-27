package repository

import (
	cache "backend/internal"
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

type SessionRepository struct {
	db DBTX
}

func NewSessionRepository(db DBTX) *SessionRepository {
	// TODO: バルクインサートしたい
	// go bulkInsertSession(db)
	return &SessionRepository{db: db}
}

// type Session struct {
// 	sessionID string    `db:"session_id"`
// 	userID    int       `db:"user_id"`
// 	expiredAt time.Time `db:"expires_at"`
// }

// var sessionCh = make(chan *Session, 1200)

// func bulkInsertSession(db DBTX) {
// 	var values [1200]*Session
// 	ticker := time.NewTicker(500 * time.Millisecond)
// 	i := 0
// 	for {
// 		if i > 1000 {
// 			query := "INSERT INTO user_sessions (session_uuid, user_id, expires_at) VALUES (:session_id, :user_id, :expires_at)"
// 			_, _ = db.NamedExec(query, values[0:i+1])
// 			i = 0
// 		}
// 		select {
// 		case <-ticker.C:
// 			if i > 0 {
// 				query := "INSERT INTO user_sessions (session_uuid, user_id, expires_at) VALUES (:session_id, :user_id, :expires_at)"
// 				_, _ = db.NamedExec(query, values[0:i+1])
// 				i = 0
// 			}
// 		case values[i] = <-sessionCh:
// 			i++
// 		}
// 	}
// }

// セッションを作成し、セッションIDと有効期限を返す
func (r *SessionRepository) Create(ctx context.Context, userBusinessID int, duration time.Duration) (string, time.Time, error) {
	sessionUUID, err := uuid.NewRandom()
	if err != nil {
		return "", time.Time{}, err
	}
	expiresAt := time.Now().Add(duration)
	sessionIDStr := sessionUUID.String()

	query := "INSERT INTO user_sessions (session_uuid, user_id, expires_at) VALUES (?, ?, ?)"
	_, err = r.db.ExecContext(ctx, query, sessionIDStr, userBusinessID, expiresAt)
	if err != nil {
		return "", time.Time{}, err
	}
	cache.Cache.Session.Lock()
	cache.Cache.Sessions[sessionIDStr] = struct {
		UserID    int
		ExpiresAt time.Time
	}{userBusinessID, expiresAt}
	cache.Cache.Session.Unlock()
	// sessionCh <- &Session{sessionIDStr, userBusinessID, expiresAt}
	return sessionIDStr, expiresAt, nil
}

// セッションIDからユーザーIDを取得
func (r *SessionRepository) FindUserBySessionID(ctx context.Context, sessionID string) (int, error) {
	var userID int
	cache.Cache.Session.Lock()
	defer cache.Cache.Session.Unlock()
	session, ok := cache.Cache.Sessions[sessionID]
	if !ok {
		return 0, errors.New("session not found")
	}
	if session.ExpiresAt.Before(time.Now()) {
		delete(cache.Cache.Sessions, sessionID)
		return 0, errors.New("session has been expired")
	}
	return userID, nil
}
