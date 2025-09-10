/*
 * Copyright 2025 coze-dev Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package xiaohongshu

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"log"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/coze-dev/coze-studio/backend/domain/publishThird/entity"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/pkg/errors"
)

// 全局变量管理浏览器实例和页面
var (
	browserManager *BrowserManager
	once           sync.Once
)

// BrowserManager 管理浏览器实例
type BrowserManager struct {
	browser *rod.Browser
	page    *rod.Page
	mu      sync.RWMutex
	isLogin bool
}

// 响应结构体
type Response struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data,omitempty"`
}

// 二维码响应
type QRCodeResponse struct {
	QRCode string `json:"qr_code"` // base64 编码的二维码图片
}

// 登录状态响应
type LoginStatusResponse struct {
	IsLogin bool `json:"is_login"`
}

// 发布请求
type PublishRequest struct {
	Title      string   `json:"title"`
	Content    string   `json:"content"`
	ImagePaths []string `json:"image_paths"`
}

const cookieFile = "cookie.json"

// NewBrowserManager 创建浏览器管理器
func NewBrowserManager() *BrowserManager {
	browser := newBrowser(false)
	page := browser.MustPage("https://www.xiaohongshu.com/")
	page.MustWaitLoad()

	bm := &BrowserManager{
		browser: browser,
		page:    page,
		isLogin: false,
	}

	// 尝试加载已有的 cookie
	if loadCookies(page) {
		bm.isLogin = true
		log.Println("已加载 cookie，用户已登录")
	}

	return bm
}

// 获取二维码
func getQRCode(ctx context.Context, c *app.RequestContext) {
	browserManager.mu.Lock()
	defer browserManager.mu.Unlock()

	try := func() (*Response, error) {
		// 如果已经登录，返回登录状态
		if browserManager.isLogin {
			return &Response{
				Code: 200,
				Msg:  "用户已登录",
				Data: LoginStatusResponse{IsLogin: true},
			}, nil
		}

		// 刷新页面以获取新的二维码
		browserManager.page.MustReload()
		browserManager.page.MustWaitLoad()

		// 等待二维码元素出现
		time.Sleep(2 * time.Second)

		// 截取二维码
		qrBase64, err := scanLogin(browserManager.page)
		if err != nil {
			return &Response{
				Code: 500,
				Msg:  "获取二维码失败: " + err.Error(),
			}, err
		}

		return &Response{
			Code: 200,
			Msg:  "获取二维码成功",
			Data: QRCodeResponse{QRCode: qrBase64},
		}, nil
	}

	resp, err := try()
	if err != nil {
		slog.Error("获取二维码失败", "error", err)
	}

	c.JSON(consts.StatusOK, resp)
}

// 检查登录状态
func getLoginStatus(ctx context.Context, c *app.RequestContext) {
	browserManager.mu.RLock()
	defer browserManager.mu.RUnlock()

	try := func() *Response {
		// 检查登录状态
		isLogin := checkLoginStatus(browserManager.page)

		if isLogin && !browserManager.isLogin {
			// 状态发生变化，保存 cookie
			saveCookies(browserManager.page)
			browserManager.isLogin = true
		}

		return &Response{
			Code: 200,
			Msg:  "获取登录状态成功",
			Data: LoginStatusResponse{IsLogin: isLogin},
		}
	}

	resp := try()
	c.JSON(consts.StatusOK, resp)
}

// 发布笔记
func publishNote(ctx context.Context, c *app.RequestContext) {
	var req PublishRequest
	if err := c.BindAndValidate(&req); err != nil {
		c.JSON(consts.StatusBadRequest, Response{
			Code: 400,
			Msg:  "请求参数错误: " + err.Error(),
		})
		return
	}

	browserManager.mu.Lock()
	defer browserManager.mu.Unlock()

	try := func() *Response {
		// 检查登录状态
		if !browserManager.isLogin {
			return &Response{
				Code: 401,
				Msg:  "用户未登录",
			}
		}

		// 创建发布动作
		action, err := NewPublishImageAction(browserManager.page)
		if err != nil {
			slog.Error("创建发布动作失败", "error", err)
			return &Response{
				Code: 500,
				Msg:  "创建发布动作失败: " + err.Error(),
			}
		}

		// 发布内容
		content := entity.PublishImageContent{
			Title:      req.Title,
			Content:    req.Content,
			ImagePaths: req.ImagePaths,
		}

		if _, err := action.publishArticle(ctx, &content); err != nil {
			slog.Error("发布失败", "error", err)
			return &Response{
				Code: 500,
				Msg:  "发布失败: " + err.Error(),
			}
		}

		return &Response{
			Code: 200,
			Msg:  "发布成功",
		}
	}

	resp := try()
	c.JSON(consts.StatusOK, resp)
}

// 重置浏览器
func resetBrowser(ctx context.Context, c *app.RequestContext) {
	browserManager.mu.Lock()
	defer browserManager.mu.Unlock()

	try := func() *Response {
		// 关闭当前浏览器
		if browserManager.browser != nil {
			browserManager.browser.MustClose()
		}

		// 重新创建浏览器实例
		browser := newBrowser(false)
		page := browser.MustPage("https://www.xiaohongshu.com/")
		page.MustWaitLoad()

		browserManager.browser = browser
		browserManager.page = page
		browserManager.isLogin = false

		return &Response{
			Code: 200,
			Msg:  "浏览器重置成功",
		}
	}

	resp := try()
	c.JSON(consts.StatusOK, resp)
}

// ---------- 原有功能函数适配 ----------

// newBrowser 启动浏览器
func newBrowser(headless bool) *rod.Browser {
	l := launcher.New().Headless(headless).MustLaunch()
	browser := rod.New().ControlURL(l).MustConnect()
	return browser
}

// saveCookies 保存 Cookie
func saveCookies(page *rod.Page) {
	cookies, _ := page.Cookies([]string{})
	data, _ := json.Marshal(cookies)
	os.WriteFile(cookieFile, data, 0644)
	log.Println("Cookie 已保存到", cookieFile)
}

// loadCookies 加载 Cookie
func loadCookies(page *rod.Page) bool {
	data, err := os.ReadFile(cookieFile)
	if err != nil {
		return false
	}

	var cookies []*proto.NetworkCookie
	if err := json.Unmarshal(data, &cookies); err != nil {
		return false
	}

	now := time.Now().Unix() // 当前时间的 Unix 秒数
	var params []*proto.NetworkCookieParam
	for _, c := range cookies {
		// 如果 c.Expires 为 0，说明是会话 cookie，不用判断
		if c.Expires != 0 && int64(c.Expires) < now {
			// cookie 已过期，跳过
			continue
		}
		params = append(params, &proto.NetworkCookieParam{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   c.Domain,
			Path:     c.Path,
			Expires:  c.Expires,
			Secure:   c.Secure,
			SameSite: c.SameSite,
		})
	}

	page.MustSetCookies(params...)
	page.MustReload()
	return true
}

// scanLogin 扫码登录
func scanLogin(page *rod.Page) (string, error) {
	// 查找二维码元素
	qrEl, err := page.Element(".qrcode")
	if err != nil {
		return "", errors.Wrap(err, "找不到二维码元素")
	}

	// 截取二维码
	qrPNG, err := qrEl.Screenshot(proto.PageCaptureScreenshotFormatPng, 0)
	if err != nil {
		return "", errors.Wrap(err, "截取二维码失败")
	}

	qrBase64 := base64.StdEncoding.EncodeToString(qrPNG)
	return qrBase64, nil
}

// checkLoginStatus 检查登录状态
func checkLoginStatus(page *rod.Page) bool {
	// 登录成功标志：页面 li.user.side-bar-component span.channel 文本为 "我"
	if el, _ := page.ElementR("li.user.side-bar-component span.channel", "我"); el != nil {
		return true
	}
	return false
}

// PublishAction 发布动作
type PublishAction struct {
	page *rod.Page
}

const urlOfPublic = `https://creator.xiaohongshu.com/publish/publish?source=official`

// NewPublishImageAction 创建发布动作
func NewPublishImageAction(page *rod.Page) (*PublishAction, error) {
	pp := page.Timeout(60 * time.Second)
	pp.MustNavigate(urlOfPublic)
	pp.MustElement(`div.upload-content`).MustWaitVisible()
	slog.Info("wait for upload-content visible success")

	// 等待一段时间确保页面完全加载
	time.Sleep(1 * time.Second)

	createElems := pp.MustElements("div.creator-tab")
	slog.Info("found creator-tab elements", "count", len(createElems))

	for _, elem := range createElems {
		text, err := elem.Text()
		if err != nil {
			slog.Error("获取元素文本失败", "error", err)
			continue
		}

		if text == "上传图文" {
			if err := elem.Click(proto.InputMouseButtonLeft, 1); err != nil {
				slog.Error("点击元素失败", "error", err)
				continue
			}
			break
		}
	}

	time.Sleep(1 * time.Second)

	return &PublishAction{page: pp}, nil
}

// Publish 发布内容
func (p *PublishAction) publishArticle(ctx context.Context, content *entity.PublishImageContent) (string, error) {
	if len(content.ImagePaths) == 0 {
		return "", errors.New("图片不能为空")
	}

	page := p.page.Context(ctx)

	if err := uploadImages(page, content.ImagePaths); err != nil {
		return "", errors.Wrap(err, "小红书上传图片失败")
	}

	if err := submitPublish(page, content.Title, content.Content); err != nil {
		return "", errors.Wrap(err, "小红书发布失败")
	}

	return "", nil
}

type NoteInfo struct {
	URL          string
	LikeCount    string
	CollectCount string
	ChatCount    string
}

// getInfo 获取点赞量、收藏量、评论量
func (p *PublishAction) getTweetInfo(ctx context.Context, links []string) ([]NoteInfo, error) {
	browser := newBrowser(true)
	defer browser.MustClose()

	resultsCh := make(chan NoteInfo, len(links))
	errCh := make(chan error, len(links))

	// 使用原来的 ctx，不要重新创建
	for _, link := range links {
		link := link // 避免闭包捕获
		go func() {
			// 每个页面加载也设置单独超时
			_, cancel := context.WithTimeout(ctx, 20*time.Second)
			defer cancel()

			notePage := browser.MustPage(link).
				Timeout(20 * time.Second).MustWaitLoad()

			var likeCount, collectCount, chatCount string

			// 获取点赞
			if likeEl := notePage.Timeout(20*time.Second).MustElementR(".like-wrapper .count", `\d|万`); likeEl != nil {
				likeCount = likeEl.MustText()
			} else {
				likeCount = "N/A"
			}

			// 获取收藏
			collectCount = notePage.MustElement(".collect-wrapper .count").MustText()

			if collectCount == "" {
				collectCount = "N/A"
			}

			// 获取评论
			chatCount = notePage.MustElement(".chat-wrapper .count").MustText()
			if chatCount == "" {
				chatCount = "N/A"
			}

			log.Printf("链接: %s 点赞:%s 收藏:%s 评论:%s", link, likeCount, collectCount, chatCount)

			resultsCh <- NoteInfo{
				URL:          link,
				LikeCount:    likeCount,
				CollectCount: collectCount,
				ChatCount:    chatCount,
			}
		}()
	}

	var results []NoteInfo
	for i := 0; i < len(links); i++ {
		select {
		case res := <-resultsCh:
			results = append(results, res)
		case err := <-errCh:
			log.Println("抓取错误:", err)
			// 可以选择继续或直接返回错误，这里继续抓取
		case <-ctx.Done():
			return results, ctx.Err() // 上下文超时或取消
		}
	}

	return results, nil
}

// uploadImages 上传图片
func uploadImages(page *rod.Page, imagesPaths []string) error {
	pp := page.Timeout(30 * time.Second)
	uploadInput := pp.MustElement(".upload-input")
	uploadInput.MustSetFiles(imagesPaths...)
	time.Sleep(3 * time.Second)
	return nil
}

// submitPublish 提交发布
func submitPublish(page *rod.Page, title, content string) error {
	titleElem := page.MustElement("div.d-input input")
	titleElem.MustInput(title)
	time.Sleep(1 * time.Second)

	if contentElem, ok := getContentElement(page); ok {
		contentElem.MustInput(content)
	} else {
		return errors.New("没有找到内容输入框")
	}

	time.Sleep(1 * time.Second)

	submitButton := page.MustElement("div.submit div.d-button-content")
	submitButton.MustClick()
	time.Sleep(3 * time.Second)

	return nil
}

// getContentElement 查找内容输入框
func getContentElement(page *rod.Page) (*rod.Element, bool) {
	var foundElement *rod.Element
	var found bool

	page.Race().
		Element("div.ql-editor").MustHandle(func(e *rod.Element) {
		foundElement = e
		found = true
	}).
		ElementFunc(func(page *rod.Page) (*rod.Element, error) {
			return findTextboxByPlaceholder(page)
		}).MustHandle(func(e *rod.Element) {
		foundElement = e
		found = true
	}).
		MustDo()

	if found {
		return foundElement, true
	}

	slog.Warn("no content element found by any method")
	return nil, false
}

// findTextboxByPlaceholder 通过占位符查找文本框
func findTextboxByPlaceholder(page *rod.Page) (*rod.Element, error) {
	elements := page.MustElements("p")
	if elements == nil {
		return nil, errors.New("no p elements found")
	}

	placeholderElem := findPlaceholderElement(elements, "输入正文描述")
	if placeholderElem == nil {
		return nil, errors.New("no placeholder element found")
	}

	textboxElem := findTextboxParent(placeholderElem)
	if textboxElem == nil {
		return nil, errors.New("no textbox parent found")
	}

	return textboxElem, nil
}

// findPlaceholderElement 查找占位符元素
func findPlaceholderElement(elements []*rod.Element, searchText string) *rod.Element {
	for _, elem := range elements {
		placeholder, err := elem.Attribute("data-placeholder")
		if err != nil || placeholder == nil {
			continue
		}

		if strings.Contains(*placeholder, searchText) {
			return elem
		}
	}
	return nil
}

// findTextboxParent 查找文本框父元素
func findTextboxParent(elem *rod.Element) *rod.Element {
	currentElem := elem
	for i := 0; i < 5; i++ {
		parent, err := currentElem.Parent()
		if err != nil {
			break
		}

		role, err := parent.Attribute("role")
		if err != nil || role == nil {
			currentElem = parent
			continue
		}

		if *role == "textbox" {
			return parent
		}

		currentElem = parent
	}
	return nil
}
