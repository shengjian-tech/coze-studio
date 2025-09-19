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

package textToImage

import (
	"context"
	"fmt"
	"github.com/coze-dev/coze-studio/backend/infra/contract/document/textToImage"
	"github.com/coze-dev/coze-studio/backend/infra/contract/storage"
	ttImpl "github.com/coze-dev/coze-studio/backend/infra/impl/document/textToImage"
	"github.com/coze-dev/coze-studio/backend/pkg/logs"
	"io"
	"net/http"
	"net/url"
	"path"
)

// 发布请求
type TextToImageRequest struct {
	Prompt     string   `json:"prompt"`
	Content    string   `json:"content"`
	ImagePaths []string `json:"image_paths"`
}

type TextToImageApplicationService struct {
	//DomainSVC   service.PublishThird
	ttImg   textToImage.TextToImg
	storage storage.Storage
}

func InitService(ctx context.Context, image textToImage.TextToImg, storage storage.Storage) *TextToImageApplicationService {
	TextToImageApplicationSVC.ttImg = image
	TextToImageApplicationSVC.storage = storage
	return TextToImageApplicationSVC
}

var TextToImageApplicationSVC = &TextToImageApplicationService{}

// 响应结构体
type GentImageResponse struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data,omitempty"`
}

func (t *TextToImageApplicationService) GentImage(ctx context.Context, req *TextToImageRequest) (*GentImageResponse, error) {
	if req.Prompt == "" {
		return &GentImageResponse{Code: 500, Msg: "请输入提示词", Data: nil}, nil
	}
	img, err := t.ttImg.GenerateImage(ctx, req.Prompt)
	if err != nil {
		logs.CtxErrorf(ctx, "gentrate img failed %v", err)
		return &GentImageResponse{
			Code: 500,
			Msg:  "生成失败",
			Data: nil,
		}, err
	}
	//生成的图片都有过期时间需要及时保存
	if img != "" {
		resp, err := http.Get(img)
		if err != nil || resp.StatusCode != http.StatusOK {
			logs.CtxErrorf(ctx, "状态码：%v下载图片失败！:%v", resp.StatusCode, err)
			return &GentImageResponse{
				Code: 500,
				Msg:  "图片获取失败",
				Data: nil,
			}, err
		}
		defer resp.Body.Close()
		// 读取二进制内容
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			logs.CtxErrorf(ctx, "读取图片失败:%v\n", err)
			return &GentImageResponse{
				Code: 500,
				Msg:  "读取图片失败",
				Data: nil,
			}, err
		}
		//获取文件名
		u, _ := url.Parse(img)
		filename := path.Base(u.Path)
		objectName := fmt.Sprintf("%s/%s", ttImpl.TextToImagePrefix, filename)
		//保存到minio中
		err = t.storage.PutObject(ctx, objectName, data)
		if err != nil {
			logs.CtxErrorf(ctx, "图片保存失败：%v", err)
		}

		url, err := t.storage.GetObjectUrl(ctx, objectName)
		if err != nil {
			logs.CtxErrorf(ctx, "获取图片url失败:%v", err)
			return &GentImageResponse{
				Code: 500,
				Msg:  "获取图片Url失败",
				Data: nil,
			}, err
		}

		resp1 := &GentImageResponse{
			Code: 0,
			Msg:  "生成图片成功",
			Data: url,
		}
		return resp1, nil

	} else {
		return &GentImageResponse{
			Code: 0,
			Msg:  "生成图片失败",
			Data: "",
		}, nil
	}

}
