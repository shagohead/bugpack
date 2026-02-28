package main

import (
	"context"
	"fmt"
	"os"

	"github.com/ClickHouse/ch-go"
	"github.com/ClickHouse/ch-go/chpool"
)

func newCHpool(ctx context.Context, opts *ch.Options) (*chpool.Pool, error) {
	for key, val := range map[string]*string{
		"CH_ADDRESS":  &(opts.Address),
		"CH_DATABASE": &(opts.Database),
		"CH_USER":     &(opts.User),
		"CH_PASSWORD": &(opts.Password),
	} {
		*val = os.Getenv(key)
		if *val == "" && key == "CH_ADDRESS" {
			return nil, fmt.Errorf("missing environment variable %s value", key)
		}
	}
	chpool, err := chpool.New(ctx, chpool.Options{ClientOptions: *opts})
	if err != nil {
		return nil, err
	}
	return chpool, chpool.Ping(ctx)
}
