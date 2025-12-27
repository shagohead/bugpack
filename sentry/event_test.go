package sentry_test

import (
	"bytes"
	"compress/gzip"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path"
	"testing"
)

var saveGoldenFiles bool
var captureOutput bool

func TestMain(t *testing.M) {
	flag.BoolVar(&saveGoldenFiles, "save-golden", false, "Save golden files named received_event.json")
	flag.BoolVar(&captureOutput, "capture-output", false, "Capture subprocess STDOUT & STDERR")
	flag.Parse()
	os.Exit(t.Run())
}

func saveGoldenFile(t *testing.T, name string, b []byte) {
	t.Helper()
	f, err := os.OpenFile(path.Join("testdata", name), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.Write(b); err != nil {
		t.Fatal(err)
	}
}

// Get quoted value start and length.
func valueDimensions(b []byte, key string) (int, int) {
	start := bytes.Index(b, []byte(`"`+key+`":`))
	if start < 1 {
		return -1, -1
	}
	start += bytes.IndexRune(b[start:], ':') + 2 // Move to quoted value.
	return start, bytes.IndexRune(b[start:], '"')
}

var (
	placeholdEventID   = []byte(`e51b3c5ce3b34c7b898e4e830ef62432`)
	placeholdDSN       = []byte(`http://username@127.0.0.1:12345/1`)
	placeholdTimestamp = []byte(`2025-12-27T14:53:00Z`)
)

// Replace quoted value with placeholder.
func replaceValue(b []byte, key string, placeholder []byte) []byte {
	start, end := valueDimensions(b, key)
	if start < 1 {
		return b
	}
	return bytes.ReplaceAll(b, b[start:start+end], placeholder)
}

// Replace all values which changes every time.
func replaceAll(b []byte) []byte {
	defer func() {
		rec := recover()
		if rec != nil {
			fmt.Printf("input bytes: %s", b)
			panic(rec)
		}
	}()
	b = replaceValue(b, "event_id", placeholdEventID)
	b = replaceValue(b, "sent_at", placeholdTimestamp)
	b = replaceValue(b, "timestamp", placeholdTimestamp)
	b = replaceValue(b, "dsn", placeholdDSN)
	return b
}

// Test received data from instrumented command calls.
func TestCommands(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	// Serve client requests.
	serve := func(t *testing.T, received *[][]byte) string {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var body bytes.Buffer
			switch r.Header.Get("Content-Encoding") {
			case "gzip":
				zr, err := gzip.NewReader(r.Body)
				if err != nil {
					t.Fatal(err)
				}
				if _, err = io.Copy(&body, zr); err != nil {
					t.Fatal(err)
				}
			case "":
				if _, err = io.Copy(&body, r.Body); err != nil {
					t.Fatal(err)
				}
			}
			*received = append(*received, replaceAll(body.Bytes()))
		}))
		t.Cleanup(func() {
			server.Close()
		})
		return server.URL
	}

	// Platform-related subtest.
	type call struct {
		args []string
	}

	// Sets of platform tests.
	for _, platform := range []struct {
		path  string
		exec  string
		env   []string
		args  []string // Base args, which will prepends call's args.
		calls []call
	}{
		{
			path: "go",
			exec: "go",
			env: []string{
				fmt.Sprintf("HOME=%s", os.Getenv("HOME")),
				fmt.Sprintf("GOPATH=%s", os.Getenv("GOPATH")),
			},
			args: []string{"run", "./command"},
			calls: []call{
				{args: []string{"captureException"}},
				{args: []string{"captureMessage"}},
				{args: []string{"captureExceptionMany"}},
				{args: []string{"captureExceptionNested"}},
				{args: []string{"captureExceptionScoped"}},
				{args: []string{"captureExceptionWrapped"}},
				{args: []string{"panic"}},
			},
		},
		{
			path: "python",
			exec: ".venv/bin/python",
			args: []string{"-m", "main"},
			calls: []call{
				{args: []string{"capture_message"}},
				{args: []string{"division_by_zero"}},
				{args: []string{"custom_exception"}},
			},
		},
	} {
		for _, call := range platform.calls {
			lastarg := call.args[len(call.args)-1]
			t.Run(fmt.Sprintf("%s/%s", platform.path, lastarg), func(t *testing.T) {

				var received [][]byte
				serverURL := serve(t, &received)
				dsn, err := url.Parse(serverURL)
				if err != nil {
					t.Fatal(err)
				}
				dsn.User = url.User("username")
				dsn.Path = "/1"

				cmd := exec.Command(platform.exec, append(platform.args, call.args...)...)
				cmd.Env = append(platform.env, fmt.Sprintf("DSN=%s", dsn.String()))
				cmd.Dir = path.Join(wd, platform.path)
				if captureOutput {
					cmd.Stdout = os.Stderr
					cmd.Stderr = os.Stderr
				}
				if err := cmd.Run(); err != nil {
					t.Fatal(err)
				}
				if n := len(received); n == 0 {
					t.Fatal("received 0 requests")
				} else {
					t.Logf("received %d requests", n)
				}

				for i, b := range received {
					if saveGoldenFiles {
						name := fmt.Sprintf("%s_%s_%d.json", platform.path, lastarg, i)
						saveGoldenFile(t, name, b)
					} else {
						t.Logf("request %d: %s", i, b)
					}
				}

			})
		}
	}
}

//
// // Oneshot event data request receiver.
// // After handling first request it closes channel `received`.
// type mockReceiver struct {
// 	received    chan struct{}
// 	requestBody []byte
// }
//
// func (m *mockReceiver) ServeHTTP(w http.ResponseWriter, r *http.Request) {
// 	var err error
// 	m.requestBody, err = io.ReadAll(r.Body)
// 	if err != nil {
// 		panic(err)
// 	}
// 	w.WriteHeader(http.StatusOK)
// 	close(m.received)
// }
//
// // Test integration with HTTP servers.
// // All instrumented servers listen :8080.
// func TestServers(t *testing.T) {
// 	wd, err := os.Getwd()
// 	if err != nil {
// 		t.Fatal(err)
// 	}
//
// 	for _, tt := range []struct {
// 		path    string
// 		cmdName string
// 		cmdArgs []string
// 	}{
// 		{
// 			path:    "go",
// 			cmdName: "go",
// 			cmdArgs: []string{"run", "main.go"},
// 		},
// 		// {
// 		// 	path:    "python",
// 		// 	cmdName: "",
// 		// 	cmdArgs: []string{},
// 		// },
// 	} {
//
// 		t.Run(tt.path, func(t *testing.T) {
// 			receiver := &mockReceiver{received: make(chan struct{})}
// 			server := httptest.NewServer(receiver)
// 			t.Cleanup(func() {
// 				server.Close()
// 			})
//
// 			dsn, err := url.Parse(server.URL)
// 			if err != nil {
// 				t.Fatal(err)
// 			}
// 			dsn.User = url.User("username")
// 			dsn.Path = "/1"
//
// 			errg, ctx := errgroup.WithContext(context.Background())
//
// 			// Горутина процесса с тестовым клиентом.
// 			// Ее процесс должен быть остановлен по получению каких-либо данных ресивером.
// 			cmd := exec.Command(tt.cmdName, tt.cmdArgs...)
// 			// receiver.callback = func() {
// 			// 	cmd.Process.Signal(os.Interrupt)
// 			// }
// 			errg.Go(func() error {
// 				cmd.Env = []string{fmt.Sprintf("DSN=%s", dsn.String())}
// 				cmd.Dir = path.Join(wd, tt.path)
// 				cmd.Stdout = os.Stderr
// 				cmd.Stderr = os.Stderr
// 				return cmd.Run()
// 			})
//
// 			// Отправка запроса, который должен привести к исключению.
// 			errg.Go(func() error {
// 				time.Sleep(time.Second)
// 				// FIXME: Ожидание старта процесса с интеграцией (cmd)
// 				req, err := http.NewRequestWithContext(
// 					ctx, http.MethodGet, "http://localhost:8080", nil,
// 				)
// 				if err != nil {
// 					return err
// 				}
// 				resp, err := http.DefaultClient.Do(req)
// 				if err != nil {
// 					return err
// 				}
// 				defer resp.Body.Close()
// 				return nil
// 			})
// 			<-receiver.received
// 			errg.Wait()
// 			// Дождаться получения JSON события из интеграции.
// 			// Проверить возможность успешной валидации данных парсером.
// 		})
// 	}
// }
