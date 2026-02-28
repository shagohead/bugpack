package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
)

func main() {
	ctx := context.Background()
	log := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	slog.SetDefault(log)
	if err := run(ctx); err != nil {
		log.LogAttrs(ctx, slog.LevelError, err.Error())
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	flag.Parse()
	var cmd func(context.Context, string, []string) error
	switch flag.Arg(0) {
	case "server":
		cmd = serve
	case "migrate":
		cmd = migrate
	case "":
		return fmt.Errorf("missing command argument")
	default:
		return fmt.Errorf("unknown command %s", flag.Arg(0))
	}
	return cmd(ctx, flag.Arg(0), flag.Args()[1:])
}
