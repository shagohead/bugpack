package sentry_test

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
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

	"github.com/go-faster/jx"

	"github.com/shagohead/bugpack/bugpack/envelope"
)

var (
	saveJSON      bool
	logRequest    bool
	captureOutput bool
)

func TestMain(t *testing.M) {
	flag.BoolVar(&saveJSON, "save-json", false, "Save events JSON data in testdata dir")
	flag.BoolVar(&logRequest, "log-request", false, "Log requests bodies")
	flag.BoolVar(&captureOutput, "capture-output", false, "Capture subprocess STDOUT & STDERR")
	flag.Parse()
	os.Exit(t.Run())
}

// Test received data from instrumented command calls.
func TestIntegration(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	// Serve client requests.
	serve := func(t *testing.T, received *[][]byte, logURL bool) string {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if logURL {
				t.Logf("RequestURI: %s", r.RequestURI)
				t.Logf("X-Sentry-Auth: %s", r.Header.Get("X-Sentry-Auth"))
			}
			var body bytes.Buffer
			switch r.Header.Get("Content-Encoding") {
			case "gzip":
				zr, err := gzip.NewReader(r.Body)
				if err != nil {
					t.Fatal(err)
				}
				defer zr.Close()
				if _, err = io.Copy(&body, zr); err != nil {
					t.Fatal(err)
				}
			case "":
				if _, err = io.Copy(&body, r.Body); err != nil {
					t.Fatal(err)
				}
			default:
				t.Fatalf("Unexpected encoding: %s", r.Header.Get("Content-Encoding"))
			}
			*received = append(*received, body.Bytes())
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

	dec := jx.GetDecoder()

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
			args: []string{"run", "./"},
			calls: []call{
				{args: []string{"captureException"}},
				{args: []string{"captureMessage"}},
				{args: []string{"captureExceptionMany"}},
				{args: []string{"captureExceptionNested"}},
				{args: []string{"captureExceptionScoped"}},
				{args: []string{"captureExceptionWrapped"}},
				{args: []string{"panic"}},
				{args: []string{"withRequest"}},
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
				{args: []string{"with_breadcrumbs"}},
				{args: []string{"raise_new_during_except"}},
				{args: []string{"raise_same_during_except"}},
				{args: []string{"raise_same_during_capture"}},
			},
		},
	} {
		for idx, call := range platform.calls {
			lastarg := call.args[len(call.args)-1]
			t.Run(fmt.Sprintf("%s/%s", platform.path, lastarg), func(t *testing.T) {

				var received [][]byte
				serverURL := serve(t, &received, idx == 0)
				dsn, err := url.Parse(serverURL)
				if err != nil {
					t.Fatal(err)
				}
				dsn.User = url.UserPassword("projectuser", "projectpass")
				dsn.Path = "/prefix/0"
				if idx == 0 {
					t.Logf("DSN=%s", dsn.String())
				}

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
				} else if n > 1 {
					t.Logf("received %d requests", n)
				}

				for i, b := range received {
					if saveJSON {
						name := path.Join("testdata", fmt.Sprintf("%s_%s_%d.json", platform.path, lastarg, i))
						file, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
						if err != nil {
							t.Fatal(err)
						}
						var o bytes.Buffer
						for start := 0; start < len(b); {
							end := bytes.Index(b[start:], []byte("}\n"))
							if end < 1 {
								break
							}
							end += start + 2
							if err = json.Indent(&o, b[start:end], "", "  "); err != nil {
								t.Fatalf("indent [%d:%d] = %v", start, end, err)
							}
							start = end
						}
						if _, err = o.WriteTo(file); err != nil {
							t.Fatal(err)
						}
					} else {
						if logRequest {
							t.Logf("request %d: %s", i, b)
						}
					}

					dec.ResetBytes(b)
					env := new(envelope.Envelope)
					if err = env.Decode(dec); err != nil {
						t.Fatal(err)
					}
				}
			})
		}
	}
}
