package auth

import (
	"net/http"

	jimohttp "github.com/jimo-go/framework/http"
)

const sessionUserIDKey = "auth.user_id"

func Login(ctx *jimohttp.Context, userID int) {
	s := ctx.Session()
	if s == nil {
		panic(jimohttp.HTTPError{Status: http.StatusInternalServerError, Message: "Session is not enabled"})
	}
	s.Put(sessionUserIDKey, userID)
}

func Logout(ctx *jimohttp.Context) {
	s := ctx.Session()
	if s == nil {
		return
	}
	s.Put(sessionUserIDKey, nil)
}

func UserID(ctx *jimohttp.Context) (int, bool) {
	s := ctx.Session()
	if s == nil {
		return 0, false
	}
	v := s.Get(sessionUserIDKey)
	switch x := v.(type) {
	case int:
		return x, true
	case float64:
		return int(x), true
	default:
		return 0, false
	}
}

func RequireAuth() jimohttp.Middleware {
	return func(next jimohttp.HandlerFunc) jimohttp.HandlerFunc {
		return func(ctx *jimohttp.Context) {
			if _, ok := UserID(ctx); !ok {
				panic(jimohttp.HTTPError{Status: http.StatusUnauthorized, Message: "Unauthenticated"})
			}
			next(ctx)
		}
	}
}
