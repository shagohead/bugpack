package chtest

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"testing"
	"time"

	"github.com/ClickHouse/ch-go"
)

func Database(t *testing.T, name string) *ch.Client {
	t.Helper()
	ctx := context.Background()
	once.Do(func() {
		var err error
		if addr = os.Getenv("CH_ADDRESS"); addr != "" {
			client = dial(t, ctx, "")
			return
		}
		docker, err := exec.LookPath("docker")
		if err != nil {
			return
		}
		addr, err = containerAddr(docker)
		if err != nil {
			if err := exec.Command(
				docker, "run", "-p", "9000", "-d", "--rm", "--name", containerName,
				"-e", "CLICKHOUSE_PASSWORD=default", "--ulimit", "nofile=262144:262144",
				"clickhouse/clickhouse-server:25.7-alpine",
			).Run(); err != nil {
				t.Fatal("docker run:", err)
			}
			time.Sleep(time.Second * 3)
			addr, err = containerAddr(docker)
			if err != nil {
				t.Fatal("container addr after docker run:", err)
			}
		}
		client = dial(t, ctx, "")
		if err := client.Ping(ctx); err != nil {
			t.Fatal("ping dockerized server:", err)
		}
	})
	if client == nil {
		t.Skip("ClickHouse server not available for integration tests")
	}
	if name == "" {
		return client
	}
	if err := client.Do(ctx, ch.Query{Body: fmt.Sprintf("DROP DATABASE IF EXISTS %s", name)}); err != nil {
		t.Fatalf("drop database %s if exists: %v", name, err)
	}
	if err := client.Do(ctx, ch.Query{Body: fmt.Sprintf("CREATE DATABASE %s", name)}); err != nil {
		t.Fatalf("create database %s: %v", name, err)
	}
	t.Cleanup(func() {
		if err := client.Do(ctx, ch.Query{Body: fmt.Sprintf("DROP DATABASE %s", name)}); err != nil {
			t.Fatalf("drop database %s: %v", name, err)
		}
	})
	return dial(t, ctx, name)
}

func dial(t *testing.T, ctx context.Context, db string) *ch.Client {
	conn, err := ch.Dial(ctx, ch.Options{Address: addr, Database: db, Password: "default"})
	if err != nil {
		t.Fatal("dial database:", err)
	}
	return conn
}

var (
	once   sync.Once
	addr   string
	client *ch.Client
)

const containerName = "bugpack-ch"

func containerAddr(docker string) (string, error) {
	out, err := exec.Command(docker, "port", containerName, "9000").Output()
	if err != nil {
		return "", err
	}
	if nl := bytes.Index(out, []byte("\n")); nl > 0 {
		out = out[:nl]
	}
	return string(out), nil
}
