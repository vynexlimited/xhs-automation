package main

import (
	"context"
	"fmt"
	"sync"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/cdp"
	"github.com/go-rod/stealth"
	"github.com/xpzouying/xiaohongshu-mcp/browser"
	"github.com/xpzouying/xiaohongshu-mcp/configs"
)

type pageRole string

const (
	pageRoleWork  pageRole = "work"
	pageRoleLogin pageRole = "login"
)

type pageLease struct {
	Page    *rod.Page
	release func(error)
}

func (l pageLease) Release(err error) {
	if l.release != nil {
		l.release(err)
	}
}

type pageController interface {
	Acquire(role pageRole) (pageLease, error)
	Close() error
}

type ephemeralPageController struct{}

func (c *ephemeralPageController) Acquire(_ pageRole) (pageLease, error) {
	b, err := newBrowser()
	if err != nil {
		return pageLease{}, err
	}
	page := b.NewPage()

	return pageLease{
		Page: page,
		release: func(_ error) {
			closePage(page)
			closeBrowser(b)
		},
	}, nil
}

func (c *ephemeralPageController) Close() error {
	return nil
}

type cdpPageControllerDeps struct {
	connect    func() (*rod.Browser, func() error, error)
	createPage func(*rod.Browser) (*rod.Page, error)
	closePage  func(*rod.Page) error
}

type cdpPageSlot struct {
	mu   sync.Mutex
	page *rod.Page
}

type cdpPageController struct {
	mu         sync.Mutex
	browser    *rod.Browser
	disconnect func() error
	slots      map[pageRole]*cdpPageSlot
	deps       cdpPageControllerDeps
}

func newPageController() pageController {
	if configs.IsCdpMode() {
		return newCDPPageController()
	}
	return &ephemeralPageController{}
}

func defaultConnectCDPBrowser() (*rod.Browser, func() error, error) {
	ws := &cdp.WebSocket{}
	if err := ws.Connect(context.Background(), configs.GetCdpURL(), nil); err != nil {
		return nil, nil, err
	}

	client := cdp.New().Start(ws)
	remoteBrowser := rod.New().Client(client)
	if err := remoteBrowser.Connect(); err != nil {
		_ = ws.Close()
		return nil, nil, err
	}

	return remoteBrowser, ws.Close, nil
}

func defaultCreateCDPPage(browser *rod.Browser) (page *rod.Page, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("create cdp page failed: %v", r)
		}
	}()
	return stealth.MustPage(browser), nil
}

func newCDPPageController() *cdpPageController {
	return newCDPPageControllerWithDeps(cdpPageControllerDeps{
		connect:    defaultConnectCDPBrowser,
		createPage: defaultCreateCDPPage,
		closePage: func(page *rod.Page) error {
			if page == nil {
				return nil
			}
			return page.Close()
		},
	})
}

func newCDPPageControllerWithDeps(deps cdpPageControllerDeps) *cdpPageController {
	return &cdpPageController{
		slots: map[pageRole]*cdpPageSlot{
			pageRoleWork:  {},
			pageRoleLogin: {},
		},
		deps: deps,
	}
}

func (c *cdpPageController) ensureBrowser() (*rod.Browser, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.browser != nil {
		return c.browser, nil
	}

	remoteBrowser, disconnect, err := c.deps.connect()
	if err != nil {
		return nil, err
	}
	c.browser = remoteBrowser
	c.disconnect = disconnect
	return c.browser, nil
}

func (c *cdpPageController) Acquire(role pageRole) (pageLease, error) {
	slot, ok := c.slots[role]
	if !ok {
		return pageLease{}, fmt.Errorf("unknown page role: %s", role)
	}

	slot.mu.Lock()

	remoteBrowser, err := c.ensureBrowser()
	if err != nil {
		slot.mu.Unlock()
		return pageLease{}, err
	}

	if slot.page == nil {
		slot.page, err = c.deps.createPage(remoteBrowser)
		if err != nil {
			slot.mu.Unlock()
			return pageLease{}, err
		}
	}

	page := slot.page
	return pageLease{
		Page: page,
		release: func(opErr error) {
			if opErr != nil && slot.page != nil {
				_ = c.deps.closePage(slot.page)
				slot.page = nil
			}
			slot.mu.Unlock()
		},
	}, nil
}

func (c *cdpPageController) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, slot := range c.slots {
		slot.mu.Lock()
		if slot.page != nil {
			_ = c.deps.closePage(slot.page)
			slot.page = nil
		}
		slot.mu.Unlock()
	}

	if c.disconnect != nil {
		err := c.disconnect()
		c.disconnect = nil
		c.browser = nil
		return err
	}

	return nil
}

func newBrowser() (browserSession, error) {
	if configs.IsCdpMode() {
		return newCDPBrowser(configs.GetCdpURL())
	}
	return legacyBrowser{
		browser: browser.NewBrowser(configs.IsHeadless(), browser.WithBinPath(configs.GetBinPath())),
	}, nil
}

func newCDPBrowser(controlURL string) (browserSession, error) {
	ws := &cdp.WebSocket{}
	if err := ws.Connect(context.Background(), controlURL, nil); err != nil {
		return nil, err
	}

	client := cdp.New().Start(ws)
	remoteBrowser := rod.New().Client(client)
	if err := remoteBrowser.Connect(); err != nil {
		_ = ws.Close()
		return nil, err
	}

	return cdpBrowser{
		browser:    remoteBrowser,
		disconnect: ws.Close,
	}, nil
}
