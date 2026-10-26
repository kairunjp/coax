//go:build linux && cgo

package b25

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func mockSharedLibrary(t *testing.T) string {
	t.Helper()
	library := filepath.Join(t.TempDir(), "libaribb25-test.so")
	cmd := exec.Command("cc", "-shared", "-fPIC", "-o", library, "testdata/mock.c")
	if result, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build B25 mock: %v: %s", err, result)
	}
	return library
}

func TestDecodeFlushAndClose(t *testing.T) {
	path := mockSharedLibrary(t)
	var output bytes.Buffer
	decoder, err := newWithPath(&output, path)
	if err != nil {
		t.Fatal(err)
	}
	if n, err := decoder.Write([]byte("abc")); n != 3 || err != nil {
		t.Fatalf("Write = (%d, %v)", n, err)
	}
	if got := output.String(); got != "bcd" {
		t.Fatalf("decoded = %q", got)
	}
	if err := decoder.Close(); err != nil {
		t.Fatal(err)
	}
	if got := output.String(); got != "bcd!" {
		t.Fatalf("decoded + flush = %q", got)
	}
	if err := decoder.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := decoder.Write([]byte("abc")); err == nil {
		t.Fatal("Write after Close should fail")
	}
}

func TestDecoderInitializationFailure(t *testing.T) {
	path := mockSharedLibrary(t)
	t.Setenv("MOCK_B25_CARD_FAIL", "1")
	_, err := newWithPath(&bytes.Buffer{}, path)
	if err == nil || !strings.Contains(err.Error(), "code -42") {
		t.Fatalf("wanted card init error: %v", err)
	}
}

func TestLibraryMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.so")
	if _, err := os.Stat(path); err == nil {
		t.Fatal("expected missing library")
	}
	if _, err := newWithPath(&bytes.Buffer{}, path); err == nil || !strings.Contains(err.Error(), "missing.so") {
		t.Fatalf("wanted library loading error, got %v", err)
	}
}
