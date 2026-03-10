package state

import (
	"context"
	"errors"
	"sync"
)

const (
	CtxKey = "leader:value"
)

var (
	ErrValueNotFound = errors.New("value not found")
)

type Value struct {
	ThreadID  string `json:"thread_id"`
	UserID    string `json:"user_id"`
	namespace map[string]interface{}
	lock      sync.RWMutex
}

func InitCtx(ctx context.Context, userID, threadID string) context.Context {
	return context.WithValue(ctx, CtxKey, &Value{
		ThreadID: threadID,
		UserID:   userID,
	})
}

func GetValue(ctx context.Context) (*Value, error) {
	val, ok := ctx.Value(CtxKey).(*Value)
	if !ok {
		return nil, ErrValueNotFound
	}
	return val, nil
}

func (v *Value) Set(key string, value interface{}) {
	v.lock.Lock()
	defer v.lock.Unlock()
	if v.namespace == nil {
		v.namespace = make(map[string]interface{})
	}
	v.namespace[key] = value
}

func (v *Value) Get(key string) interface{} {
	v.lock.RLock()
	defer v.lock.RUnlock()
	if v.namespace == nil {
		return nil
	}
	return v.namespace[key]
}
