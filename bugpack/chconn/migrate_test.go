package chconn

import (
	"context"
	"testing"

	"github.com/shagohead/bugpack/chtest"
)

func TestMigrate(t *testing.T) {
	ctx := context.Background()
	chconn := chtest.Database(t, "chconn_migrate")

	t.Log("Migrate on fresh database")
	if err := Migrate(ctx, chconn); err != nil {
		t.Fatal(err)
	}

	applied, err := Applied(ctx, chconn)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Applied migrations: %v", applied)

	t.Log("Second migration call")
	if err := Migrate(ctx, chconn); err != nil {
		t.Fatal(err)
	}
}
