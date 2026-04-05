package browser

import (
	"errors"
	"testing"

	"github.com/go-rod/rod"
	"github.com/xpzouying/xiaohongshu-mcp/configs"
)

func TestCurrentModeReturnsCDPWhenCdpURLConfigured(t *testing.T) {
	original := configs.GetCdpURL()
	t.Cleanup(func() {
		configs.SetCdpURL(original)
	})

	configs.SetCdpURL("ws://127.0.0.1:9222/devtools/browser/test")

	if got := CurrentMode(); got != "cdp" {
		t.Fatalf("expected cdp mode, got %q", got)
	}
}

func TestCurrentModeReturnsLegacyWhenCdpURLMissing(t *testing.T) {
	original := configs.GetCdpURL()
	t.Cleanup(func() {
		configs.SetCdpURL(original)
	})

	configs.SetCdpURL("")

	if got := CurrentMode(); got != "legacy" {
		t.Fatalf("expected legacy mode, got %q", got)
	}
}

func TestConnectCDPBrowserUsesProvidedURL(t *testing.T) {
	expectedURL := "ws://127.0.0.1:9222/devtools/browser/test"
	expectedBrowser := rod.New()

	var gotURL string
	gotBrowser, err := connectCDPBrowser(expectedURL, func(controlURL string) (*rod.Browser, error) {
		gotURL = controlURL
		return expectedBrowser, nil
	})
	if err != nil {
		t.Fatalf("connect cdp browser: %v", err)
	}
	if gotURL != expectedURL {
		t.Fatalf("unexpected control url: got %q", gotURL)
	}
	if gotBrowser != expectedBrowser {
		t.Fatalf("expected helper to return connected browser")
	}
}

func TestConnectCDPBrowserRejectsEmptyURL(t *testing.T) {
	called := false

	gotBrowser, err := connectCDPBrowser("", func(string) (*rod.Browser, error) {
		called = true
		return rod.New(), nil
	})
	if err == nil {
		t.Fatalf("expected empty control url to be rejected")
	}
	if called {
		t.Fatalf("expected connector not to be called for empty control url")
	}
	if gotBrowser != nil {
		t.Fatalf("expected nil browser when control url is empty")
	}
}

func TestConnectCDPBrowserPropagatesConnectorError(t *testing.T) {
	expectedErr := errors.New("connect failed")

	gotBrowser, err := connectCDPBrowser("ws://127.0.0.1:9222/devtools/browser/test", func(string) (*rod.Browser, error) {
		return nil, expectedErr
	})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected connector error, got %v", err)
	}
	if gotBrowser != nil {
		t.Fatalf("expected nil browser on connector error")
	}
}
