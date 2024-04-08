package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/rs/xid"
)

type ctxKey string

const slogFields ctxKey = "slog_fields"

// AccessLogMiddleware creates http.Handler which logs http requests.
// It measures duration of request. It records response code and response size.
// It also adds correlation ID to the log entry.
func AccessLogMiddleware(f func(r *http.Request, status, size int, duration time.Duration)) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			lrw := newLoggingResponseWriter(w)

			correlationID := xid.New().String()

			ctx := AppendCtx(r.Context(), slog.String("correlation_id", correlationID))
			r = r.WithContext(ctx)

			w.Header().Add("X-Correlation-ID", correlationID)

			defer func() {
				f(r, lrw.statusCode, lrw.bytes, time.Since(start))
			}()

			next.ServeHTTP(lrw, r)
		})
	}
}

// CustomHeaderHandler adds header value to slog.Record with key fieldKey
func CustomHeaderHandler(fieldKey, header string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if v := r.Header.Get(header); v != "" {
				ctx := AppendCtx(r.Context(), slog.String(fieldKey, v))
				r = r.WithContext(ctx)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ContextHandler is used to log fields added to context.Context
type ContextHandler struct {
	slog.Handler
}

func (h ContextHandler) Handle(ctx context.Context, r slog.Record) error {
	if attrs, ok := ctx.Value(slogFields).([]slog.Attr); ok {
		for _, v := range attrs {
			r.AddAttrs(v)
		}
	}
	return h.Handler.Handle(ctx, r)
}

// AppendCtx returns a copy of context.Context with value named slogFields containing []slog.Attr needed for ContextHandler
func AppendCtx(parent context.Context, attr slog.Attr) context.Context {
	if parent == nil {
		parent = context.Background()
	}

	if v, ok := parent.Value(slogFields).([]slog.Attr); ok {
		v = append(v, attr)
		return context.WithValue(parent, slogFields, v)
	}
	v := []slog.Attr{}
	v = append(v, attr)
	return context.WithValue(parent, slogFields, v)
}

func getHost(hostPort string) string {
	if hostPort == "" {
		return ""
	}

	host, _, err := net.SplitHostPort(hostPort)
	if err != nil {
		return hostPort
	}
	return host
}

type loggingResponseWriter struct {
	http.ResponseWriter
	wroteHeader bool
	statusCode  int
	bytes       int
}

func newLoggingResponseWriter(w http.ResponseWriter) *loggingResponseWriter {
	return &loggingResponseWriter{ResponseWriter: w}
}

func (w *loggingResponseWriter) WriteHeader(statusCode int) {
	if !w.wroteHeader {
		w.statusCode = statusCode
		w.wroteHeader = true
		w.ResponseWriter.WriteHeader(statusCode)
	}
}

func (w *loggingResponseWriter) Write(buf []byte) (int, error) {
	w.WriteHeader(http.StatusOK)
	n, err := w.ResponseWriter.Write(buf)
	w.bytes += n
	return n, err
}

func main() {
	handler := &ContextHandler{slog.NewJSONHandler(os.Stdout, nil)}
	logger := slog.New(handler)
	slog.SetDefault(logger)

	mux := http.NewServeMux()
	mux.HandleFunc("/ping", pingHandler)

	addr := ":8888"
	slog.Info("starting listening", slog.String("addr", addr))

	alog := AccessLogMiddleware(func(r *http.Request, status, size int, duration time.Duration) {
		slog.InfoContext(
			r.Context(),
			"access log",
			slog.String("method", r.Method),
			slog.String("url", r.URL.RequestURI()),
			slog.String("user_agent", r.UserAgent()),
			slog.String("remote_addr", r.RemoteAddr),
			slog.String("remote_host", getHost(r.RemoteAddr)),
			slog.String("referer", r.Referer()),
			slog.String("proto", r.Proto),
			slog.Duration("took", duration),
			slog.Int("status_code", status),
			slog.Int("bytes", size),
		)
	})
	forwarded := CustomHeaderHandler("x-forwarded-for", "X-Forwarded-For")

	err := http.ListenAndServe(addr, forwarded(alog(mux)))
	if err != nil {
		slog.Error(err.Error())
	}
}

func pingHandler(w http.ResponseWriter, r *http.Request) {
	slog.InfoContext(r.Context(), "test")
	fmt.Fprintf(w, "pong")
}
