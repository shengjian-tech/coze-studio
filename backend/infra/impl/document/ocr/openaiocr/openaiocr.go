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

package openaiocr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/coze-dev/coze-studio/backend/infra/contract/document/ocr"
	"github.com/coze-dev/coze-studio/backend/pkg/errorx"
	"github.com/coze-dev/coze-studio/backend/types/errno"
)

type Config struct {
	Client      *http.Client
	URL         string
	APIKey      string
	Model       string
	MaxTokens   *int     // OpenAI特定参数：最大token数
	Temperature *float64 // OpenAI特定参数：温度值
	Language    *string  // 指定OCR识别的语言，如"中文"、"英文"等
	Prompt      *string  // 自定义OCR提示词

}

func NewOCR(config *Config) ocr.OCR {
	// 设置默认值
	if config.URL == "" {
		config.URL = "https://api.openai.com/v1/chat/completions"
	}
	if config.Model == "" {
		config.Model = "gpt-4o"
	}
	if config.MaxTokens == nil {
		maxTokens := 4000
		config.MaxTokens = &maxTokens
	}
	if config.Temperature == nil {
		temperature := 0.0
		config.Temperature = &temperature
	}
	if config.Language == nil {
		language := "中文"
		config.Language = &language
	}

	return &openaiocrImpl{config}
}

type openaiocrImpl struct {
	config *Config
}

// OpenAI API请求结构体
type openAIRequest struct {
	Model       string    `json:"model"`
	Messages    []message `json:"messages"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Temperature float64   `json:"temperature,omitempty"`
}

type message struct {
	Role    string    `json:"role"`
	Content []content `json:"content"`
}

type content struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *imageURL `json:"image_url,omitempty"`
}

type imageURL struct {
	URL string `json:"url"`
}

// OpenAI API响应结构体
type openAIResponse struct {
	Choices []choice  `json:"choices"`
	Error   *apiError `json:"error,omitempty"`
}

type choice struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
}

type apiError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code"`
}

func (o *openaiocrImpl) FromBase64(ctx context.Context, b64 string) ([]string, error) {
	fmt.Println("openai_OCR FromBase64,进入了 ***************************")
	imageURL := fmt.Sprintf("data:image/jpeg;base64,%s", b64)
	return o.makeRequest(ctx, imageURL)
}

func (o *openaiocrImpl) FromURL(ctx context.Context, url string) ([]string, error) {
	return o.makeRequest(ctx, url)
}

func (o *openaiocrImpl) makeRequest(ctx context.Context, imageurl string) ([]string, error) {
	// 构建OCR提示词
	prompt := o.buildOCRPrompt()

	// 构建请求体
	reqBody := openAIRequest{
		Model: o.config.Model,
		Messages: []message{
			{
				Role: "user",
				Content: []content{
					{
						Type: "text",
						Text: prompt,
					},
					{
						Type: "image_url",
						ImageURL: &imageURL{
							URL: imageurl,
						},
					},
				},
			},
		},
		MaxTokens:   *o.config.MaxTokens,
		Temperature: *o.config.Temperature,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		fmt.Printf("openai_ocr_error1////////////////////////////////////////////")
		return nil, errorx.WrapByCode(err, errno.ErrKnowledgeNonRetryableCode)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", o.config.URL, bytes.NewReader(bodyBytes))
	if err != nil {
		fmt.Printf("openai_ocr_error2////////////////////////////////////////////")
		return nil, errorx.WrapByCode(err, errno.ErrKnowledgeNonRetryableCode)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", o.config.APIKey))
	fmt.Println("openai_OCR_body:", reqBody)

	resp, err := o.config.Client.Do(req)
	if err != nil {
		fmt.Printf("openai_ocr_error3////////////////////////////////////////////")

		return nil, errorx.WrapByCode(err, errno.ErrKnowledgeNonRetryableCode)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, errorx.WrapByCode(
			fmt.Errorf("OpenAI API请求失败，状态码: %d, 响应: %s", resp.StatusCode, string(respBody)),
			errno.ErrKnowledgeNonRetryableCode,
		)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("openai_ocr_error4////////////////////////////////////////////")

		return nil, errorx.WrapByCode(err, errno.ErrKnowledgeNonRetryableCode)
	}

	var res openAIResponse
	if err := json.Unmarshal(respBody, &res); err != nil {
		fmt.Printf("openai_ocr_error5////////////////////////////////////////////")

		return nil, errorx.WrapByCode(err, errno.ErrKnowledgeNonRetryableCode)
	}

	// 检查API错误
	if res.Error != nil {
		return nil, errorx.WrapByCode(
			fmt.Errorf("OpenAI API错误: %s", res.Error.Message),
			errno.ErrKnowledgeNonRetryableCode,
		)
	}

	// 检查响应
	if len(res.Choices) == 0 {
		return nil, errorx.WrapByCode(
			fmt.Errorf("OpenAI API没有返回有效响应"),
			errno.ErrKnowledgeNonRetryableCode,
		)
	}

	// 解析OCR结果
	return o.parseOCRResult(res.Choices[0].Message.Content), nil
}

// 构建OCR提示词
func (o *openaiocrImpl) buildOCRPrompt() string {
	// 如果有自定义提示词，直接使用
	if o.config.Prompt != nil && *o.config.Prompt != "" {
		return *o.config.Prompt
	}

	// 否则构建默认提示词
	language := "中文"
	if o.config.Language != nil {
		language = *o.config.Language
	}

	prompt := fmt.Sprintf(`请对这张图片进行OCR文字识别，要求：
1. 识别图片中的所有文字内容
2. 保持原有的文字顺序和布局
3. 每行文字单独输出
4. 使用%s进行输出
5. 只输出识别到的文字内容，不要添加任何解释或描述
6. 如果图片中没有文字，请输出"无文字内容"

请按行输出识别结果：`, language)

	return prompt
}

// 解析OCR结果
func (o *openaiocrImpl) parseOCRResult(content string) []string {
	// 去除前后空白字符
	content = strings.TrimSpace(content)

	if content == "" || content == "无文字内容" {
		return []string{}
	}

	// 按行分割文字内容
	lines := strings.Split(content, "\n")
	var results []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			// 过滤掉可能的解释性文字
			if !o.isExplanationText(line) {
				results = append(results, line)
			}
		}
	}
	fmt.Println("OCR结果:", content)

	return results
}

// 判断是否为解释性文字
func (o *openaiocrImpl) isExplanationText(text string) bool {
	explanationPrefixes := []string{
		"识别结果：",
		"识别到的文字：",
		"OCR结果：",
		"图片中的文字：",
		"文字内容：",
		"以下是识别结果：",
		"图片文字识别结果：",
	}

	for _, prefix := range explanationPrefixes {
		if strings.HasPrefix(text, prefix) {
			return true
		}
	}

	return false
}

// 辅助方法：设置自定义提示词
func (o *openaiocrImpl) SetCustomPrompt(prompt string) {
	o.config.Prompt = &prompt
}

// 辅助方法：设置识别语言
func (o *openaiocrImpl) SetLanguage(language string) {
	o.config.Language = &language
}

// 辅助方法：获取配置信息
func (o *openaiocrImpl) GetConfig() *Config {
	return o.config
}

// 批量OCR处理（如果需要）
func (o *openaiocrImpl) BatchFromBase64(ctx context.Context, base64List []string) ([][]string, error) {
	var results [][]string

	for i, b64 := range base64List {
		result, err := o.FromBase64(ctx, b64)
		if err != nil {
			return nil, errorx.WrapByCode(
				fmt.Errorf("批量处理第%d个图片失败: %v", i+1, err),
				errno.ErrKnowledgeNonRetryableCode,
			)
		}
		results = append(results, result)
	}

	return results, nil
}

// 批量URL处理
func (o *openaiocrImpl) BatchFromURL(ctx context.Context, urlList []string) ([][]string, error) {
	var results [][]string

	for i, url := range urlList {
		result, err := o.FromURL(ctx, url)
		if err != nil {
			return nil, errorx.WrapByCode(
				fmt.Errorf("批量处理第%d个URL失败: %v", i+1, err),
				errno.ErrKnowledgeNonRetryableCode,
			)
		}
		results = append(results, result)
	}

	return results, nil
}
