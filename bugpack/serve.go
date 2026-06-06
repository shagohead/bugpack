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

	"github.com/ClickHouse/ch-go"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"

	"github.com/shagohead/bugpack/bugpack/batcher"
	"github.com/shagohead/bugpack/bugpack/batcher/chbuf"
	"github.com/shagohead/bugpack/bugpack/config"
	"github.com/shagohead/bugpack/bugpack/ingester"
)

func logger(cfg *config.Config) (*slog.Logger, error) {
	var h slog.Handler
	o := new(slog.HandlerOptions)
	o.Level = cfg.LogLevel
	switch cfg.LogFormat {
	case config.LogFormatJSON:
		h = slog.NewJSONHandler(os.Stderr, o)
	case config.LogFormatText:
		h = slog.NewTextHandler(os.Stderr, o)
	default:
		return nil, fmt.Errorf("unsupported log format %q", cfg.LogFormat)
	}
	l := slog.New(h)
	slog.SetDefault(l)
	return l, nil
}

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
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}
	log, err := logger(cfg)
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
	batcher := batcher.New(chbuf.Bufferer(chpool), cfg.Batcher)
	handler := &handler{
		healthPath: cfg.HealthPath,
		apiHandler: ingester.New[*chbuf.Envelope](cfg, batcher),
	}
	server := &http.Server{
		Addr:    cfg.ServerAddr,
		Handler: handler,
	}
	if cfg.ListenPrefix != "" {
		server.Handler = http.StripPrefix(cfg.ListenPrefix, server.Handler)
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
		log.InfoContext(ctx, "shutdown server")
		if err := server.Shutdown(ctx); err != nil {
			log.ErrorContext(ctx, err.Error())
		}
	}()

	log.InfoContext(ctx, "start listening", slog.String("addr", server.Addr))
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
