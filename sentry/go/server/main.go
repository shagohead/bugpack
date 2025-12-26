package main

import (
	"net/http"
	"os"
	"time"

	"github.com/getsentry/sentry-go"
	sentryhttp "github.com/getsentry/sentry-go/http"
)

func main() {
	if err := run(); err != nil {
		os.Stderr.WriteString(err.Error() + "\n")
		os.Exit(1)
	} else {
		os.Stderr.WriteString("server stopped\n")
	}
}

type handler struct{}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	panic("manual exception")
}

func run() error {
	if err := sentry.Init(sentry.ClientOptions{
		Dsn:         os.Getenv("DSN"),
		Debug:       true,
		Environment: "local",
	}); err != nil {
		return err
	}
	defer sentry.CurrentHub().Flush(time.Second)
	defer sentry.Recover()
	sentryHandler := sentryhttp.New(sentryhttp.Options{
		Repanic:         false,
		WaitForDelivery: true,
		Timeout:         time.Second,
	})
	server := &http.Server{
		Addr: ":8080", Handler: sentryHandler.Handle(&handler{}),
	}
	return server.ListenAndServe()
}
