package configs

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"strings"
)

var (
	useHeadless = true

	binPath = ""
	cdpURL  = ""
)

type StartupConfig struct {
	Headless bool
	BinPath  string
	Port     string
	CDPURL   string
}

func InitHeadless(h bool) {
	useHeadless = h
}

// IsHeadless 是否无头模式。
func IsHeadless() bool {
	return useHeadless
}

func SetBinPath(b string) {
	binPath = b
}

func GetBinPath() string {
	return binPath
}

func SetCdpURL(url string) {
	cdpURL = url
}

func GetCdpURL() string {
	return cdpURL
}

func IsCdpMode() bool {
	return len(strings.TrimSpace(cdpURL)) > 0
}

func newStartupFlagSet(output io.Writer) (*flag.FlagSet, *StartupConfig) {
	fs := flag.NewFlagSet("xhs-automation", flag.ContinueOnError)
	fs.SetOutput(output)

	cfg := &StartupConfig{}
	fs.BoolVar(&cfg.Headless, "headless", true, "是否无头模式")
	fs.StringVar(&cfg.BinPath, "bin", "", "浏览器二进制文件路径")
	fs.StringVar(&cfg.Port, "port", ":18060", "端口")
	fs.StringVar(&cfg.CDPURL, "cdp", "", "CDP websocket URL")

	return fs, cfg
}

func ParseStartupFlags(args []string) (StartupConfig, error) {
	fs, cfg := newStartupFlagSet(io.Discard)

	if err := fs.Parse(args); err != nil {
		return StartupConfig{}, err
	}
	if extraArgs := fs.Args(); len(extraArgs) > 0 {
		return StartupConfig{}, fmt.Errorf("unexpected positional args: %v", extraArgs)
	}

	return *cfg, nil
}

func StartupHelpText() string {
	var buf bytes.Buffer
	fs, _ := newStartupFlagSet(&buf)
	_, _ = fmt.Fprintf(&buf, "Usage of %s:\n", fs.Name())
	fs.PrintDefaults()
	return buf.String()
}
