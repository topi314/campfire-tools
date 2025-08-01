package auth

import (
	"context"
	"math/rand"
	"net/http"

	"github.com/topi314/campfire-tools/server/database"
)

var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890")

type sessionKey struct{}

var sessionContextKey = &sessionKey{}

func SetSession(ctx context.Context, session database.SessionWithUserSetting) context.Context {
	return context.WithValue(ctx, sessionContextKey, session)
}

func GetSession(r *http.Request) database.SessionWithUserSetting {
	return r.Context().Value(sessionContextKey).(database.SessionWithUserSetting)
}

func RandomStr(length int) string {
	b := make([]rune, length)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
