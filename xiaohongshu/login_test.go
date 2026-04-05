package xiaohongshu

import (
	"errors"
	"testing"
	"time"

	"github.com/go-rod/rod"
)

type fakeLoginStatusPage struct {
	navigateErr error
	waitLoadErr error
	exists      bool
	hasErr      error
	navigatedTo string
	waited      bool
	selector    string
}

func (f *fakeLoginStatusPage) Navigate(url string) error {
	f.navigatedTo = url
	return f.navigateErr
}

func (f *fakeLoginStatusPage) WaitLoad() error {
	f.waited = true
	return f.waitLoadErr
}

func (f *fakeLoginStatusPage) Has(selector string) (bool, *rod.Element, error) {
	f.selector = selector
	return f.exists, nil, f.hasErr
}

func TestInterpretLoginStatusResult_NotLoggedInIsNotAnError(t *testing.T) {
	loggedIn, err := interpretLoginStatusResult(false, nil)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if loggedIn {
		t.Fatalf("expected loggedIn=false")
	}
}

func TestInterpretLoginStatusResult_ProbeErrorStillFails(t *testing.T) {
	probeErr := errors.New("probe failed")
	loggedIn, err := interpretLoginStatusResult(false, probeErr)
	if err == nil {
		t.Fatalf("expected error")
	}
	if loggedIn {
		t.Fatalf("expected loggedIn=false")
	}
}

func TestCheckLoginStatusOnPage_ReturnsLoggedInWhenSelectorExists(t *testing.T) {
	page := &fakeLoginStatusPage{exists: true}
	settleCalls := 0

	loggedIn, err := checkLoginStatusOnPage(page, func() { settleCalls++ })
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !loggedIn {
		t.Fatalf("expected loggedIn=true")
	}
	if page.navigatedTo != loginStatusExploreURL {
		t.Fatalf("expected navigate to %s, got %s", loginStatusExploreURL, page.navigatedTo)
	}
	if !page.waited {
		t.Fatalf("expected WaitLoad to be called")
	}
	if page.selector != loginStatusSelector {
		t.Fatalf("expected selector %s, got %s", loginStatusSelector, page.selector)
	}
	if settleCalls != 1 {
		t.Fatalf("expected settle to be called once, got %d", settleCalls)
	}
}

func TestCheckLoginStatusOnPage_WrapsNavigateError(t *testing.T) {
	page := &fakeLoginStatusPage{navigateErr: errors.New("boom")}

	loggedIn, err := checkLoginStatusOnPage(page, func() {})
	if err == nil {
		t.Fatalf("expected error")
	}
	if loggedIn {
		t.Fatalf("expected loggedIn=false")
	}
	if err.Error() != "navigate explore failed: boom" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCheckLoginStatusOnPage_WrapsWaitLoadError(t *testing.T) {
	page := &fakeLoginStatusPage{waitLoadErr: errors.New("slow load")}

	loggedIn, err := checkLoginStatusOnPage(page, func() {})
	if err == nil {
		t.Fatalf("expected error")
	}
	if loggedIn {
		t.Fatalf("expected loggedIn=false")
	}
	if err.Error() != "wait explore load failed: slow load" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoginPageSettleDelay_IsThreeSeconds(t *testing.T) {
	if loginPageSettleDelay != 3*time.Second {
		t.Fatalf("expected loginPageSettleDelay=3s, got %s", loginPageSettleDelay)
	}
}
