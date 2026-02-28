package chconn

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"path/filepath"
	"slices"

	"github.com/ClickHouse/ch-go"
	"github.com/ClickHouse/ch-go/proto"
)

//go:embed migrations/*
var migrations embed.FS

const migrationsTable = `CREATE TABLE IF NOT EXISTS migrations (
	Applied DateTime DEFAULT now(),
	Name String
)
ENGINE = MergeTree
ORDER BY (Name);`

type Conn interface {
	Do(context.Context, ch.Query) error
}

func Migrate(ctx context.Context, conn Conn) error {
	dir := "migrations"
	dirs, err := migrations.ReadDir(dir)
	if err != nil {
		return err
	}
	if err := conn.Do(ctx, ch.Query{Body: migrationsTable}); err != nil {
		return err
	}
	saved, err := Applied(ctx, conn)
	if err != nil {
		return err
	}
	var names []string
	for _, e := range dirs {
		if slices.Contains(saved, e.Name()) {
			continue
		}
		names = append(names, e.Name())
	}
	slices.Sort(names)
	for _, name := range names {
		fmt.Printf("Migrate %s\n", name)
		if err := migrate(ctx, conn, filepath.Join(dir, name)); err != nil {
			return err
		}
		if err := save(ctx, conn, name); err != nil {
			return err
		}
	}
	return nil
}

func Applied(ctx context.Context, conn Conn) ([]string, error) {
	var saved []string
	var name proto.ColStr
	if err := conn.Do(ctx, ch.Query{
		Body:   "SELECT Name FROM migrations ORDER BY Name",
		Result: proto.Results{{Name: "Name", Data: &name}},
		OnResult: func(ctx context.Context, block proto.Block) error {
			for i := range block.Rows {
				saved = append(saved, name.Row(i))
			}
			return nil
		},
	}); err != nil {
		return nil, err
	}
	return saved, nil
}

func save(ctx context.Context, conn Conn, name string) error {
	var v proto.ColStr
	v.Append(name)
	return conn.Do(ctx, ch.Query{
		Body:  "INSERT INTO migrations (Name) VALUES",
		Input: proto.Input{{Name: "Name", Data: v}},
	})
}

func migrate(ctx context.Context, conn Conn, name string) error {
	b, err := migrations.ReadFile(name)
	if err != nil {
		return err
	}
	for {
		if len(b) == 0 {
			return nil
		}
		i := bytes.IndexRune(b, ';')
		if i == -1 {
			i = len(b) - 1
		}
		q := b[:i]
		b = b[i+1:]
		if len(q) == 0 {
			continue
		}
		q = bytes.Trim(q, "\n\t ")
		if err := conn.Do(ctx, ch.Query{Body: string(q)}); err != nil {
			return &MigrationError{File: name, Query: q, Err: err}
		}
	}
}

type MigrationError struct {
	File  string
	Query []byte
	Err   error
}

func (m *MigrationError) Error() string {
	return fmt.Sprintf(
		"failed %s with `%v` caused by query:\n%s",
		m.File, m.Err, m.Query,
	)
}

func (m *MigrationError) Unwrap() error {
	return m.Err
}
