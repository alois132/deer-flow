package trace

import (
	"context"
	"github.com/google/uuid"
	"time"
)

const (
	CTX_DB_KEY        = "ctx:db"
	CTX_TRACE_ID      = "ctx:trace_id"
	CTXP_REQUEST_TIME = "ctx:request_time"
)

const (
	RESP_TRACE_ID     = "trace_id"
	RESP_REQUEST_TIME = "request_time"
)

// SetTraceID 设置 trace_id 到 Request Context
func SetTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, CTX_TRACE_ID, traceID)
}

// GetTraceID 从 Request Context 获取 trace_id
func GetTraceID(ctx context.Context) (string, bool) {
	traceID, ok := ctx.Value(CTX_TRACE_ID).(string)
	return traceID, ok
}

// GenerateTraceID 生成新的 trace_id
func GenerateTraceID() string {
	return uuid.New().String()
}

// SetRequestTime 设置请求开始时间
func SetRequestTime(ctx context.Context, requestTime time.Time) context.Context {
	return context.WithValue(ctx, RESP_REQUEST_TIME, requestTime)
}

// GetRequestTime 获取请求开始时间
func GetRequestTime(ctx context.Context) (time.Time, bool) {
	requestTime, ok := ctx.Value(RESP_REQUEST_TIME).(time.Time)
	return requestTime, ok
}
