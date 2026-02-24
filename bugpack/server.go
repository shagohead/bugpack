package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"

	"github.com/ClickHouse/ch-go"
	"github.com/ClickHouse/ch-go/chpool"
	"github.com/shagohead/bugpack/bugpack/batcher"
	"github.com/shagohead/bugpack/bugpack/batcher/chbuf"
	"github.com/shagohead/bugpack/bugpack/config"
	"github.com/shagohead/bugpack/bugpack/ingester"
)

func server(ctx context.Context, name string, args []string) error {
	fs := flag.NewFlagSet(name, flag.ExitOnError)
	cfile := fs.String("f", "", "Config file path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *cfile == "" {
		return errors.New("missing configuration file option value")
	}
	config, err := config.Load(*cfile)
	if err != nil {
		return err
	}
	// TODO: Config otel resource.
	opts := ch.Options{
		ClientName:                   "bugpack",
		OpenTelemetryInstrumentation: true,
		MeterProvider:                otel.GetMeterProvider(),
		TracerProvider:               otel.GetTracerProvider(),
	}
	for key, val := range map[string]*string{
		"CH_ADDRESS":  &(opts.Address),
		"CH_DATABASE": &(opts.Database),
		"CH_USER":     &(opts.User),
		"CH_PASSWORD": &(opts.Password),
	} {
		*val = os.Getenv(key)
		if *val == "" {
			return fmt.Errorf("missing environment variable %s value", key)
		}
	}
	chpool, err := chpool.New(ctx, chpool.Options{ClientOptions: opts})
	if err != nil {
		return err
	}
	factory := chbuf.Factory(chpool)
	batcher := batcher.New(factory, config.Batcher)
	handler := &handler{
		healthPath: config.HealthPath,
		apiHandler: ingester.New(config, batcher),
	}
	server := &http.Server{
		Addr:    config.ServerAddr,
		Handler: handler,
	}
	if config.ListenPrefix != "" {
		server.Handler = http.StripPrefix(config.ListenPrefix, server.Handler)
	}
	server.Handler = otelhttp.NewHandler(
		server.Handler, "IngestEnvelope",
		otelhttp.WithMessageEvents(
			otelhttp.ReadEvents, otelhttp.WriteEvents,
		),
	)
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt)
		<-sig
		if err := server.Shutdown(ctx); err != nil {
			slog.ErrorContext(ctx, err.Error())
		}
	}()
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}
	return nil
}

type handler struct {
	healthPath string
	apiHandler http.Handler
}

// ServeHTTP implements http.Handler.
func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if len(r.URL.Path) == len(h.healthPath) && r.URL.Path == h.healthPath {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	h.apiHandler.ServeHTTP(w, r)
}

var _ http.Handler = (*handler)(nil)
