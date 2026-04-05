package xiaohongshu

import (
	"context"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

const (
	explorePageURL         = "https://www.xiaohongshu.com/explore"
	exploreAppSelector     = "div#app"
	profileSidebarSelector = "div.main-container li.user.side-bar-component a.link-wrapper span.channel"
)

type navigateElement interface {
	Click(button proto.InputMouseButton, clickCount int) error
}

type navigatePage interface {
	Navigate(url string) error
	WaitLoad() error
	WaitStable() error
	Element(selector string) (navigateElement, error)
}

type rodNavigatePage struct {
	page *rod.Page
}

func (r rodNavigatePage) Navigate(url string) error {
	return r.page.Navigate(url)
}

func (r rodNavigatePage) WaitLoad() error {
	return r.page.WaitLoad()
}

func (r rodNavigatePage) WaitStable() error {
	return r.page.WaitStable(2 * time.Second)
}

func (r rodNavigatePage) Element(selector string) (navigateElement, error) {
	return r.page.Element(selector)
}

type NavigateAction struct {
	page *rod.Page
}

func NewNavigate(page *rod.Page) *NavigateAction {
	return &NavigateAction{page: page}
}

func navigateToExplorePage(page navigatePage) error {
	if err := page.Navigate(explorePageURL); err != nil {
		return wrapNavigateError("navigate explore failed", err)
	}
	if err := page.WaitLoad(); err != nil {
		return wrapNavigateError("wait explore load failed", err)
	}
	if _, err := page.Element(exploreAppSelector); err != nil {
		return wrapNavigateError("locate explore app container failed", err)
	}
	return nil
}

func wrapNavigateError(message string, err error) error {
	if err == nil {
		return nil
	}
	return &NavigateError{message: message, cause: err}
}

type NavigateError struct {
	message string
	cause   error
}

func (e *NavigateError) Error() string {
	return e.message + ": " + e.cause.Error()
}

func (e *NavigateError) Unwrap() error {
	return e.cause
}

func navigateToProfilePage(page navigatePage) error {
	if err := navigateToExplorePage(page); err != nil {
		return err
	}
	if err := page.WaitStable(); err != nil {
		return wrapNavigateError("wait explore stable failed", err)
	}

	profileLink, err := page.Element(profileSidebarSelector)
	if err != nil {
		return wrapNavigateError("locate profile sidebar link failed", err)
	}
	if err := profileLink.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return wrapNavigateError("click profile sidebar link failed", err)
	}
	if err := page.WaitLoad(); err != nil {
		return wrapNavigateError("wait profile page load failed", err)
	}

	return nil
}

func (n *NavigateAction) ToExplorePage(ctx context.Context) error {
	page := rodNavigatePage{page: n.page.Context(ctx)}
	return navigateToExplorePage(page)
}

func (n *NavigateAction) ToProfilePage(ctx context.Context) error {
	page := rodNavigatePage{page: n.page.Context(ctx)}
	return navigateToProfilePage(page)
}
