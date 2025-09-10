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
	"github.com/coze-dev/coze-studio/backend/infra/contract/cache"
	"github.com/coze-dev/coze-studio/backend/infra/contract/storage"
	"github.com/coze-dev/coze-studio/backend/infra/impl/cache/redis"
	storage1 "github.com/coze-dev/coze-studio/backend/infra/impl/storage"
	"github.com/coze-dev/coze-studio/backend/pkg/logs"
	"github.com/coze-dev/coze-studio/backend/pkg/safego"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"io"
	"io/ioutil"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type PublishThirdApplicationService struct {
	DomainSVC   service.PublishThird
	cacheClient cache.Cmdable
	Oss         storage.Storage
}

var PublishThirdApplicationSVC = &PublishThirdApplicationService{}

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
	browser := newBrowser(true)
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

func getContent(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status: %s", resp.Status)
	}

	// 读取到 []byte
	return ioutil.ReadAll(resp.Body)
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
	Manager := NewBrowserManager()
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
		action, err := NewPublishImageAction(Manager.page)
		if err != nil {
			slog.Error("创建发布动作失败", "error", err)
			return &Response{
				Code: 500,
				Msg:  "创建发布动作失败: " + err.Error(),
			}
		}
		if len(req.Title) == 0 && len(req.Content) == 0 {
			slog.Error("标题或内容为空", "error", err)
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
			s, init_err := storage1.New(ctx)
			if init_err != nil {
				return &Response{
					Code: 500,
					Msg:  "初始化storage失败: " + err.Error(),
				}
			}
			for index, path := range paths {
				fmt.Printf("Index: %d, Path: %s\n", index, path)
				//先存到minio里面
				bytes, bytes_err := getContent(path)
				if bytes_err != nil {
					slog.Error("图片获取失败", "error", err)
					return &Response{
						Code: 500,
						Msg:  "图片获取失败: " + err.Error(),
					}
				}
				objKey := "xhs"
				newString := uuid.NewString() + ".jpg"
				objectName := fmt.Sprintf("%s/%s", objKey, newString)
				upload_err := s.PutObject(ctx, objectName, bytes)
				if upload_err != nil {
					return &Response{
						Code: 500,
						Msg:  "图片获取失败: " + err.Error(),
					}
				}
				_, getUrl_err := s.GetObjectUrl(ctx, objectName)
				if getUrl_err != nil {
					return &Response{
						Code: 500,
						Msg:  "图片获取失败: " + err.Error(),
					}
				}

				saveDir := "./static/xhs/"
				// 确保目录存在
				if err := os.MkdirAll(saveDir, 0755); err != nil {
					panic(err)
				}
				savePath := saveDir + newString
				image, local_image_err := downloadImage(path, savePath)
				if local_image_err != nil {
					return &Response{
						Code: 500,
						Msg:  "本地保存失败: " + err.Error(),
					}
				}
				Images = append(Images, image)
			}

			content.ImagePaths = Images
		}
		if _, err := action.PublishArticle(ctx, &content); err != nil {
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

	res := try()
	if res.Code == 200 {
		resp.Data = "发布成功"
		return &resp, nil
	} else {
		resp.Data = "发布失败"
		return &resp, nil
	}
}

// 获取推文url列表
func (p *PublishThirdApplicationService) GetTweetUrlList(ctx context.Context, req *publishThird.GetThirdUrlRequest) (*publishThird.PublishThirdResponse[*publishThird.PublishThirdUrl], error) {
	resp := publishThird.PublishThirdResponse[*publishThird.PublishThirdUrl]{
		Code:    0,
		Message: "ok",
	}
	request := service.ThirdRequest{}
	request.UserId = req.UserId
	request.Order = req.Order
	request.Status = req.Status
	request.UrlType = req.UrlType
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
	p.DomainSVC.GetTweetUrlList(ctx, &request)

	return &resp, nil
}

// 小红书登录
func (p *PublishThirdApplicationService) XhsLogin(ctx context.Context) (*publishThird.PublishThirdResponse[string], error) {
	resp := publishThird.PublishThirdResponse[string]{
		Code:    0,
		Message: "ok",
	}
	manager := NewBrowserManager()
	page := manager.page

	// 检查是否已登录
	if checkLoginStatus(page) {
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
		userID := int64(123456789)
		key := strconv.FormatInt(userID, 10)

		redisCli := redis.New()
		if err := redisCli.Set(timeoutCtx, key, qrBase64, 5*time.Minute).Err(); err != nil {
			logs.CtxErrorf(timeoutCtx, "保存二维码失败: %v", err)
		}

		// 等待扫码登录，最长 60 秒
		if waitForLogin(page, 120*time.Second) {
			logs.CtxInfof(timeoutCtx, "扫码登录成功")
			if err := redisCli.Set(timeoutCtx, key, "扫码登录成功", 5*time.Minute).Err(); err != nil {
				logs.CtxErrorf(timeoutCtx, "保存登录成功状态失败: %v", err)
			}
			saveCookies(page)
		} else {
			logs.CtxWarnf(timeoutCtx, "扫码登录超时")
			if err := redisCli.Set(timeoutCtx, key, "扫码登录超时", 5*time.Minute).Err(); err != nil {
				logs.CtxErrorf(timeoutCtx, "保存扫码超时状态失败: %v", err)
			}
		}
	})

	return &resp, nil
}

// 获取小红书二维码
func (p *PublishThirdApplicationService) GetXhsLoginQr(ctx context.Context) (*publishThird.PublishThirdResponse[string], error) {
	resp := publishThird.PublishThirdResponse[string]{
		Code:    0,
		Message: "ok",
	}
	userID := int64(123456789)
	str := strconv.FormatInt(userID, 10)
	cmdable := redis.New()
	val, redis_err := cmdable.Get(ctx, str).Result()
	if redis_err != nil {
		if redis_err.Error() == "redis: nil" {
			resp.Message = "二维码已失效"
		}
		resp.Message = "获取二维码失败"
	}
	resp.Data = val
	//获取之后删除
	cmdable.Del(ctx, str)
	return &resp, nil
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
	l := launcher.New().Headless(headless).NoSandbox(true).MustLaunch()
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

// getInfo 获取点赞量、收藏量、评论量
func (p *PublishThirdApplicationService) GetTweetInfo(ctx context.Context, req publishThird.GetTweetXHSRequest) (*publishThird.PublishThirdResponse[[]publishThird.NoteInfo], error) {
	manager := NewBrowserManager()
	browser := manager.browser
	defer browser.MustClose()

	links := []string{}
	links = req.Data
	resultsCh := make(chan publishThird.NoteInfo, len(links))
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

			resultsCh <- publishThird.NoteInfo{
				URL:          link,
				LikeCount:    likeCount,
				CollectCount: collectCount,
				ChatCount:    chatCount,
			}
		}()
	}

	resp := publishThird.PublishThirdResponse[[]publishThird.NoteInfo]{
		Code:    0,
		Message: "ok",
	}
	resp.Data = make([]publishThird.NoteInfo, 0)
	for i := 0; i < len(links); i++ {
		select {
		case res := <-resultsCh:
			resp.Data = append(resp.Data, res)
		case err := <-errCh:
			log.Println("抓取错误:", err)
			// 可以选择继续或直接返回错误，这里继续抓取
		case <-ctx.Done():
			return &resp, ctx.Err() // 上下文超时或取消
		}
	}

	return &resp, nil
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
