package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/stealth"
	"github.com/sirupsen/logrus"
	"github.com/xpzouying/headless_browser"
	"github.com/xpzouying/xiaohongshu-mcp/configs"
	"github.com/xpzouying/xiaohongshu-mcp/cookies"
	"github.com/xpzouying/xiaohongshu-mcp/pkg/downloader"
	"github.com/xpzouying/xiaohongshu-mcp/pkg/xhsutil"
	"github.com/xpzouying/xiaohongshu-mcp/xiaohongshu"
)

// XiaohongshuService 小红书业务服务
type XiaohongshuService struct {
	pageController pageController
}

// NewXiaohongshuService 创建小红书服务实例
func NewXiaohongshuService() *XiaohongshuService {
	return &XiaohongshuService{
		pageController: newPageController(),
	}
}

type browserSession interface {
	NewPage() *rod.Page
	Close() error
}

type legacyBrowser struct {
	browser *headless_browser.Browser
}

func (b legacyBrowser) NewPage() *rod.Page {
	return b.browser.NewPage()
}

func (b legacyBrowser) Close() error {
	b.browser.Close()
	return nil
}

type cdpBrowser struct {
	browser    *rod.Browser
	disconnect func() error
}

func (b cdpBrowser) NewPage() *rod.Page {
	return stealth.MustPage(b.browser)
}

func (b cdpBrowser) Close() error {
	if b.disconnect != nil {
		return b.disconnect()
	}
	return nil
}

func getCachedLoginStatus(cookiePath string) (*LoginStatusResponse, bool) {
	if configs.IsCdpMode() {
		return nil, false
	}
	if _, err := os.Stat(cookiePath); err != nil {
		if os.IsNotExist(err) {
			return &LoginStatusResponse{
				IsLoggedIn: false,
				Username:   configs.Username,
				State:      string(xiaohongshu.LoginStateLoggedOut),
			}, true
		}
		return nil, false
	}
	return nil, false
}

// PublishRequest 发布请求
type PublishRequest struct {
	Title      string   `json:"title" binding:"required"`
	Content    string   `json:"content" binding:"required"`
	Images     []string `json:"images" binding:"required,min=1"`
	Tags       []string `json:"tags,omitempty"`
	ScheduleAt string   `json:"schedule_at,omitempty"` // 定时发布时间，ISO8601格式，为空则立即发布
	IsOriginal bool     `json:"is_original,omitempty"` // 是否声明原创
	Visibility string   `json:"visibility,omitempty"`  // 可见范围: "公开可见"(默认), "仅自己可见", "仅互关好友可见"
	Products   []string `json:"products,omitempty"`    // 商品关键词列表，用于绑定带货商品
}

// LoginStatusResponse 登录状态响应
type LoginStatusResponse struct {
	IsLoggedIn bool                           `json:"is_logged_in"`
	Username   string                         `json:"username,omitempty"`
	State      string                         `json:"state,omitempty"`
	Signals    xiaohongshu.LoginStatusSignals `json:"signals"`
}

// LoginQrcodeResponse 登录扫码二维码
type LoginQrcodeResponse struct {
	Timeout    string `json:"timeout"`
	IsLoggedIn bool   `json:"is_logged_in"`
	Img        string `json:"img,omitempty"`
}

// PublishResponse 发布响应
type PublishResponse struct {
	Title   string `json:"title"`
	Content string `json:"content"`
	Images  int    `json:"images"`
	Status  string `json:"status"`
	PostID  string `json:"post_id,omitempty"`
}

// PublishVideoRequest 发布视频请求（仅支持本地单个视频文件）
type PublishVideoRequest struct {
	Title      string   `json:"title" binding:"required"`
	Content    string   `json:"content" binding:"required"`
	Video      string   `json:"video" binding:"required"`
	Tags       []string `json:"tags,omitempty"`
	ScheduleAt string   `json:"schedule_at,omitempty"` // 定时发布时间，ISO8601格式，为空则立即发布
	Visibility string   `json:"visibility,omitempty"`  // 可见范围: "公开可见"(默认), "仅自己可见", "仅互关好友可见"
	Products   []string `json:"products,omitempty"`    // 商品关键词列表，用于绑定带货商品
}

// PublishVideoResponse 发布视频响应
type PublishVideoResponse struct {
	Title   string `json:"title"`
	Content string `json:"content"`
	Video   string `json:"video"`
	Status  string `json:"status"`
	PostID  string `json:"post_id,omitempty"`
}

// FeedsListResponse Feeds列表响应
type FeedsListResponse struct {
	Feeds []xiaohongshu.Feed `json:"feeds"`
	Count int                `json:"count"`
}

// UserProfileResponse 用户主页响应
type UserProfileResponse struct {
	UserBasicInfo xiaohongshu.UserBasicInfo      `json:"userBasicInfo"`
	Interactions  []xiaohongshu.UserInteractions `json:"interactions"`
	Feeds         []xiaohongshu.Feed             `json:"feeds"`
}

// DeleteCookies 删除 cookies 文件，用于登录重置
func (s *XiaohongshuService) DeleteCookies(ctx context.Context) error {
	cookiePath := cookies.GetCookiesFilePath()
	cookieLoader := cookies.NewLoadCookie(cookiePath)
	return cookieLoader.DeleteCookies()
}

// CheckLoginStatus 检查登录状态
func (s *XiaohongshuService) CheckLoginStatus(ctx context.Context) (*LoginStatusResponse, error) {
	cookiePath := cookies.GetCookiesFilePath()
	if cached, ok := getCachedLoginStatus(cookiePath); ok {
		return cached, nil
	}

	lease, err := s.pageController.Acquire(pageRoleLogin)
	if err != nil {
		return nil, err
	}
	var opErr error
	defer func() { lease.Release(opErr) }()

	loginAction := xiaohongshu.NewLogin(lease.Page)

	probe, opErr := loginAction.CheckLoginStatus(ctx)
	if opErr != nil {
		return nil, opErr
	}

	response := &LoginStatusResponse{
		IsLoggedIn: probe.IsLoggedIn,
		Username:   configs.Username,
		State:      string(probe.State),
		Signals:    probe.Signals,
	}

	return response, nil
}

// GetLoginQrcode 获取登录的扫码二维码
func (s *XiaohongshuService) GetLoginQrcode(ctx context.Context) (*LoginQrcodeResponse, error) {
	lease, err := s.pageController.Acquire(pageRoleLogin)
	if err != nil {
		return nil, err
	}
	page := lease.Page
	releaseLease := func(opErr error) {
		lease.Release(opErr)
	}

	loginAction := xiaohongshu.NewLogin(page)

	img, loggedIn, err := loginAction.FetchQrcodeImage(ctx)
	if err != nil || loggedIn {
		defer releaseLease(err)
	}
	if err != nil {
		return nil, err
	}

	timeout := 4 * time.Minute

	if !loggedIn {
		go func() {
			ctxTimeout, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			var asyncErr error
			defer releaseLease(asyncErr)

			if loginAction.WaitForLogin(ctxTimeout) {
				if !configs.IsCdpMode() {
					if er := saveCookies(page); er != nil {
						logrus.Errorf("failed to save cookies: %v", er)
					}
				}
				if configs.IsCdpMode() {
					logrus.Debug("cdp mode login complete; relying on Chrome profile instead of cookies.json")
				}
			}
		}()
	}

	return &LoginQrcodeResponse{
		Timeout: func() string {
			if loggedIn {
				return "0s"
			}
			return timeout.String()
		}(),
		Img:        img,
		IsLoggedIn: loggedIn,
	}, nil
}

func closeBrowser(b browserSession) {
	if b != nil {
		_ = b.Close()
	}
}

func closePage(page *rod.Page) {
	if page != nil {
		_ = page.Close()
	}
}

// PublishContent 发布内容
func (s *XiaohongshuService) PublishContent(ctx context.Context, req *PublishRequest) (*PublishResponse, error) {
	// 验证标题长度（小红书限制：最大20个字）
	if xhsutil.CalcTitleLength(req.Title) > 20 {
		return nil, fmt.Errorf("标题长度超过限制")
	}

	// 处理图片：下载URL图片或使用本地路径
	imagePaths, err := s.processImages(req.Images)
	if err != nil {
		return nil, err
	}

	// 解析定时发布时间
	var scheduleTime *time.Time
	if req.ScheduleAt != "" {
		t, err := time.Parse(time.RFC3339, req.ScheduleAt)
		if err != nil {
			return nil, fmt.Errorf("定时发布时间格式错误，请使用 ISO8601 格式: %v", err)
		}

		// 校验定时发布时间范围：1小时至14天
		now := time.Now()
		minTime := now.Add(1 * time.Hour)
		maxTime := now.Add(14 * 24 * time.Hour)

		if t.Before(minTime) {
			return nil, fmt.Errorf("定时发布时间必须至少在1小时后，当前设置: %s，最早可选: %s",
				t.Format("2006-01-02 15:04"), minTime.Format("2006-01-02 15:04"))
		}
		if t.After(maxTime) {
			return nil, fmt.Errorf("定时发布时间不能超过14天，当前设置: %s，最晚可选: %s",
				t.Format("2006-01-02 15:04"), maxTime.Format("2006-01-02 15:04"))
		}

		scheduleTime = &t
		logrus.Infof("设置定时发布时间: %s", t.Format("2006-01-02 15:04"))
	}

	// 构建发布内容
	content := xiaohongshu.PublishImageContent{
		Title:        req.Title,
		Content:      req.Content,
		Tags:         req.Tags,
		ImagePaths:   imagePaths,
		ScheduleTime: scheduleTime,
		IsOriginal:   req.IsOriginal,
		Visibility:   req.Visibility,
		Products:     req.Products,
	}

	// 执行发布
	if err := s.publishContent(ctx, content); err != nil {
		logrus.Errorf("发布内容失败: title=%s %v", content.Title, err)
		return nil, err
	}

	response := &PublishResponse{
		Title:   req.Title,
		Content: req.Content,
		Images:  len(imagePaths),
		Status:  "发布完成",
	}

	return response, nil
}

// processImages 处理图片列表，支持URL下载和本地路径
func (s *XiaohongshuService) processImages(images []string) ([]string, error) {
	processor := downloader.NewImageProcessor()
	return processor.ProcessImages(images)
}

// publishContent 执行内容发布
func (s *XiaohongshuService) publishContent(ctx context.Context, content xiaohongshu.PublishImageContent) error {
	lease, err := s.pageController.Acquire(pageRoleWork)
	if err != nil {
		return err
	}
	var opErr error
	defer func() { lease.Release(opErr) }()

	action, opErr := xiaohongshu.NewPublishImageAction(lease.Page)
	if opErr != nil {
		return opErr
	}

	// 执行发布
	opErr = action.Publish(ctx, content)
	return opErr
}

// PublishVideo 发布视频（本地文件）
func (s *XiaohongshuService) PublishVideo(ctx context.Context, req *PublishVideoRequest) (*PublishVideoResponse, error) {
	// 标题长度校验（小红书限制：最大20个字）
	if xhsutil.CalcTitleLength(req.Title) > 20 {
		return nil, fmt.Errorf("标题长度超过限制")
	}

	// 本地视频文件校验
	if req.Video == "" {
		return nil, fmt.Errorf("必须提供本地视频文件")
	}
	if _, err := os.Stat(req.Video); err != nil {
		return nil, fmt.Errorf("视频文件不存在或不可访问: %v", err)
	}

	// 解析定时发布时间
	var scheduleTime *time.Time
	if req.ScheduleAt != "" {
		t, err := time.Parse(time.RFC3339, req.ScheduleAt)
		if err != nil {
			return nil, fmt.Errorf("定时发布时间格式错误，请使用 ISO8601 格式: %v", err)
		}

		// 校验定时发布时间范围：1小时至14天
		now := time.Now()
		minTime := now.Add(1 * time.Hour)
		maxTime := now.Add(14 * 24 * time.Hour)

		if t.Before(minTime) {
			return nil, fmt.Errorf("定时发布时间必须至少在1小时后，当前设置: %s，最早可选: %s",
				t.Format("2006-01-02 15:04"), minTime.Format("2006-01-02 15:04"))
		}
		if t.After(maxTime) {
			return nil, fmt.Errorf("定时发布时间不能超过14天，当前设置: %s，最晚可选: %s",
				t.Format("2006-01-02 15:04"), maxTime.Format("2006-01-02 15:04"))
		}

		scheduleTime = &t
		logrus.Infof("设置定时发布时间: %s", t.Format("2006-01-02 15:04"))
	}

	// 构建发布内容
	content := xiaohongshu.PublishVideoContent{
		Title:        req.Title,
		Content:      req.Content,
		Tags:         req.Tags,
		VideoPath:    req.Video,
		ScheduleTime: scheduleTime,
		Visibility:   req.Visibility,
		Products:     req.Products,
	}

	// 执行发布
	if err := s.publishVideo(ctx, content); err != nil {
		return nil, err
	}

	resp := &PublishVideoResponse{
		Title:   req.Title,
		Content: req.Content,
		Video:   req.Video,
		Status:  "发布完成",
	}
	return resp, nil
}

// publishVideo 执行视频发布
func (s *XiaohongshuService) publishVideo(ctx context.Context, content xiaohongshu.PublishVideoContent) error {
	lease, err := s.pageController.Acquire(pageRoleWork)
	if err != nil {
		return err
	}
	var opErr error
	defer func() { lease.Release(opErr) }()

	action, opErr := xiaohongshu.NewPublishVideoAction(lease.Page)
	if opErr != nil {
		return opErr
	}

	opErr = action.PublishVideo(ctx, content)
	return opErr
}

// ListFeeds 获取Feeds列表
func (s *XiaohongshuService) ListFeeds(ctx context.Context) (*FeedsListResponse, error) {
	lease, err := s.pageController.Acquire(pageRoleWork)
	if err != nil {
		return nil, err
	}
	var opErr error
	defer func() { lease.Release(opErr) }()

	// 创建 Feeds 列表 action
	action := xiaohongshu.NewFeedsListAction(lease.Page)

	// 获取 Feeds 列表
	feeds, opErr := action.GetFeedsList(ctx)
	if opErr != nil {
		logrus.Errorf("获取 Feeds 列表失败: %v", opErr)
		return nil, opErr
	}

	response := &FeedsListResponse{
		Feeds: feeds,
		Count: len(feeds),
	}

	return response, nil
}

func (s *XiaohongshuService) SearchFeeds(ctx context.Context, keyword string, filters ...xiaohongshu.FilterOption) (*FeedsListResponse, error) {
	lease, err := s.pageController.Acquire(pageRoleWork)
	if err != nil {
		return nil, err
	}
	var opErr error
	defer func() { lease.Release(opErr) }()

	action := xiaohongshu.NewSearchAction(lease.Page)

	feeds, opErr := action.Search(ctx, keyword, filters...)
	if opErr != nil {
		return nil, opErr
	}

	response := &FeedsListResponse{
		Feeds: feeds,
		Count: len(feeds),
	}

	return response, nil
}

// GetFeedDetail 获取Feed详情
func (s *XiaohongshuService) GetFeedDetail(ctx context.Context, feedID, xsecToken string, loadAllComments bool) (*FeedDetailResponse, error) {
	return s.GetFeedDetailWithConfig(ctx, feedID, xsecToken, loadAllComments, xiaohongshu.DefaultCommentLoadConfig())
}

// GetFeedDetailWithConfig 使用配置获取Feed详情
func (s *XiaohongshuService) GetFeedDetailWithConfig(ctx context.Context, feedID, xsecToken string, loadAllComments bool, config xiaohongshu.CommentLoadConfig) (*FeedDetailResponse, error) {
	lease, err := s.pageController.Acquire(pageRoleWork)
	if err != nil {
		return nil, err
	}
	var opErr error
	defer func() { lease.Release(opErr) }()

	// 创建 Feed 详情 action
	action := xiaohongshu.NewFeedDetailAction(lease.Page)

	// 获取 Feed 详情
	result, opErr := action.GetFeedDetailWithConfig(ctx, feedID, xsecToken, loadAllComments, config)
	if opErr != nil {
		return nil, opErr
	}

	response := &FeedDetailResponse{
		FeedID: feedID,
		Data:   result,
	}

	return response, nil
}

// UserProfile 获取用户信息
func (s *XiaohongshuService) UserProfile(ctx context.Context, userID, xsecToken string) (*UserProfileResponse, error) {
	lease, err := s.pageController.Acquire(pageRoleWork)
	if err != nil {
		return nil, err
	}
	var opErr error
	defer func() { lease.Release(opErr) }()

	action := xiaohongshu.NewUserProfileAction(lease.Page)
	result, opErr := action.UserProfile(ctx, userID, xsecToken)
	if opErr != nil {
		return nil, opErr
	}
	response := &UserProfileResponse{
		UserBasicInfo: result.UserBasicInfo,
		Interactions:  result.Interactions,
		Feeds:         result.Feeds,
	}

	return response, nil

}

// PostCommentToFeed 发表评论到Feed
func (s *XiaohongshuService) PostCommentToFeed(ctx context.Context, feedID, xsecToken, content string) (*PostCommentResponse, error) {
	lease, err := s.pageController.Acquire(pageRoleWork)
	if err != nil {
		return nil, err
	}
	var opErr error
	defer func() { lease.Release(opErr) }()

	action := xiaohongshu.NewCommentFeedAction(lease.Page)

	opErr = action.PostComment(ctx, feedID, xsecToken, content)
	if opErr != nil {
		return nil, opErr
	}

	return &PostCommentResponse{FeedID: feedID, Success: true, Message: "评论发表成功"}, nil
}

// LikeFeed 点赞笔记
func (s *XiaohongshuService) LikeFeed(ctx context.Context, feedID, xsecToken string) (*ActionResult, error) {
	lease, err := s.pageController.Acquire(pageRoleWork)
	if err != nil {
		return nil, err
	}
	var opErr error
	defer func() { lease.Release(opErr) }()

	action := xiaohongshu.NewLikeAction(lease.Page)
	opErr = action.Like(ctx, feedID, xsecToken)
	if opErr != nil {
		return nil, opErr
	}
	return &ActionResult{FeedID: feedID, Success: true, Message: "点赞成功或已点赞"}, nil
}

// UnlikeFeed 取消点赞笔记
func (s *XiaohongshuService) UnlikeFeed(ctx context.Context, feedID, xsecToken string) (*ActionResult, error) {
	lease, err := s.pageController.Acquire(pageRoleWork)
	if err != nil {
		return nil, err
	}
	var opErr error
	defer func() { lease.Release(opErr) }()

	action := xiaohongshu.NewLikeAction(lease.Page)
	opErr = action.Unlike(ctx, feedID, xsecToken)
	if opErr != nil {
		return nil, opErr
	}
	return &ActionResult{FeedID: feedID, Success: true, Message: "取消点赞成功或未点赞"}, nil
}

// FavoriteFeed 收藏笔记
func (s *XiaohongshuService) FavoriteFeed(ctx context.Context, feedID, xsecToken string) (*ActionResult, error) {
	lease, err := s.pageController.Acquire(pageRoleWork)
	if err != nil {
		return nil, err
	}
	var opErr error
	defer func() { lease.Release(opErr) }()

	action := xiaohongshu.NewFavoriteAction(lease.Page)
	opErr = action.Favorite(ctx, feedID, xsecToken)
	if opErr != nil {
		return nil, opErr
	}
	return &ActionResult{FeedID: feedID, Success: true, Message: "收藏成功或已收藏"}, nil
}

// UnfavoriteFeed 取消收藏笔记
func (s *XiaohongshuService) UnfavoriteFeed(ctx context.Context, feedID, xsecToken string) (*ActionResult, error) {
	lease, err := s.pageController.Acquire(pageRoleWork)
	if err != nil {
		return nil, err
	}
	var opErr error
	defer func() { lease.Release(opErr) }()

	action := xiaohongshu.NewFavoriteAction(lease.Page)
	opErr = action.Unfavorite(ctx, feedID, xsecToken)
	if opErr != nil {
		return nil, opErr
	}
	return &ActionResult{FeedID: feedID, Success: true, Message: "取消收藏成功或未收藏"}, nil
}

// ReplyCommentToFeed 回复指定评论
func (s *XiaohongshuService) ReplyCommentToFeed(ctx context.Context, feedID, xsecToken, commentID, userID, content string) (*ReplyCommentResponse, error) {
	lease, err := s.pageController.Acquire(pageRoleWork)
	if err != nil {
		return nil, err
	}
	var opErr error
	defer func() { lease.Release(opErr) }()

	action := xiaohongshu.NewCommentFeedAction(lease.Page)

	opErr = action.ReplyToComment(ctx, feedID, xsecToken, commentID, userID, content)
	if opErr != nil {
		return nil, opErr
	}

	return &ReplyCommentResponse{
		FeedID:          feedID,
		TargetCommentID: commentID,
		TargetUserID:    userID,
		Success:         true,
		Message:         "评论回复成功",
	}, nil
}

func saveCookies(page *rod.Page) error {
	cks, err := page.Browser().GetCookies()
	if err != nil {
		return err
	}

	data, err := json.Marshal(cks)
	if err != nil {
		return err
	}

	cookieLoader := cookies.NewLoadCookie(cookies.GetCookiesFilePath())
	return cookieLoader.SaveCookies(data)
}

// GetMyProfile 获取当前登录用户的个人信息
func (s *XiaohongshuService) GetMyProfile(ctx context.Context) (*UserProfileResponse, error) {
	var result *xiaohongshu.UserProfileResponse
	lease, err := s.pageController.Acquire(pageRoleWork)
	if err != nil {
		return nil, err
	}
	var opErr error
	defer func() { lease.Release(opErr) }()

	opErr = func(page *rod.Page) error {
		action := xiaohongshu.NewUserProfileAction(page)
		result, err = action.GetMyProfileViaSidebar(ctx)
		return err
	}(lease.Page)

	if opErr != nil {
		return nil, opErr
	}

	response := &UserProfileResponse{
		UserBasicInfo: result.UserBasicInfo,
		Interactions:  result.Interactions,
		Feeds:         result.Feeds,
	}

	return response, nil
}
