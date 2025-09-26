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

package publishThird

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/coze-dev/coze-studio/backend/api/model/publishThird"
	"github.com/coze-dev/coze-studio/backend/domain/publishThird/entity"
	"github.com/coze-dev/coze-studio/backend/domain/publishThird/service"
	"github.com/coze-dev/coze-studio/backend/infra/contract/storage"
	"github.com/coze-dev/coze-studio/backend/infra/impl/cache/redis"
	"github.com/coze-dev/coze-studio/backend/pkg/errorx"
	"github.com/coze-dev/coze-studio/backend/pkg/logs"
	"github.com/coze-dev/coze-studio/backend/pkg/safego"
	"github.com/coze-dev/coze-studio/backend/types/errno"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/pkg/errors"
	"io"
	"log"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"
)

type PublishThirdApplicationService struct {
	DomainSVC service.PublishThird
	storage   storage.Storage
}

var PublishThirdApplicationSVC = &PublishThirdApplicationService{}

// 全局变量管理浏览器实例和页面
var (
	browserManager *BrowserManager
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
func NewBrowserManager(ctx context.Context, key string) (*BrowserManager, *launcher.Launcher) {
	browser, l := newBrowser(true)
	page := browser.MustPage("https://www.xiaohongshu.com/")
	page.MustWaitLoad()

	bm := &BrowserManager{
		browser: browser,
		page:    page,
		isLogin: false,
	}

	// 尝试加载已有的 cookie
	if loadCookies(ctx, page, key) {
		bm.isLogin = true
		log.Println("已加载 cookie，用户已登录")
	}

	return bm, l
}

// 获取二维码
func GetQRCode(ctx context.Context, c *app.RequestContext) {
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
func GetLoginStatus(ctx context.Context, c *app.RequestContext) {
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

func waitForLogin(page *rod.Page, timeout time.Duration) bool {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return false
		case <-ticker.C:
			// 登录成功标志：页面 li.user.side-bar-component span.channel 文本为 "我"
			if el, _ := page.ElementR("li.user.side-bar-component span.channel", "我"); el != nil {
				return true
			}
		}
	}
}

func downloadImage(url, savePath string) (string, error) {
	// 发送 GET 请求
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("bad status: %s", resp.Status)
	}

	// 创建保存文件
	out, err := os.Create(savePath)
	if err != nil {
		return "", err
	}
	defer out.Close()

	// 将图片内容写入文件
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return "", err
	}

	return savePath, nil
}

// 发布笔记
func (p *PublishThirdApplicationService) PublishNote(ctx context.Context, req publishThird.GetXHSRequest) (*publishThird.PublishThirdResponse[string], error) {
	userID := req.UserId
	if req.UserId == "" {
		return nil, errorx.New(errno.ErrUserAuthenticationFailed, errorx.KV("reason", "missing session_key in cookie"))
	}

	Manager, l := NewBrowserManager(ctx, userID)
	defer func() {
		if l != nil {
			l.Kill()
		}
	}()

	resp := publishThird.PublishThirdResponse[string]{
		Code:    0,
		Message: "ok",
	}

	try := func() *Response {
		// 不用再检测是否登录,调这个接口前必定先调是否登录接口
		if !Manager.isLogin {
			return &Response{
				Code: 401,
				Msg:  "用户未登录",
			}
		}

		// 创建发布动作
		page := Manager.page
		time.Sleep(1 * time.Second)
		action, err := NewPublishImageAction(page)
		if err != nil {
			fmt.Printf("cuowu :---,%v", err)
			logs.CtxErrorf(ctx, "发布失败：----%v", err)
			l.Kill()
			return &Response{
				Code: 503,
				Msg:  "创建发布动作失败: " + err.Error(),
			}
		}
		if len(req.Title) == 0 && len(req.Content) == 0 {
			slog.Error("标题或内容为空", "error", err)
			l.Kill()
			return &Response{
				Code: 500,
				Msg:  "标题或内容不能为空: " + err.Error(),
			}
		}
		// 发布内容
		content := entity.PublishImageContent{
			Title:   req.Title,
			Content: req.Content,
		}
		paths := req.ImagePaths
		if paths != nil && len(paths) > 0 {
			Images := []string{}

			for index, path_ := range paths {
				fmt.Printf("Index: %d, Path: %s\n", index, path_)

				u, _ := url.Parse(path_)
				filename := path.Base(u.Path)

				saveDir := os.Getenv("STATIC_XHS_DIR")
				if saveDir == "" {
					saveDir = "./static/xhs/"
				}
				// 确保目录存在
				if err := os.MkdirAll(saveDir, 0755); err != nil {
					panic(err)
				}
				savePath := saveDir + filename
				image, local_image_err := downloadImage(path_, savePath)
				if local_image_err != nil {
					l.Kill()
					return &Response{
						Code: 500,
						Msg:  "本地保存失败: " + local_image_err.Error(),
					}
				}
				Images = append(Images, image)
			}

			content.ImagePaths = Images
		}
		if _, err := action.PublishArticle(ctx, &content); err != nil {
			slog.Error("发布失败", "error", err)
			l.Kill()
			return &Response{
				Code: 500,
				Msg:  "发布失败: " + err.Error(),
			}
		}

		// ===========================
		// 👉 刷新 → 进入我的 → 点第一篇 → 获取详情 URL
		// ===========================
		slog.Info("开始获取最新的推文信息")

		time.Sleep(2 * time.Second)
		// 导航到首页并等待加载完成
		shouye_err := page.Navigate("https://www.xiaohongshu.com/")
		if shouye_err != nil {
			slog.Error("跳转首页失败", "err", err)
			l.Kill()
			return &Response{
				Code: 501,
				Msg:  "跳转首页失败: " + shouye_err.Error(),
			}
		}

		// 等待页面加载完成
		page.MustWaitLoad()
		slog.Info("获取首页.......")
		// 设置视口大小
		page.MustSetViewport(1200, 800, 1, false)

		// 添加短暂延时确保页面稳定
		time.Sleep(2 * time.Second)

		// 刷新页面并等待加载
		page.MustReload().MustWaitLoad()

		//加载cookies
		loadCookies(ctx, page, userID)

		// 找到「我的」tab 并点击
		//page.MustElement("li.user.side-bar-component span.channel").MustClick()
		myTab := page.MustElement("li.user.side-bar-component span.channel")

		if myTab == nil {
			slog.Info("未找到「我的」tab")
			l.Kill()
			return &Response{
				Code: 501,
				Msg:  "未找到「我」tab",
			}
		}
		//点击wo
		myTab.MustClick()

		// 等待页面导航到用户页面
		page.MustWaitLoad()
		time.Sleep(5 * time.Second) // 等待内容加载

		// 等待列表渲染第一篇推文
		// 定位第一篇笔记，设置超时时间避免无限等待
		slog.Info("开始查找第一篇笔记...")
		note, err := page.Timeout(5 * time.Minute).Element("section.note-item[data-index='0']")
		if err != nil {
			slog.Error("查找第一篇笔记超时", "error", err)
			// 尝试其他选择器
			note, err = page.Timeout(5 * time.Second).Element("section.note-item")
			if err != nil {
				l.Kill()
				slog.Error("使用备用选择器也未找到笔记", "error", err)
				return &Response{
					Code: 501,
					Msg:  "未找到第一篇笔记: " + err.Error(),
				}
			}
		}

		if note == nil {
			slog.Info("未找到第一篇笔记")
			l.Kill()
			return &Response{
				Code: 501,
				Msg:  "未找到第一篇笔记",
			}
		}
		// 获取链接
		cover, err := note.Timeout(5 * time.Second).Element("a.cover")

		if err != nil || cover == nil {
			slog.Info("未找到笔记封面链接", "error", err)
			l.Kill()
			return &Response{
				Code: 501,
				Msg:  "未找到笔记封面链接",
			}
		}
		hrefProp, href_error := cover.Property("href")

		if href_error != nil {
			slog.Info("获取笔记链接失败", "err", href_error)
			l.Kill()
			return &Response{
				Code: 501,
				Msg:  "获取笔记链接失败: " + href_error.Error(),
			}
		}
		detailURL := hrefProp.String()

		// 获取标题
		titleEl, title_error := note.Element("div.footer a.title span")

		if title_error != nil {
			slog.Info("未找到笔记标题")
			l.Kill()
			return &Response{
				Code: 501,
				Msg:  "未找到笔记标题: " + title_error.Error(),
			}
		}
		detailTitle := titleEl.MustText()

		if detailTitle == "" {
			slog.Info("获取笔记标题失败")
			l.Kill()
			return &Response{
				Code: 501,
				Msg:  "获取笔记标题失败",
			}
		}

		slog.Info("小红书详情页 URL", "url", detailURL)
		slog.Info("小红书详情页 标题", "title", detailTitle)

		request := service.ThirdRequest{}
		request.Introduction = &detailTitle
		request.UserId = &userID
		request.Url = &detailURL
		_, response_err := p.DomainSVC.SaveTweetUrl(ctx, &request)
		if response_err != nil {
			l.Kill()
			return &Response{
				Code: 502,
				Msg:  "保存到数据库失败: " + response_err.Error(),
			}
		}
		return &Response{
			Code: 200,
			Msg:  "发布成功",
		}

	}

	res := try()

	// 在 try 函数执行完成后释放资源
	if l != nil {
		l.Kill()
	}

	if res.Code == 200 {
		resp.Data = "发布成功"
		return &resp, nil
	} else if res.Code == 501 {
		resp.Code = 1
		resp.Message = "error"
		resp.Data = "获取笔记失败"
		return &resp, nil
	} else if res.Code == 502 {
		resp.Code = 1
		resp.Message = "error"
		resp.Data = "数据保存到数据库失败"
		return &resp, nil
	} else if res.Code == 503 {
		resp.Code = 1
		resp.Message = "error"
		resp.Data = "跳转发布页面失败或今日发布已达到上限"
		return &resp, nil
	} else {
		resp.Code = 1
		resp.Message = "error"
		resp.Data = "发布失败"
		return &resp, nil
	}
}

// 修改推文url
func (p *PublishThirdApplicationService) UpdateTweetUrl(ctx context.Context, req *publishThird.GetThirdUrlRequest) (*publishThird.PublishThirdResponse[string], error) {
	resp := publishThird.PublishThirdResponse[string]{
		Code:    0,
		Message: "ok",
	}
	request := service.ThirdRequest{}
	request.Id = req.Id
	request.LikeCount = req.LikeCount
	request.CollectCount = req.CollectCount
	request.ChatCount = req.ChatCount
	response, response_err := p.DomainSVC.UpdateTweetUrlById(ctx, &request)
	if response_err != nil {
		resp.Code = 1
		resp.Message = "error"
		return &resp, nil
	}
	if response.Msg != "ok" {
		resp.Code = 1
		resp.Message = response.Msg
		return &resp, nil
	}
	resp.Data = "修改成功"
	return &resp, nil
}

// 保存推文url列表
func (p *PublishThirdApplicationService) SaveTweetUrl(ctx context.Context, req *publishThird.GetThirdUrlRequest) (*publishThird.PublishThirdResponse[string], error) {
	resp := publishThird.PublishThirdResponse[string]{
		Code:    0,
		Message: "ok",
	}
	request := service.ThirdRequest{}
	var userID string
	if req.UserId != nil {
		userID = *req.UserId
	} else {
		return nil, errorx.New(errno.ErrUserAuthenticationFailed, errorx.KV("reason", "missing session_key in cookie"))
	}
	request.UserId = &userID
	request.Introduction = req.Introduction
	request.Url = req.Url
	response, response_err := p.DomainSVC.SaveTweetUrl(ctx, &request)
	if response_err != nil {
		resp.Code = 1
		resp.Message = "error"
		return &resp, nil
	}
	if response.Msg != "ok" {
		resp.Code = 1
		resp.Message = response.Msg
		return &resp, nil
	}
	resp.Data = "保存成功"
	return &resp, nil
}

// 获取推文url列表
func (p *PublishThirdApplicationService) GetTweetUrlList(ctx context.Context, req *publishThird.GetThirdUrlRequest) (*publishThird.PublishThirdResponse[[]*publishThird.PublishThirdUrl], error) {
	resp := publishThird.PublishThirdResponse[[]*publishThird.PublishThirdUrl]{
		Code:    0,
		Message: "ok",
	}
	request := service.ThirdRequest{}
	var userID string
	if req.UserId != nil {
		userID = *req.UserId
	} else {
		return nil, errorx.New(errno.ErrUserAuthenticationFailed, errorx.KV("reason", "missing session_key in cookie"))
	}
	request.UserId = &userID
	request.Order = req.Order
	request.Status = req.Status
	request.UrlType = req.UrlType
	if req.Introduction != nil {
		request.Introduction = req.Introduction
	}
	page := 1
	pageSize := 10
	if req.Page != nil && *req.Page > 0 {
		page = int(*req.Page)
	}
	if req.PageSize != nil && *req.PageSize > 0 {
		pageSize = int(*req.PageSize)
	}
	request.Page = &page
	request.PageSize = &pageSize
	response, response_err := p.DomainSVC.GetTweetUrlList(ctx, &request)
	if response_err != nil {
		resp.Code = 1
		resp.Message = "error"
		return &resp, nil
	}
	list := response.PublishThirdList
	thirdUrls := []*publishThird.PublishThirdUrl{}
	for _, item := range list {
		thirdUrl := publishThird.PublishThirdUrl{}
		thirdUrl.Id = item.ID
		thirdUrl.URL = item.Url
		thirdUrl.UrlType = item.UrlType
		thirdUrl.Introduction = item.Introduction
		thirdUrl.Status = item.Status
		thirdUrl.CreatedAt = item.CreatedAt
		thirdUrl.UpdatedAt = item.UpdatedAt
		thirdUrl.CreatorID = item.CreatorID
		thirdUrl.LikeCount = item.LikeCount
		thirdUrl.CollectCount = item.CollectCount
		thirdUrl.ChatCount = item.ChatCount
		thirdUrls = append(thirdUrls, &thirdUrl)
	}
	resp.Data = thirdUrls
	resp.Total = response.Total

	return &resp, nil
}

// 小红书登录
func (p *PublishThirdApplicationService) XhsLogin(ctx context.Context, req *publishThird.GetThirdLoginRequest) (*publishThird.PublishThirdResponse[string], error) {
	resp := publishThird.PublishThirdResponse[string]{
		Code:    0,
		Message: "ok",
	}
	userID := req.UserId
	if req.UserId == "" {
		return nil, errorx.New(errno.ErrUserAuthenticationFailed, errorx.KV("reason", "missing session_key in cookie"))
	}

	manager, l := NewBrowserManager(ctx, userID)
	page := manager.page

	// 检查是否已登录
	if checkLoginStatus(page) {
		if l != nil {
			l.Kill() //释放资源
		}
		resp.Code = 0
		resp.Message = "已登录"
		return &resp, nil
	}

	// 未登录，立即返回
	resp.Code = 1
	resp.Message = "未登录，二维码已生成，请扫码"

	// 异步扫码流程
	safego.Go(context.Background(), func() {
		// 独立上下文，360秒超时
		timeoutCtx, cancel := context.WithTimeout(context.Background(), 360*time.Second)

		defer cancel()

		defer func() {
			fmt.Printf("执行结束。。。。。")
			if l != nil {
				l.Kill()
			}
			if r := recover(); r != nil {
				logs.CtxErrorf(timeoutCtx, "扫码 goroutine panic: %v", r)
			}
		}()

		// 刷新页面
		page.MustReload()
		page.MustWaitLoad()

		// 等待二维码元素出现，示例 Sleep 可替换为 page.Timeout(...).Element(...)
		time.Sleep(2 * time.Second)

		qrBase64, err := scanLogin(page)
		if err != nil {
			logs.CtxErrorf(timeoutCtx, "获取二维码失败: %v", err)
			return
		}

		// 用户 ID 示例
		redisCli := redis.New()
		if qr_err := redisCli.Set(timeoutCtx, userID, qrBase64, 5*time.Minute).Err(); qr_err != nil {
			logs.CtxErrorf(timeoutCtx, "保存二维码失败: %v", qr_err)
		}

		// 等待扫码登录，最长 60 秒
		if waitForLogin(page, 120*time.Second) {
			logs.CtxInfof(timeoutCtx, "扫码登录成功")
			if scanqr_err := redisCli.Set(timeoutCtx, userID, "扫码登录成功", 5*time.Minute).Err(); scanqr_err != nil {
				logs.CtxErrorf(timeoutCtx, "保存登录成功状态失败: %v", scanqr_err)
			}
			//把cookies放在redis里面,有效期24小时
			//saveCookies(page)
			cookies, _ := page.Cookies([]string{})
			data, _ := json.Marshal(cookies)
			//os.WriteFile(cookieFile, data, 0644)
			cookie_key := "Cookies" + userID
			if cookies_err := redisCli.Set(timeoutCtx, cookie_key, data, 24*time.Hour).Err(); cookies_err != nil {
				logs.CtxErrorf(timeoutCtx, "保存cookies失败: %v", cookies_err)
			} else {
				log.Println("Cookie 已保存到", cookieFile)
			}
		} else {
			logs.CtxWarnf(timeoutCtx, "扫码登录超时")
			if err := redisCli.Set(timeoutCtx, userID, "扫码登录超时", 5*time.Minute).Err(); err != nil {
				logs.CtxErrorf(timeoutCtx, "保存扫码超时状态失败: %v", err)
			}
		}
	})

	return &resp, nil
}

// 获取小红书二维码
func (p *PublishThirdApplicationService) GetXhsLoginQr(ctx context.Context, req *publishThird.GetThirdLoginRequest) (*publishThird.PublishThirdResponse[string], error) {
	resp := publishThird.PublishThirdResponse[string]{
		Code:    0,
		Message: "ok",
	}
	userId := req.UserId

	cmdable := redis.New()
	val, redis_err := cmdable.Get(ctx, userId).Result()
	if redis_err != nil {
		if redis_err.Error() == "redis: nil" {
			resp.Message = "二维码已失效"
		}
		resp.Message = "获取二维码失败"
	}
	resp.Data = val
	//获取之后删除
	cmdable.Del(ctx, userId)
	return &resp, nil
}

// getInfo 获取点赞量、收藏量、评论量
func (p *PublishThirdApplicationService) GetTweetInfo(ctx context.Context, req publishThird.GetTweetXHSRequest) (*publishThird.PublishThirdResponse[string], error) {
	resp := publishThird.PublishThirdResponse[string]{
		Code:    0,
		Message: "ok",
	}
	userID := req.UserId
	if req.UserId == "" {
		return nil, errorx.New(errno.ErrUserAuthenticationFailed, errorx.KV("reason", "missing session_key in cookie"))
	}

	manager, l := NewBrowserManager(ctx, userID)
	if !manager.isLogin {
		resp.Code = 2
		resp.Message = "error"
		resp.Data = "请先登录小红书"
		return &resp, nil
	}
	browser := manager.browser
	defer l.Kill() //确保资源释放
	defer browser.MustClose()

	ids := []string{}
	ids = req.Data
	if len(ids) == 0 {
		return &resp, nil
	}
	// 使用原来的 ctx，不要重新创建
	for _, id := range ids {
		request := service.ThirdRequest{}
		url_id, err := strconv.ParseInt(id, 10, 64)
		request.Id = &url_id
		response, err := p.DomainSVC.GetTweetUrlById(ctx, &request)
		if err != nil {
			return nil, err
		}
		if response.Code != 0 {
			return nil, err
		}
		ThirdUrl := response.PublishThirdList[0]
		link := ThirdUrl.Url // 避免闭包捕获

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
			likeCount = "0"
		}

		// 获取收藏
		collectCount = notePage.MustElement(".collect-wrapper .count").MustText()

		if collectCount == "" {
			collectCount = "0"
		}

		// 获取评论
		chatCount = notePage.MustElement(".chat-wrapper .count").MustText()
		if chatCount == "" {
			chatCount = "0"
		}

		log.Printf("链接: %s 点赞:%s 收藏:%s 评论:%s", link, likeCount, collectCount, chatCount)
		like_Count, like_err := strconv.ParseInt(likeCount, 10, 64)
		if like_err != nil {
			like_Count = 0
		}
		ThirdUrl.LikeCount = like_Count
		collect_Count, collect_err := strconv.ParseInt(collectCount, 10, 64)
		if collect_err != nil {
			collect_Count = 0
		}
		ThirdUrl.CollectCount = collect_Count
		chat_Count, chat_err := strconv.ParseInt(chatCount, 10, 64)
		if chat_err != nil {
			chat_Count = 0
		}
		ThirdUrl.ChatCount = chat_Count
		up_request := service.ThirdUrlRequest{}
		up_request.PublishThirdUrl = ThirdUrl
		_, err2 := p.DomainSVC.UpdateTweetUrl(ctx, &up_request)
		if err2 != nil {
			resp.Code = 1
			resp.Message = "error"
			resp.Data = "获取详细信息失败,请稍后重试"
			break
		}
	}
	return &resp, nil
}

// 重置浏览器
func resetBrowser(ctx context.Context, c *app.RequestContext) {
	browserManager.mu.Lock()

	try := func() *Response {
		// 关闭当前浏览器
		if browserManager.browser != nil {
			browserManager.browser.MustClose()
		}

		// 重新创建浏览器实例
		browser, l := newBrowser(true)
		page := browser.MustPage("https://www.xiaohongshu.com/")
		page.MustWaitLoad()

		browserManager.browser = browser
		browserManager.page = page
		browserManager.isLogin = false
		defer l.Kill()
		defer browserManager.mu.Unlock()
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
func newBrowser(headless bool) (*rod.Browser, *launcher.Launcher) {
	l := launcher.New().Headless(headless)
	rodPath := os.Getenv("ROD_BROWSER_PATH")
	var ll string
	if rodPath != "" {
		ll = l.NoSandbox(true).Bin(rodPath).MustLaunch()
	} else {
		ll = l.NoSandbox(true).MustLaunch()
	}
	//ll = l.NoSandbox(true).Bin(os.Getenv("ROD_BROWSER_PATH")).MustLaunch()
	//l := launcher.New().Headless(headless).NoSandbox(true).MustLaunch()
	//defer l.Kill() //确保释放资源
	browser := rod.New().ControlURL(ll).MustConnect()

	return browser, l
}

// saveCookies 保存 Cookie
func saveCookies(page *rod.Page) {
	cookies, _ := page.Cookies([]string{})
	data, _ := json.Marshal(cookies)
	os.WriteFile(cookieFile, data, 0644)
	log.Println("Cookie 已保存到", cookieFile)
}

// loadCookies 加载 Cookie
func loadCookies(ctx context.Context, page *rod.Page, key string) bool {
	//data, err := os.ReadFile(cookieFile)
	cookie_key := "Cookies" + key
	cmdable := redis.New()
	data, redis_err := cmdable.Get(ctx, cookie_key).Bytes()
	if redis_err != nil {
		if redis_err.Error() == "redis: nil" {
			log.Println("Cookie 已失效", cookieFile)
			return false
		}
		log.Println("获取Cookie失败", cookieFile)
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
	page.WaitLoad()

	// ✅ 优先检查未登录按钮
	if el, _ := page.Timeout(3*time.Second).ElementR("button, a", "登录"); el != nil {
		log.Println("检测到登录按钮 → 未登录")
		return false
	}

	// ✅ 检查已登录标志：侧边栏“我”
	if el, _ := page.Timeout(3*time.Second).ElementR("li.user.side-bar-component span.channel", ".*我.*"); el != nil {
		log.Println("检测到侧边栏‘我’ → 已登录")
		return true
	}

	// ✅ 检查头像节点
	if el, _ := page.Timeout(3 * time.Second).Element("img.avatar, .user-avatar"); el != nil {
		log.Println("检测到头像 → 已登录")
		return true
	}

	log.Println("没检测到登录或已登录标志，默认判定为未登录")
	return false
}

// PublishAction 发布动作
type PublishAction struct {
	page *rod.Page
}

const urlOfPublic = `https://creator.xiaohongshu.com/publish/publish?source=official`

// NewPublishImageAction 创建发布动作
func NewPublishImageAction(page *rod.Page) (action *PublishAction, err error) {
	pp := page.Timeout(60 * time.Second)
	pp.MustNavigate(urlOfPublic)
	defer func() {
		if r := recover(); r != nil {
			// 你可以在这里做更复杂的解析，比如根据 r 判断是不是 "element not found" 等
			err = fmt.Errorf("panic captured: %v", r)
		}
	}()

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
func (p *PublishAction) PublishArticle(ctx context.Context, content *entity.PublishImageContent) (string, error) {
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

type ListTweetInfoResponse struct {
	DateList []*publishThird.NoteInfo `thrift:"dataset_list,1" form:"dataset_list" json:"dataset_list" query:"dataset_list"`
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
