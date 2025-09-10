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

package openaiTextToImage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/coze-dev/coze-studio/backend/infra/contract/document/textToImage"
	"github.com/coze-dev/coze-studio/backend/pkg/logs"
	"io"
	"net/http"
	"time"
)

// OpenAI DALL-E API 请求结构
type ImageGenerationRequest struct {
	Prompt         string `json:"prompt"`
	N              int    `json:"n,omitempty"`               // 生成图片数量，默认1
	Size           string `json:"size,omitempty"`            // 图片尺寸: "1024x1024", "1792x1024", "1024x1792"
	Quality        string `json:"quality,omitempty"`         // 图片质量: "standard" 或 "hd"
	ResponseFormat string `json:"response_format,omitempty"` // 响应格式: "url" 或 "b64_json"
	Style          string `json:"style,omitempty"`           // 图片风格: "vivid" 或 "natural"
	User           string `json:"user,omitempty"`            // 用户标识
	Model          string `json:"model,omitempty"`           // 模型，默认"dall-e-3"
}

// OpenAI API 响应结构
type ImageGenerationResponse struct {
	Created int64       `json:"created"`
	Data    []ImageData `json:"data"`
}

type ImageData struct {
	URL           string `json:"url,omitempty"`            // 图片URL
	B64JSON       string `json:"b64_json,omitempty"`       // Base64编码的图片
	RevisedPrompt string `json:"revised_prompt,omitempty"` // 修订后的提示词
}

// 错误响应结构
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

type ErrorDetail struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code"`
}

// OpenAI 客户端
type OpenAIClient struct {
	APIKey  string
	BaseURL string
	Model   string
	Client  *http.Client
}

func NewTextToImage(key string, url string, model string) textToImage.TextToImg {
	client := &openaiTextTOImageImpl{}
	client.OpenAIClient = NewOpenAIClient(key, url, model)
	return client
}

// 创建新的 OpenAI 客户端
func NewOpenAIClient(apiKey string, baseurl string, model string) *OpenAIClient {
	return &OpenAIClient{
		APIKey:  apiKey,
		BaseURL: baseurl,
		Model:   model,
		Client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}
func (c *openaiTextTOImageImpl) GenerateImage(ctx context.Context, prompt string) (string, error) {
	req := &ImageGenerationRequest{
		Prompt:  prompt,
		Size:    "1792x1024",
		Quality: "hd",
		Style:   "vivid",
		N:       1,
	}
	resp, eror := c.GenerateImageRequest(ctx, req)
	if eror != nil {
		logs.CtxErrorf(ctx, "text to Image failed %v", eror)
		return "", eror
	}
	imgData := resp.Data[0]

	return imgData.URL, nil
}

// 生成图片
func (c *openaiTextTOImageImpl) GenerateImageRequest(ctx context.Context, req *ImageGenerationRequest) (*ImageGenerationResponse, error) {
	// 设置默认值
	if req.Model == "" {
		req.Model = c.Model
	}
	if req.Size == "" {
		req.Size = "1024x1024"
	}
	if req.Quality == "" {
		req.Quality = "standard"
	}
	if req.ResponseFormat == "" {
		req.ResponseFormat = "url"
	}
	if req.N == 0 {
		req.N = 1
	}

	// 序列化请求体
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	// 创建HTTP请求
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/images/generations", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	// 设置请求头
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)

	// 发送请求
	resp, err := c.Client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("发送请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	// 检查HTTP状态码
	if resp.StatusCode != http.StatusOK {
		var errorResp ErrorResponse
		if err := json.Unmarshal(body, &errorResp); err == nil {
			return nil, fmt.Errorf("API错误 (%d): %s", resp.StatusCode, errorResp.Error.Message)
		}
		return nil, fmt.Errorf("HTTP错误 (%d): %s", resp.StatusCode, string(body))
	}

	// 解析响应
	var imageResp ImageGenerationResponse
	if err := json.Unmarshal(body, &imageResp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	return &imageResp, nil
}

// 兼容其他API的客户端接口
type openaiTextTOImageImpl struct {
	*OpenAIClient
	// 可以扩展支持其他API，如Stability AI, Midjourney等
}

// 批量生成图片
func (c *openaiTextTOImageImpl) GenerateBatch(ctx context.Context, prompts []string, options *ImageGenerationRequest) ([]string, error) {
	var urls []string

	for _, prompt := range prompts {
		req := &ImageGenerationRequest{
			Prompt: prompt,
		}

		// 应用额外选项
		if options != nil {
			if options.Size != "" {
				req.Size = options.Size
			}
			if options.Quality != "" {
				req.Quality = options.Quality
			}
			if options.Style != "" {
				req.Style = options.Style
			}
		}

		resp, err := c.GenerateImageRequest(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("生成图片失败 (提示词: %s): %w", prompt, err)
		}

		if len(resp.Data) > 0 {
			urls = append(urls, resp.Data[0].URL)
		}

		// 添加延迟避免频率限制
		time.Sleep(1 * time.Second)
	}

	return urls, nil
}
