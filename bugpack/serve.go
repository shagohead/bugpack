package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"

	"github.com/ClickHouse/ch-go"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"

	"github.com/shagohead/bugpack/bugpack/batcher"
	"github.com/shagohead/bugpack/bugpack/batcher/chbuf"
	"github.com/shagohead/bugpack/bugpack/config"
	"github.com/shagohead/bugpack/bugpack/ingester"
)

func serve(ctx context.Context, name string, args []string) error {
	var configPath string
	fs := flag.NewFlagSet(name, flag.ExitOnError)
	fs.StringVar(&configPath, "f", "", "Config file path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if configPath == "" {
		return errors.New("missing configuration file option value")
	}
	config, err := config.Load(configPath)
	if err != nil {
		return err
	}
	// TODO: Config optional otel resource.
	opts := ch.Options{
		ClientName:                   "bugpack",
		OpenTelemetryInstrumentation: true,
		MeterProvider:                otel.GetMeterProvider(),
		TracerProvider:               otel.GetTracerProvider(),
	}
	chpool, err := newCHpool(ctx, &opts)
	if err != nil {
		return err
	}
	batcher := batcher.New(chbuf.Bufferer(chpool), config.Batcher)
	handler := &handler{
		healthPath: config.HealthPath,
		apiHandler: ingester.New[*chbuf.Envelope](config, batcher),
	}
	server := &http.Server{
		Addr:    config.ServerAddr,
		Handler: handler,
	}
	if config.ListenPrefix != "" {
		server.Handler = http.StripPrefix(config.ListenPrefix, server.Handler)
	}
	server.Handler = otelhttp.NewHandler(
		server.Handler, "Ingest",
		otelhttp.WithMessageEvents(
			otelhttp.ReadEvents, otelhttp.WriteEvents,
		),
	)

	batcherErr := make(chan error, 1)
	go func() {
		batcherErr <- batcher.Serve()
	}()

	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt)
		<-sig
		slog.InfoContext(ctx, "Shutdown")
		if err := server.Shutdown(ctx); err != nil {
			slog.ErrorContext(ctx, err.Error())
		}
	}()

	slog.InfoContext(ctx, "Start listening", slog.String("addr", server.Addr))
	if e := server.ListenAndServe(); e != http.ErrServerClosed {
		err = errors.Join(err, e)
	}
	batcher.Shutdown()
	if e := <-batcherErr; e != nil {
		err = errors.Join(err, e)
	}
	return err
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
