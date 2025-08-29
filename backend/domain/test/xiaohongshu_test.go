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

package test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/pkg/errors"
	//"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"log"

	"log/slog"
	"os"
	"strings"
	"testing"
	"time"
)

const cookieFile = "cookie.json"

func TestXHP(t *testing.T) {
	// 启动浏览器
	browser := newBrowser(false)
	defer browser.MustClose()

	// 打开小红书首页
	page := browser.MustPage("https://www.xiaohongshu.com/")
	page.MustWaitLoad()

	// 尝试加载 cookie
	if loadCookies(page) {
		log.Println("已加载 cookie，尝试使用登录状态")
	} else {
		log.Println("未找到有效 cookie，需要扫码登录")
		qrCode := scanLogin(page)
		fmt.Println("二维码 Base64 (前端可展示):", qrCode[:100], "...")
		if waitForLogin(page, 60*time.Second) {
			log.Println("扫码登录成功")
			saveCookies(page)
		} else {
			log.Fatal("扫码登录超时")
		}
	}

	// 登录成功后执行后续操作
	//publishNoteExample(page, "Rod 自动化测试笔记", "这是通过 Rod 自动化发布的内容")
	action, err := NewPublishImageAction(page)
	require.NoError(t, err)

	err = action.Publish(context.Background(), PublishImageContent{
		Title:      "Hello World",
		Content:    "Hello World",
		ImagePaths: []string{"./tmp/ceshikele.jpg"},
	})
}

// ---------- 浏览器启动 ----------
func newBrowser(headless bool) *rod.Browser {
	l := launcher.New().Headless(headless).MustLaunch()
	browser := rod.New().ControlURL(l).MustConnect()
	return browser
}

// ---------- Cookie 管理 ----------
func saveCookies(page *rod.Page) {
	cookies, _ := page.Cookies([]string{})
	data, _ := json.Marshal(cookies)
	os.WriteFile(cookieFile, data, 0644)
	log.Println("Cookie 已保存到", cookieFile)
}

func loadCookies(page *rod.Page) bool {
	data, err := os.ReadFile(cookieFile)
	if err != nil {
		return false
	}

	var cookies []*proto.NetworkCookie
	if err := json.Unmarshal(data, &cookies); err != nil {
		return false
	}

	var params []*proto.NetworkCookieParam
	for _, c := range cookies {
		params = append(params, &proto.NetworkCookieParam{
			Name:    c.Name,
			Value:   c.Value,
			Domain:  c.Domain,
			Path:    c.Path,
			Expires: c.Expires,

			Secure:   c.Secure,
			SameSite: c.SameSite,
		})
	}

	page.MustSetCookies(params...)
	page.MustReload()
	return true
}

// ---------- 扫码登录 ----------
func scanLogin(page *rod.Page) string {
	// 截取二维码
	qrEl := page.MustElement(".qrcode") // 修改为真实选择器
	qrPNG, _ := qrEl.Screenshot(proto.PageCaptureScreenshotFormatPng, 0)
	qrBase64 := base64.StdEncoding.EncodeToString(qrPNG)
	return qrBase64
}

// ---------- 等待扫码登录 ----------
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

// PublishImageContent 发布图文内容
type PublishImageContent struct {
	Title      string
	Content    string
	ImagePaths []string
}

type PublishAction struct {
	page *rod.Page
}

const (
	urlOfPublic = `https://creator.xiaohongshu.com/publish/publish?source=official`
)

func NewPublishImageAction(page *rod.Page) (*PublishAction, error) {

	pp := page.Timeout(60 * time.Second)

	pp.MustNavigate(urlOfPublic)

	pp.MustElement(`div.upload-content`).MustWaitVisible()
	slog.Info("wait for upload-content visible success")

	// 等待一段时间确保页面完全加载
	time.Sleep(1 * time.Second)

	createElems := pp.MustElements("div.creator-tab")
	slog.Info("foundcreator-tab elements", "count", len(createElems))
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

	return &PublishAction{
		page: pp,
	}, nil
}

func (p *PublishAction) Publish(ctx context.Context, content PublishImageContent) error {
	if len(content.ImagePaths) == 0 {
		return errors.New("图片不能为空")
	}

	page := p.page.Context(ctx)

	if err := uploadImages(page, content.ImagePaths); err != nil {
		return errors.Wrap(err, "小红书上传图片失败")
	}

	if err := submitPublish(page, content.Title, content.Content); err != nil {
		return errors.Wrap(err, "小红书发布失败")
	}

	return nil
}

func uploadImages(page *rod.Page, imagesPaths []string) error {
	pp := page.Timeout(30 * time.Second)

	// 等待上传输入框出现
	uploadInput := pp.MustElement(".upload-input")

	// 上传多个文件
	uploadInput.MustSetFiles(imagesPaths...)

	// 等待上传完成
	time.Sleep(3 * time.Second)

	return nil
}

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

// 查找内容输入框 - 使用Race方法处理两种样式
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

func findTextboxByPlaceholder(page *rod.Page) (*rod.Element, error) {
	elements := page.MustElements("p")
	if elements == nil {
		return nil, errors.New("no p elements found")
	}

	// 查找包含指定placeholder的元素
	placeholderElem := findPlaceholderElement(elements, "输入正文描述")
	if placeholderElem == nil {
		return nil, errors.New("no placeholder element found")
	}

	// 向上查找textbox父元素
	textboxElem := findTextboxParent(placeholderElem)
	if textboxElem == nil {
		return nil, errors.New("no textbox parent found")
	}

	return textboxElem, nil
}

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
