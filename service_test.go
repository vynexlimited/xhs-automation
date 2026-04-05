package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/xpzouying/xiaohongshu-mcp/configs"
)

func TestGetCachedLoginStatus_MissingCookieFileShortCircuitsToLoggedOut(t *testing.T) {
	tmpDir := t.TempDir()
	cookiePath := filepath.Join(tmpDir, "cookies.json")

	status, ok := getCachedLoginStatus(cookiePath)
	if !ok {
		t.Fatalf("expected helper to short-circuit when cookie file is missing")
	}
	if status == nil {
		t.Fatalf("expected status")
	}
	if status.IsLoggedIn {
		t.Fatalf("expected logged out when cookie file is missing")
	}
}

func TestGetCachedLoginStatus_ExistingCookieFileDoesNotShortCircuit(t *testing.T) {
	tmpDir := t.TempDir()
	cookiePath := filepath.Join(tmpDir, "cookies.json")
	if err := os.WriteFile(cookiePath, []byte("[]"), 0o644); err != nil {
		t.Fatalf("write cookie file: %v", err)
	}

	status, ok := getCachedLoginStatus(cookiePath)
	if ok {
		t.Fatalf("expected browser probe path when cookie file exists, got %+v", status)
	}
}

func TestGetCachedLoginStatus_DoesNotShortCircuitInCdpMode(t *testing.T) {
	originalCdpURL := configs.GetCdpURL()
	t.Cleanup(func() {
		configs.SetCdpURL(originalCdpURL)
	})

	configs.SetCdpURL("ws://127.0.0.1:9292/devtools/browser/test")

	tmpDir := t.TempDir()
	cookiePath := filepath.Join(tmpDir, "cookies.json")

	status, ok := getCachedLoginStatus(cookiePath)
	if ok || status != nil {
		t.Fatalf("expected no cookie short-circuit in cdp mode, got status=%+v ok=%v", status, ok)
	}
}

func TestCdpBrowserCloseOnlyDisconnects(t *testing.T) {
	disconnected := false
	b := cdpBrowser{
		disconnect: func() error {
			disconnected = true
			return nil
		},
	}

	if err := b.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}
	if !disconnected {
		t.Fatalf("expected disconnect to be called")
	}
}
