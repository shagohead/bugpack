package main

import (
	"context"

	"github.com/ClickHouse/ch-go"

	"github.com/shagohead/bugpack/bugpack/chconn"
)

func migrate(ctx context.Context, name string, args []string) error {
	opts := ch.Options{ClientName: "bugpack-migrate"}
	chpool, err := newCHpool(ctx, &opts)
	if err != nil {
		return err
	}
	return chconn.Migrate(ctx, chpool)
}
