package emulator

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestMain lets the test binary double as a fake app emulator. With
// EMULATOR_HELPER=serve it listens on --port and answers /admin/health; with
// EMULATOR_HELPER=exit it exits immediately. Flags are read by hand because the
// test binary would reject --port.
func TestMain(m *testing.M) {
	switch os.Getenv("EMULATOR_HELPER") {
	case "serve":
		port := 0
		for _, a := range os.Args[1:] {
			if v, ok := strings.CutPrefix(a, "--port="); ok {
				port, _ = strconv.Atoi(v)
			}
		}
		http.HandleFunc("/admin/health", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"status":"ok"}`))
		})
		fmt.Fprintln(os.Stderr, http.ListenAndServe(fmt.Sprintf("127.0.0.1:%d", port), nil))
		os.Exit(1)
	case "exit":
		os.Exit(3)
	}
	os.Exit(m.Run())
}

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func TestStartAndStop(t *testing.T) {
	spec := Spec{Name: "fake", Binary: os.Args[0], Port: freePort(t), Env: []string{"EMULATOR_HELPER=serve"}}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	p, err := Start(ctx, spec)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	resp, err := http.Get(spec.URL() + "/admin/health")
	if err != nil {
		t.Fatalf("health after Start: %v", err)
	}
	resp.Body.Close()

	p.Stop()
	if _, err := http.Get(spec.URL() + "/admin/health"); err == nil {
		t.Fatal("app emulator still answering after Stop")
	}
}

func TestStartFailsFastWhenProcessExits(t *testing.T) {
	spec := Spec{Name: "fake", Binary: os.Args[0], Port: freePort(t), Env: []string{"EMULATOR_HELPER=exit"}}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	start := time.Now()
	if _, err := Start(ctx, spec); err == nil {
		t.Fatal("expected an error when the process exits before becoming healthy")
	}
	if time.Since(start) > 5*time.Second {
		t.Fatalf("Start did not fail fast: %s", time.Since(start))
	}
}

func TestStartReportsMissingBinary(t *testing.T) {
	spec := Spec{Name: "fake", Binary: "/nonexistent/app-emulator", Port: freePort(t)}
	if _, err := Start(context.Background(), spec); err == nil {
		t.Fatal("expected an error for a missing binary")
	}
}
