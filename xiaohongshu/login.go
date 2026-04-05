package xiaohongshu

import (
	"context"
	"time"

	"github.com/go-rod/rod"
	"github.com/pkg/errors"
)

const (
	loginStatusExploreURL = "https://www.xiaohongshu.com/explore"
	loginStatusSelector   = ".main-container .user .link-wrapper .channel"
	loginPageSettleDelay  = 3 * time.Second
)

type loginStatusPage interface {
	Navigate(url string) error
	WaitLoad() error
	Has(selector string) (bool, *rod.Element, error)
}

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

func checkLoginStatusOnPage(page loginStatusPage, settle func()) (bool, error) {
	if err := page.Navigate(loginStatusExploreURL); err != nil {
		return false, errors.Wrap(err, "navigate explore failed")
	}
	if err := page.WaitLoad(); err != nil {
		return false, errors.Wrap(err, "wait explore load failed")
	}

	settle()

	exists, _, err := page.Has(loginStatusSelector)
	return interpretLoginStatusResult(exists, err)
}

func (a *LoginAction) CheckLoginStatus(ctx context.Context) (bool, error) {
	pp := a.page.Timeout(25 * time.Second).Context(ctx)
	return checkLoginStatusOnPage(pp, func() {
		time.Sleep(1 * time.Second)
	})
}

func (a *LoginAction) Login(ctx context.Context) error {
	pp := a.page.Context(ctx)

	// 导航到小红书首页，这会触发二维码弹窗
	pp.MustNavigate(loginStatusExploreURL).MustWaitLoad()

	// 等待一小段时间让页面完全加载
	time.Sleep(loginPageSettleDelay)

	// 检查是否已经登录
	if exists, _, _ := pp.Has(loginStatusSelector); exists {
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
	if exists, _, _ := pp.Has(loginStatusSelector); exists {
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
			el, err := pp.Element(loginStatusSelector)
			if err == nil && el != nil {
				return true
			}
		}
	}
}
