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

	probe, err := checkLoginStatusOnPage(page, func() { settleCalls++ }, func(page loginStatusPage) (LoginStatusSignals, error) {
		exists, _, err := page.Has(loginStatusSelector)
		if err != nil {
			return LoginStatusSignals{}, err
		}
		return LoginStatusSignals{DOM: exists}, nil
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !probe.IsLoggedIn {
		t.Fatalf("expected loggedIn=true")
	}
	if probe.State != LoginStateLoggedIn {
		t.Fatalf("expected state=%s, got %s", LoginStateLoggedIn, probe.State)
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

	probe, err := checkLoginStatusOnPage(page, func() {}, func(loginStatusPage) (LoginStatusSignals, error) {
		return LoginStatusSignals{}, nil
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if probe.IsLoggedIn {
		t.Fatalf("expected loggedIn=false")
	}
	if err.Error() != "navigate explore failed: boom" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCheckLoginStatusOnPage_WrapsWaitLoadError(t *testing.T) {
	page := &fakeLoginStatusPage{waitLoadErr: errors.New("slow load")}

	probe, err := checkLoginStatusOnPage(page, func() {}, func(loginStatusPage) (LoginStatusSignals, error) {
		return LoginStatusSignals{}, nil
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if probe.IsLoggedIn {
		t.Fatalf("expected loggedIn=false")
	}
	if err.Error() != "wait explore load failed: slow load" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCheckLoginStatusOnPage_UsesSignalsWhenWaitLoadTimesOut(t *testing.T) {
	page := &fakeLoginStatusPage{waitLoadErr: errors.New("context deadline exceeded")}

	probe, err := checkLoginStatusOnPage(page, func() {}, func(loginStatusPage) (LoginStatusSignals, error) {
		return LoginStatusSignals{Cookie: true}, nil
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !probe.IsLoggedIn {
		t.Fatalf("expected loggedIn=true")
	}
	if probe.State != LoginStateLoggedIn {
		t.Fatalf("expected state=%s, got %s", LoginStateLoggedIn, probe.State)
	}
}

func TestDeriveLoginState_PrefersCookieSignal(t *testing.T) {
	state := deriveLoginState(LoginStatusSignals{
		Cookie:      true,
		DOM:         false,
		LoginPrompt: true,
	})

	if state != LoginStateLoggedIn {
		t.Fatalf("expected state=%s, got %s", LoginStateLoggedIn, state)
	}
}

func TestDeriveLoginState_UsesLoginPromptForLoggedOut(t *testing.T) {
	state := deriveLoginState(LoginStatusSignals{
		Cookie:      false,
		DOM:         false,
		LoginPrompt: true,
	})

	if state != LoginStateLoggedOut {
		t.Fatalf("expected state=%s, got %s", LoginStateLoggedOut, state)
	}
}

func TestDeriveLoginState_ReturnsUnknownWithoutStableSignals(t *testing.T) {
	state := deriveLoginState(LoginStatusSignals{})

	if state != LoginStateUnknown {
		t.Fatalf("expected state=%s, got %s", LoginStateUnknown, state)
	}
}

func TestCheckLoginStatusOnPage_ReturnsLoggedInWhenCookieSignalExists(t *testing.T) {
	page := &fakeLoginStatusPage{}

	probe, err := checkLoginStatusOnPage(page, func() {}, func(loginStatusPage) (LoginStatusSignals, error) {
		return LoginStatusSignals{Cookie: true}, nil
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !probe.IsLoggedIn {
		t.Fatalf("expected loggedIn=true")
	}
	if probe.State != LoginStateLoggedIn {
		t.Fatalf("expected state=%s, got %s", LoginStateLoggedIn, probe.State)
	}
	if !probe.Signals.Cookie {
		t.Fatalf("expected cookie signal to be preserved")
	}
}

func TestCheckLoginStatusOnPage_ReturnsUnknownWhenSignalsAreInconclusive(t *testing.T) {
	page := &fakeLoginStatusPage{}

	probe, err := checkLoginStatusOnPage(page, func() {}, func(loginStatusPage) (LoginStatusSignals, error) {
		return LoginStatusSignals{}, nil
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if probe.IsLoggedIn {
		t.Fatalf("expected loggedIn=false for unknown state")
	}
	if probe.State != LoginStateUnknown {
		t.Fatalf("expected state=%s, got %s", LoginStateUnknown, probe.State)
	}
}

func TestLoginPageSettleDelay_IsThreeSeconds(t *testing.T) {
	if loginPageSettleDelay != 3*time.Second {
		t.Fatalf("expected loginPageSettleDelay=3s, got %s", loginPageSettleDelay)
	}
}
