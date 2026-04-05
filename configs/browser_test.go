package configs

import (
	"strings"
	"testing"
)

func TestBrowserConfigSetCdpURL(t *testing.T) {
	original := GetCdpURL()
	t.Cleanup(func() {
		SetCdpURL(original)
	})

	SetCdpURL("ws://127.0.0.1:9292/devtools/browser/test")

	if got := GetCdpURL(); got != "ws://127.0.0.1:9292/devtools/browser/test" {
		t.Fatalf("unexpected cdp url: got %q", got)
	}
	if !IsCdpMode() {
		t.Fatalf("expected cdp mode to be enabled")
	}
}

func TestBrowserConfigWhitespaceOnlyCdpDoesNotEnableMode(t *testing.T) {
	original := GetCdpURL()
	t.Cleanup(func() {
		SetCdpURL(original)
	})

	SetCdpURL("   \t\n  ")

	if got := GetCdpURL(); got != "   \t\n  " {
		t.Fatalf("unexpected cdp url: got %q", got)
	}
	if IsCdpMode() {
		t.Fatalf("expected whitespace-only cdp url to keep cdp mode disabled")
	}
}

func TestBrowserConfigResetCdpURLDisablesMode(t *testing.T) {
	original := GetCdpURL()
	t.Cleanup(func() {
		SetCdpURL(original)
	})

	SetCdpURL("ws://127.0.0.1:9292/devtools/browser/test")
	SetCdpURL("")

	if got := GetCdpURL(); got != "" {
		t.Fatalf("expected cdp url to reset, got %q", got)
	}
	if IsCdpMode() {
		t.Fatalf("expected cdp mode to be disabled after reset")
	}
}

func TestParseStartupFlags(t *testing.T) {
	cfg, err := ParseStartupFlags([]string{
		"--cdp", "ws://127.0.0.1:9292/devtools/browser/test",
		"--port", ":18060",
	})
	if err != nil {
		t.Fatalf("parse startup flags: %v", err)
	}

	if got := cfg.CDPURL; got != "ws://127.0.0.1:9292/devtools/browser/test" {
		t.Fatalf("unexpected cdp url: got %q", got)
	}
	if got := cfg.Port; got != ":18060" {
		t.Fatalf("unexpected port: got %q", got)
	}
}

func TestParseStartupFlagsDefaults(t *testing.T) {
	cfg, err := ParseStartupFlags(nil)
	if err != nil {
		t.Fatalf("parse startup flags: %v", err)
	}

	if !cfg.Headless {
		t.Fatalf("expected headless default true")
	}
	if got := cfg.BinPath; got != "" {
		t.Fatalf("unexpected default bin path: got %q", got)
	}
	if got := cfg.Port; got != ":18060" {
		t.Fatalf("unexpected default port: got %q", got)
	}
	if got := cfg.CDPURL; got != "" {
		t.Fatalf("unexpected default cdp url: got %q", got)
	}
}

func TestParseStartupFlagsRejectsPositionalArgs(t *testing.T) {
	_, err := ParseStartupFlags([]string{"--port", ":18060", "extra"})
	if err == nil {
		t.Fatalf("expected positional args to be rejected")
	}
	if !strings.Contains(err.Error(), "unexpected positional args") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStartupHelpTextIncludesCdpFlag(t *testing.T) {
	helpText := StartupHelpText()
	if !strings.Contains(helpText, "-cdp") {
		t.Fatalf("expected help text to advertise -cdp, got: %s", helpText)
	}
	if !strings.Contains(helpText, "-headless") {
		t.Fatalf("expected help text to advertise -headless, got: %s", helpText)
	}
}
