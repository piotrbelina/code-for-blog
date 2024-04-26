package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptrace"
	"os"
	"strings"
	"sync"
	"time"
)

type LoggingTransport struct {
	rt                  http.RoundTripper
	logger              *slog.Logger
	detailedTiming      bool
	detailedTimingLevel slog.Level
}

func NewLoggingTransport(options ...Option) *LoggingTransport {
	t := &LoggingTransport{
		rt:             http.DefaultTransport,
		logger:         slog.Default(),
		detailedTiming: false,
	}

	for _, option := range options {
		option(t)
	}

	return t
}

type Option func(transport *LoggingTransport)

func WithRoundTripper(rt http.RoundTripper) Option {
	return func(t *LoggingTransport) {
		t.rt = rt
	}
}

func WithLogger(logger *slog.Logger) Option {
	return func(t *LoggingTransport) {
		t.logger = logger
	}
}

func WithDetailedTiming(level slog.Level) Option {
	return func(t *LoggingTransport) {
		t.detailedTiming = true
		t.detailedTimingLevel = level
	}
}

// RoundTrip logs the request & response data, if the detailed timing is set, it logs it as well
// code adopted from https://github.com/kubernetes/kubernetes/blob/master/staging/src/k8s.io/client-go/transport/round_trippers.go#L459
func (t *LoggingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	rCtx := r.Context()

	reqInfo := newRequestInfo(r)

	methodAttr := slog.String("method", reqInfo.RequestMethod)
	urlAttr := slog.String("url", reqInfo.RequestURL)
	t.logger.DebugContext(rCtx, "request info", methodAttr, urlAttr)

	var headers []any
	for key, values := range reqInfo.RequestHeaders {
		for _, value := range values {
			value = maskValue(key, value)
			headers = append(headers, slog.String(key, value))
		}
	}

	t.logger.DebugContext(rCtx, "request headers", headers...)

	startTime := time.Now()

	if t.detailedTiming {
		var getConn, dnsStart, dialStart, tlsStart, serverStart time.Time
		var host string
		trace := &httptrace.ClientTrace{
			// DNS
			DNSStart: func(info httptrace.DNSStartInfo) {
				reqInfo.muTrace.Lock()
				defer reqInfo.muTrace.Unlock()
				dnsStart = time.Now()
				host = info.Host
			},
			DNSDone: func(info httptrace.DNSDoneInfo) {
				reqInfo.muTrace.Lock()
				defer reqInfo.muTrace.Unlock()
				reqInfo.DNSLookup = time.Since(dnsStart)
				t.logger.Log(rCtx, t.detailedTimingLevel, "HTTP Trace", slog.String("DNS_lookup", host), slog.String("resolved", fmt.Sprintf("%v", info.Addrs)))
			},
			// Dial
			ConnectStart: func(network, addr string) {
				reqInfo.muTrace.Lock()
				defer reqInfo.muTrace.Unlock()
				dialStart = time.Now()
			},
			ConnectDone: func(network, addr string, err error) {
				reqInfo.muTrace.Lock()
				defer reqInfo.muTrace.Unlock()
				reqInfo.Dialing = time.Since(dialStart)
				if err != nil {
					t.logger.Log(rCtx, t.detailedTimingLevel, "HTTP Trace: Dial failed", slog.String("network", network), slog.String("addr", addr), slog.Any("error", err))
				} else {
					t.logger.Log(rCtx, t.detailedTimingLevel, "HTTP Trace: Dial succeed", slog.String("network", network), slog.String("addr", addr))
				}
			},
			// TLS
			TLSHandshakeStart: func() {
				tlsStart = time.Now()
			},
			TLSHandshakeDone: func(_ tls.ConnectionState, _ error) {
				reqInfo.muTrace.Lock()
				defer reqInfo.muTrace.Unlock()
				reqInfo.TLSHandshake = time.Since(tlsStart)
			},
			// Connection (it can be DNS + Dial or just the time to get one from the connection pool)
			GetConn: func(hostPort string) {
				getConn = time.Now()
			},
			GotConn: func(info httptrace.GotConnInfo) {
				reqInfo.muTrace.Lock()
				defer reqInfo.muTrace.Unlock()
				reqInfo.GetConnection = time.Since(getConn)
				reqInfo.ConnectionReused = info.Reused
			},
			// Server Processing (time since we wrote the request until first byte is received)
			WroteRequest: func(info httptrace.WroteRequestInfo) {
				reqInfo.muTrace.Lock()
				defer reqInfo.muTrace.Unlock()
				serverStart = time.Now()
			},
			GotFirstResponseByte: func() {
				reqInfo.muTrace.Lock()
				defer reqInfo.muTrace.Unlock()
				reqInfo.ServerProcessing = time.Since(serverStart)
			},
		}
		r = r.WithContext(httptrace.WithClientTrace(r.Context(), trace))
	}

	resp, err := t.rt.RoundTrip(r)
	reqInfo.Duration = time.Since(startTime)

	reqInfo.complete(resp, err)

	t.logger.InfoContext(rCtx, "response", methodAttr, urlAttr, slog.String("status", reqInfo.ResponseStatus), slog.Int64("Duration_ms", reqInfo.Duration.Nanoseconds()/int64(time.Millisecond)))

	if t.detailedTiming {
		var stats []slog.Attr
		if !reqInfo.ConnectionReused {
			stats = append(stats, slog.Int64("DNSLookup_ms", reqInfo.DNSLookup.Nanoseconds()/int64(time.Millisecond)))
			stats = append(stats, slog.Int64("Dial_ms", reqInfo.Dialing.Nanoseconds()/int64(time.Millisecond)))
			stats = append(stats, slog.Int64("TLSHandshake_ms", reqInfo.TLSHandshake.Nanoseconds()/int64(time.Millisecond)))
		} else {
			stats = append(stats, slog.Int64("GetConnection_ms", reqInfo.GetConnection.Nanoseconds()/int64(time.Millisecond)))
		}
		if reqInfo.ServerProcessing != 0 {
			stats = append(stats, slog.Int64("ServerProcessing_ms", reqInfo.ServerProcessing.Nanoseconds()/int64(time.Millisecond)))
		}
		stats = append(stats, slog.Int64("Duration_ms", reqInfo.Duration.Nanoseconds()/int64(time.Millisecond)))
		t.logger.LogAttrs(rCtx, t.detailedTimingLevel, "HTTP statistics", stats...)

		var responseHeaders []slog.Attr
		for key, values := range reqInfo.ResponseHeaders {
			for _, value := range values {
				value = maskValue(key, value)
				responseHeaders = append(responseHeaders, slog.String(key, value))
			}
		}
		t.logger.LogAttrs(rCtx, slog.LevelDebug, "response headers", responseHeaders...)
	}

	return resp, err
}

// requestInfo keeps track of information about a request/response combination
type requestInfo struct {
	RequestHeaders http.Header
	RequestMethod  string
	RequestURL     string

	ResponseStatus  string
	ResponseHeaders http.Header
	ResponseErr     error

	muTrace          sync.Mutex // Protect trace fields
	DNSLookup        time.Duration
	Dialing          time.Duration
	GetConnection    time.Duration
	TLSHandshake     time.Duration
	ServerProcessing time.Duration
	ConnectionReused bool

	Duration time.Duration
}

func newRequestInfo(r *http.Request) *requestInfo {
	return &requestInfo{
		RequestURL:     r.URL.String(),
		RequestMethod:  r.Method,
		RequestHeaders: r.Header,
	}
}

// complete adds information about the response to the requestInfo
func (r *requestInfo) complete(response *http.Response, err error) {
	if err != nil {
		r.ResponseErr = err
		return
	}
	r.ResponseStatus = response.Status
	r.ResponseHeaders = response.Header
}

// toCurl returns a string that can be run as a command in a terminal (minus the body)
func (r *requestInfo) toCurl() string {
	headers := ""
	for key, values := range r.RequestHeaders {
		for _, value := range values {
			value = maskValue(key, value)
			headers += fmt.Sprintf(` -H %q`, fmt.Sprintf("%s: %s", key, value))
		}
	}

	return fmt.Sprintf("curl -v -X%s %s '%s'", r.RequestMethod, headers, r.RequestURL)
}

var knownAuthTypes = map[string]bool{
	"bearer":    true,
	"basic":     true,
	"negotiate": true,
}

// maskValue masks credential content from authorization headers
// See https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Authorization
func maskValue(key string, value string) string {
	if !strings.EqualFold(key, "Authorization") {
		return value
	}
	if len(value) == 0 {
		return ""
	}
	var authType string
	if i := strings.Index(value, " "); i > 0 {
		authType = value[0:i]
	} else {
		authType = value
	}
	if !knownAuthTypes[strings.ToLower(authType)] {
		return "<masked>"
	}
	if len(value) > len(authType)+1 {
		value = authType + " <masked>"
	} else {
		value = authType
	}
	return value
}

const LevelTrace = slog.Level(-8)

var LevelNames = map[slog.Leveler]string{
	LevelTrace: "TRACE",
}

func main() {
	w := os.Stderr
	opts := &slog.HandlerOptions{
		// Level: slog.LevelDebug,
		Level: LevelTrace,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.LevelKey {
				level := a.Value.Any().(slog.Level)
				levelLabel, exists := LevelNames[level]
				if !exists {
					levelLabel = level.String()
				}

				a.Value = slog.StringValue(levelLabel)
			}

			return a
		},
	}
	// logger := slog.New(slog.NewJSONHandler(w, opts))
	logger := slog.New(slog.NewTextHandler(w, opts))
	slog.SetDefault(logger)

	ctx := context.Background()

	client := http.Client{Transport: NewLoggingTransport(WithLogger(logger), WithDetailedTiming(LevelTrace))}
	req, err := http.NewRequestWithContext(ctx, "GET", "https://httpbin.org/get", nil)
	if err != nil {
		slog.ErrorContext(ctx, "Error creating request", slog.Any("error", err))
		return
	}
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Authorization", "Bearer: XXX")

	resp, err := client.Do(req)
	resp, err = client.Do(req)
	if err != nil {
		slog.ErrorContext(ctx, "Error creating request", slog.Any("error", err))
		return
	}

	bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.ErrorContext(ctx, "Error reading body", slog.Any("error", err))
		return
	}

	fmt.Printf("body: %s\n", string(bytes))
}
