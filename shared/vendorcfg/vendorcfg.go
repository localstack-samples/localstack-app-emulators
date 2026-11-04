// Package vendorcfg is the one place each vendor's base URL is read.
//
// Every demo points its vendor SDK at an app emulator through a base URL. The
// default is the real vendor host, never a local port. Destructive tooling such
// as reset must use Require, so it cannot fall back to a port another demo is
// using.
//
// The package knows no vendor by name. A demo passes the vendors it uses.
package vendorcfg

import (
	"fmt"
	"strings"
)

// URLs maps a vendor name, as the demo spelled it, to its base URL. An empty
// value means the real vendor host.
type URLs map[string]string

// EnvName is the environment variable for a vendor: "stripe" gives
// STRIPE_BASE_URL, and "logo-dev" gives LOGO_DEV_BASE_URL.
func EnvName(vendor string) string { return envPrefix(vendor) + "_BASE_URL" }

// EmulatorEnvName is the variable that says where a vendor's app emulator runs
// when you run it yourself: "plaid" gives PLAID_EMULATOR_URL. Unset means
// LocalStack hosts it.
func EmulatorEnvName(vendor string) string { return envPrefix(vendor) + "_EMULATOR_URL" }

func envPrefix(vendor string) string {
	return strings.ToUpper(strings.ReplaceAll(vendor, "-", "_"))
}

// FromEnv reads the base URL of each vendor through getenv, which is
// os.Getenv in production. A trailing slash is trimmed.
func FromEnv(getenv func(string) string, vendors ...string) URLs {
	urls := make(URLs, len(vendors))
	for _, v := range vendors {
		urls[v] = strings.TrimRight(getenv(EnvName(v)), "/")
	}
	return urls
}

// Require returns value, or an error naming the vendor when it is empty. Use it
// for any operation that writes to or wipes an app emulator.
func Require(vendor, value string) (string, error) {
	if value == "" {
		return "", fmt.Errorf("%s URL is required for this operation and has no default (set %s)", vendor, EnvName(vendor))
	}
	return value, nil
}
