package xiaohongshu

import (
	"context"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/pkg/errors"
)

const (
	loginStatusExploreURL  = "https://www.xiaohongshu.com/explore"
	loginStatusSelector    = ".main-container .user .link-wrapper .channel"
	loginStatusAltSelector = ".main-container .user .link-wrapper"
	loginPromptSelector    = ".login-container .qrcode-img"
	loginPageSettleDelay   = 3 * time.Second
)

type LoginState string

const (
	LoginStateLoggedIn  LoginState = "logged_in"
	LoginStateLoggedOut LoginState = "logged_out"
	LoginStateUnknown   LoginState = "unknown"
)

type LoginStatusSignals struct {
	Cookie      bool `json:"cookie"`
	DOM         bool `json:"dom"`
	LoginPrompt bool `json:"login_prompt"`
}

type LoginStatusProbe struct {
	State      LoginState         `json:"state"`
	IsLoggedIn bool               `json:"is_logged_in"`
	Signals    LoginStatusSignals `json:"signals"`
}

type loginStatusPage interface {
	Navigate(url string) error
	WaitLoad() error
	Has(selector string) (bool, *rod.Element, error)
}

type loginStatusBrowserPage interface {
	loginStatusPage
	Browser() *rod.Browser
}

type loginStatusSignalDetector func(page loginStatusPage) (LoginStatusSignals, error)

type LoginAction struct {
	page *rod.Page
}

func NewLogin(page *rod.Page) *LoginAction {
	return &LoginAction{page: page}
}

func interpretLoginStatusResult(exists bool, err error) (bool, error) {
	if err != nil {
		return false, errors.Wrap(err, "check login status failed")
	}
	if !exists {
		return false, nil
	}
	return true, nil
}

func deriveLoginState(signals LoginStatusSignals) LoginState {
	if signals.Cookie || signals.DOM {
		return LoginStateLoggedIn
	}
	if signals.LoginPrompt {
		return LoginStateLoggedOut
	}
	return LoginStateUnknown
}

func buildLoginStatusProbe(signals LoginStatusSignals) LoginStatusProbe {
	state := deriveLoginState(signals)
	return LoginStatusProbe{
		State:      state,
		IsLoggedIn: state == LoginStateLoggedIn,
		Signals:    signals,
	}
}

func hasAnySelector(page loginStatusPage, selectors []string) (bool, error) {
	for _, selector := range selectors {
		exists, _, err := page.Has(selector)
		if err != nil {
			return false, err
		}
		if exists {
			return true, nil
		}
	}
	return false, nil
}

func hasLoginSessionCookie(page loginStatusPage) (bool, error) {
	browserPage, ok := page.(loginStatusBrowserPage)
	if !ok {
		return false, errors.New("browser cookie inspection unsupported")
	}

	cks, err := browserPage.Browser().GetCookies()
	if err != nil {
		return false, err
	}

	for _, ck := range cks {
		if ck == nil {
			continue
		}
		domain := strings.ToLower(ck.Domain)
		if !strings.Contains(domain, "xiaohongshu.com") {
			continue
		}
		switch ck.Name {
		case "web_session", "id_token", "a1":
			return true, nil
		}
	}
	return false, nil
}

func detectLoginStatusSignals(page loginStatusPage) (LoginStatusSignals, error) {
	cookie, err := hasLoginSessionCookie(page)
	if err != nil {
		return LoginStatusSignals{}, errors.Wrap(err, "inspect login cookies failed")
	}

	dom, err := hasAnySelector(page, []string{
		loginStatusSelector,
		loginStatusAltSelector,
	})
	if err != nil {
		return LoginStatusSignals{}, errors.Wrap(err, "check logged-in markers failed")
	}

	loginPrompt, err := hasAnySelector(page, []string{
		loginPromptSelector,
	})
	if err != nil {
		return LoginStatusSignals{}, errors.Wrap(err, "check login prompt markers failed")
	}

	return LoginStatusSignals{
		Cookie:      cookie,
		DOM:         dom,
		LoginPrompt: loginPrompt,
	}, nil
}

func checkLoginStatusOnPage(page loginStatusPage, settle func(), detector loginStatusSignalDetector) (LoginStatusProbe, error) {
	if err := page.Navigate(loginStatusExploreURL); err != nil {
		return LoginStatusProbe{}, errors.Wrap(err, "navigate explore failed")
	}
	waitLoadErr := page.WaitLoad()

	settle()

	signals, err := detector(page)
	if err != nil {
		if waitLoadErr != nil {
			return LoginStatusProbe{}, errors.Wrap(waitLoadErr, "wait explore load failed")
		}
		return LoginStatusProbe{}, errors.Wrap(err, "detect login status signals failed")
	}

	probe := buildLoginStatusProbe(signals)
	if waitLoadErr != nil && probe.State == LoginStateUnknown {
		return LoginStatusProbe{}, errors.Wrap(waitLoadErr, "wait explore load failed")
	}

	return probe, nil
}

func (a *LoginAction) CheckLoginStatus(ctx context.Context) (LoginStatusProbe, error) {
	pp := a.page.Timeout(25 * time.Second).Context(ctx)
	return checkLoginStatusOnPage(pp, func() {
		time.Sleep(1 * time.Second)
	}, detectLoginStatusSignals)
}

func (a *LoginAction) Login(ctx context.Context) error {
	pp := a.page.Context(ctx)

	// 导航到小红书首页，这会触发二维码弹窗
	pp.MustNavigate(loginStatusExploreURL).MustWaitLoad()

	// 等待一小段时间让页面完全加载
	time.Sleep(loginPageSettleDelay)

	// 检查是否已经登录
	probe, err := detectLoginStatusSignals(pp)
	if err == nil && deriveLoginState(probe) == LoginStateLoggedIn {
		// 已经登录，直接返回
		return nil
	}

	// 等待扫码成功提示或者登录完成
	// 这里我们等待登录成功的元素出现，这样更简单可靠
	pp.MustElement(loginStatusSelector)

	return nil
}

func (a *LoginAction) FetchQrcodeImage(ctx context.Context) (string, bool, error) {
	pp := a.page.Context(ctx)

	// 导航到小红书首页，这会触发二维码弹窗
	pp.MustNavigate(loginStatusExploreURL).MustWaitLoad()

	// 等待一小段时间让页面完全加载
	time.Sleep(loginPageSettleDelay)

	// 检查是否已经登录
	probe, err := detectLoginStatusSignals(pp)
	if err == nil && deriveLoginState(probe) == LoginStateLoggedIn {
		return "", true, nil
	}

	// 获取二维码图片
	src, err := pp.MustElement(".login-container .qrcode-img").Attribute("src")
	if err != nil {
		return "", false, errors.Wrap(err, "get qrcode src failed")
	}
	if src == nil || len(*src) == 0 {
		return "", false, errors.New("qrcode src is empty")
	}

	return *src, false, nil
}

func (a *LoginAction) WaitForLogin(ctx context.Context) bool {
	pp := a.page.Context(ctx)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return false
		case <-ticker.C:
			signals, err := detectLoginStatusSignals(pp)
			if err == nil && deriveLoginState(signals) == LoginStateLoggedIn {
				return true
			}
		}
	}
}
