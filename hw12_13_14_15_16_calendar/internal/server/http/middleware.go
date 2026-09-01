// Package internalhttp provides the calendar HTTP transport and access logging.
package internalhttp

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

const unknownClientIP = "-"

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(body)
}

func (w *statusWriter) statusCode() int {
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
}

func loggingMiddleware(logger Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startedAt := time.Now()
		writer := &statusWriter{ResponseWriter: w}

		defer func() {
			if logger == nil {
				return
			}

			requestURI := r.RequestURI
			if requestURI == "" {
				requestURI = r.URL.RequestURI()
			}

			message := fmt.Sprintf(
				"%s [%s] %s %s %s %d latency=%s",
				clientIP(r.RemoteAddr),
				startedAt.Format("02/Jan/2006:15:04:05 -0700"),
				sanitizeLogField(r.Method),
				sanitizeLogField(requestURI),
				sanitizeLogField(r.Proto),
				writer.statusCode(),
				time.Since(startedAt).Round(time.Microsecond),
			)
			if userAgent := r.UserAgent(); userAgent != "" {
				message += fmt.Sprintf(" %q", userAgent)
			}

			logger.Info(message)
		}()

		next.ServeHTTP(writer, r)
	})
}

func clientIP(remoteAddress string) string {
	if remoteAddress == "" {
		return unknownClientIP
	}

	host, _, err := net.SplitHostPort(remoteAddress)
	if err == nil {
		return host
	}
	return sanitizeLogField(remoteAddress)
}

func sanitizeLogField(value string) string {
	return strings.NewReplacer("\r", `\r`, "\n", `\n`).Replace(value)
}
