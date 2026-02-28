package chmigrated

import (
	"context"
	"testing"

	"github.com/ClickHouse/ch-go"

	"github.com/shagohead/bugpack/bugpack/chconn"
	"github.com/shagohead/bugpack/chtest"
)

func Database(t testing.TB, name string) *ch.Client {
	ctx := context.Background()
	conn := chtest.Database(t, name)
	if err := chconn.Migrate(ctx, conn); err != nil {
		t.Fatal(err)
	}
	return conn
}
