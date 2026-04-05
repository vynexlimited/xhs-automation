package browser

import (
	"errors"
	"net/url"
	"os"
	"strings"

	"github.com/go-rod/rod"
	"github.com/sirupsen/logrus"
	"github.com/xpzouying/headless_browser"
	"github.com/xpzouying/xiaohongshu-mcp/configs"
	"github.com/xpzouying/xiaohongshu-mcp/cookies"
)

const (
	modeCDP    = "cdp"
	modeLegacy = "legacy"
)

type browserConfig struct {
	binPath string
}

type Option func(*browserConfig)

func WithBinPath(binPath string) Option {
	return func(c *browserConfig) {
		c.binPath = binPath
	}
}

// maskProxyCredentials masks username and password in proxy URL for safe logging.
func maskProxyCredentials(proxyURL string) string {
	u, err := url.Parse(proxyURL)
	if err != nil || u.User == nil {
		return proxyURL
	}
	if _, hasPassword := u.User.Password(); hasPassword {
		u.User = url.UserPassword("***", "***")
	} else {
		u.User = url.User("***")
	}
	return u.String()
}

func CurrentMode() string {
	if configs.IsCdpMode() {
		return modeCDP
	}
	return modeLegacy
}

type cdpConnector func(controlURL string) (*rod.Browser, error)

// ConnectCDPBrowser attaches to an existing Chrome instance over CDP.
// It does not launch or manage a Chrome process.
func ConnectCDPBrowser() (*rod.Browser, error) {
	return connectCDPBrowser(configs.GetCdpURL(), defaultCDPConnector)
}

func connectCDPBrowser(controlURL string, connector cdpConnector) (*rod.Browser, error) {
	if strings.TrimSpace(controlURL) == "" {
		return nil, errors.New("cdp control url is required")
	}
	return connector(controlURL)
}

func defaultCDPConnector(controlURL string) (*rod.Browser, error) {
	browser := rod.New().ControlURL(controlURL)
	if err := browser.Connect(); err != nil {
		return nil, err
	}
	return browser, nil
}

func NewBrowser(headless bool, options ...Option) *headless_browser.Browser {
	cfg := &browserConfig{}
	for _, opt := range options {
		opt(cfg)
	}

	opts := []headless_browser.Option{
		headless_browser.WithHeadless(headless),
	}
	if cfg.binPath != "" {
		opts = append(opts, headless_browser.WithChromeBinPath(cfg.binPath))
	}

	// Read proxy from environment variable
	if proxy := os.Getenv("XHS_PROXY"); proxy != "" {
		opts = append(opts, headless_browser.WithProxy(proxy))
		logrus.Infof("Using proxy: %s", maskProxyCredentials(proxy))
	}

	// 加载 cookies
	cookiePath := cookies.GetCookiesFilePath()
	cookieLoader := cookies.NewLoadCookie(cookiePath)

	if data, err := cookieLoader.LoadCookies(); err == nil {
		opts = append(opts, headless_browser.WithCookies(string(data)))
		logrus.Debugf("loaded cookies from filesuccessfully")
	} else {
		logrus.Warnf("failed to load cookies: %v", err)
	}

	return headless_browser.New(opts...)
}
