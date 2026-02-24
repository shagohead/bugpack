package ingester

import (
	"testing"
)

func Test_projectKey(t *testing.T) {
	for src, want := range map[string]string{
		"Sentry sentry_version=7, sentry_client=sentry.go/0.35.1, sentry_secret=projectpass, sentry_key=gokey":     "gokey",
		"Sentry sentry_version=7, sentry_client=sentry.go/0.35.1, sentry_key=gokey, sentry_secret=projectpass":     "gokey",
		"Sentry sentry_key=pykey, sentry_version=7, sentry_client=sentry.python/2.37.0, sentry_secret=projectpass": "pykey",
		"Entry sentry_key=pykey, sentry_version=7, sentry_client=sentry.python/2.37.0, sentry_secret=projectpass":  "",
		"Sentry sentry_key=, sentry_version=7, sentry_client=sentry.python/2.37.0, sentry_secret=projectpass":      "",
		"Sentry sentry_version=7, sentry_client=sentry.python/2.37.0, sentry_secret=projectpass":                   "",
		"Sentry sentry_key=,":       "",
		"Sentry sentry_key=":        "",
		"Sentry sentry_keys=gokey,": "",
		"":                          "",
	} {
		if got := projectKey(src); got != want {
			t.Errorf("projectKey(%q) = %q, want %q", src, got, want)
		}
	}
}
