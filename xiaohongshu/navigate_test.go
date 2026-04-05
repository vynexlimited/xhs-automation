package xiaohongshu

import (
	"errors"
	"testing"

	"github.com/go-rod/rod/lib/proto"
)

type fakeNavigateElement struct {
	clickErr error
	clicked  bool
}

func (f *fakeNavigateElement) Click(_ proto.InputMouseButton, _ int) error {
	f.clicked = true
	return f.clickErr
}

type fakeNavigatePage struct {
	navigateErr   error
	waitLoadErr   error
	waitStableErr error
	appErr        error
	profileErr    error
	profileEl     *fakeNavigateElement
	navigatedTo   []string
	elementCalls  []string
}

func (f *fakeNavigatePage) Navigate(url string) error {
	f.navigatedTo = append(f.navigatedTo, url)
	return f.navigateErr
}

func (f *fakeNavigatePage) WaitLoad() error {
	return f.waitLoadErr
}

func (f *fakeNavigatePage) WaitStable() error {
	return f.waitStableErr
}

func (f *fakeNavigatePage) Element(selector string) (navigateElement, error) {
	f.elementCalls = append(f.elementCalls, selector)
	if selector == exploreAppSelector {
		if f.appErr != nil {
			return nil, f.appErr
		}
		return &fakeNavigateElement{}, nil
	}
	if selector == profileSidebarSelector {
		if f.profileErr != nil {
			return nil, f.profileErr
		}
		if f.profileEl == nil {
			f.profileEl = &fakeNavigateElement{}
		}
		return f.profileEl, nil
	}
	return nil, errors.New("unexpected selector")
}

func TestNavigateToExplorePage_WrapsNavigateError(t *testing.T) {
	page := &fakeNavigatePage{navigateErr: errors.New("boom")}

	err := navigateToExplorePage(page)
	if err == nil {
		t.Fatalf("expected error")
	}
	if err.Error() != "navigate explore failed: boom" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNavigateToExplorePage_WrapsAppLookupError(t *testing.T) {
	page := &fakeNavigatePage{appErr: errors.New("missing app")}

	err := navigateToExplorePage(page)
	if err == nil {
		t.Fatalf("expected error")
	}
	if err.Error() != "locate explore app container failed: missing app" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNavigateToProfilePage_WrapsProfileClickError(t *testing.T) {
	page := &fakeNavigatePage{
		profileEl: &fakeNavigateElement{clickErr: errors.New("click failed")},
	}

	err := navigateToProfilePage(page)
	if err == nil {
		t.Fatalf("expected error")
	}
	if err.Error() != "click profile sidebar link failed: click failed" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNavigateToProfilePage_ClicksSidebarAndLoads(t *testing.T) {
	page := &fakeNavigatePage{}

	err := navigateToProfilePage(page)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(page.navigatedTo) != 1 || page.navigatedTo[0] != explorePageURL {
		t.Fatalf("unexpected navigations: %#v", page.navigatedTo)
	}
	if page.profileEl == nil || !page.profileEl.clicked {
		t.Fatalf("expected profile sidebar to be clicked")
	}
}
