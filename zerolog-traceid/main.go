package main

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/hlog"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/trace"
)

var logger zerolog.Logger

func requestLogger(next http.Handler) http.Handler {
	h := hlog.NewHandler(logger)
	accessHandler := hlog.AccessHandler(func(r *http.Request, status, size int, duration time.Duration) {
		hlog.FromRequest(r).Info().
			Str("method", r.Method).
			Stringer("url", r.URL).
			Int("status", status).
			Int("size", size).
			Dur("duration", duration).
			Msg("access log")
	})
	addr := hlog.RemoteAddrHandler("ip")
	userAgent := hlog.UserAgentHandler("user_agent")
	traceID := traceIDHandler("trace_id", "span_id")
	requestID := hlog.RequestIDHandler("req_id", "X-Request-Id")

	return h(addr(userAgent(traceID(requestID(accessHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	})))))))
}

func traceIDHandler(traceFieldKey, spanFieldKey string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			spanContext := trace.SpanFromContext(r.Context()).SpanContext()
			traceID := spanContext.TraceID().String()
			spanID := spanContext.SpanID().String()
			if traceID != "" {
				l := zerolog.Ctx(r.Context())
				l.UpdateContext(func(c zerolog.Context) zerolog.Context {
					return c.Str(traceFieldKey, traceID)
				})
			}
			if spanID != "" {
				l := zerolog.Ctx(r.Context())
				l.UpdateContext(func(c zerolog.Context) zerolog.Context {
					return c.Str(spanFieldKey, spanID)
				})
			}
			next.ServeHTTP(w, r)
		})
	}
}

func main() {
	logger = zerolog.New(os.Stderr).With().
		Timestamp().
		Logger()

	if err := run(); err != nil {
		log.Fatalln(err)
	}
}

func run() (err error) {
	// Handle SIGINT (CTRL+C) gracefully.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// Set up OpenTelemetry.
	otelShutdown, err := setupOTelSDK(ctx)
	if err != nil {
		return
	}
	// Handle shutdown properly so nothing leaks.
	defer func() {
		err = errors.Join(err, otelShutdown(context.Background()))
	}()

	// Start HTTP server.
	srv := &http.Server{
		Addr:         ":8888",
		BaseContext:  func(_ net.Listener) context.Context { return ctx },
		ReadTimeout:  time.Second,
		WriteTimeout: 10 * time.Second,
		Handler:      newHTTPHandler(),
	}
	srvErr := make(chan error, 1)
	go func() {
		srvErr <- srv.ListenAndServe()
	}()

	// Wait for interruption.
	select {
	case err = <-srvErr:
		// Error when starting HTTP server.
		return
	case <-ctx.Done():
		// Wait for first CTRL+C.
		// Stop receiving signal notifications as soon as possible.
		stop()
	}

	// When Shutdown is called, ListenAndServe immediately returns ErrServerClosed.
	err = srv.Shutdown(context.Background())
	return
}

func newHTTPHandler() http.Handler {
	mux := http.NewServeMux()

	// handleFunc is a replacement for mux.HandleFunc
	// which enriches the handler's HTTP instrumentation with the pattern as the http.route.
	handleFunc := func(pattern string, handlerFunc func(http.ResponseWriter, *http.Request)) {
		// Configure the "http.route" for the HTTP instrumentation.
		handler := otelhttp.WithRouteTag(pattern, http.HandlerFunc(handlerFunc))
		mux.Handle(pattern, handler)
	}

	// Register handlers.
	handleFunc("/rolldice", rolldice)

	// Add HTTP instrumentation for the whole server.
	handler := otelhttp.NewHandler(requestLogger(mux), "/")

	return handler
}
