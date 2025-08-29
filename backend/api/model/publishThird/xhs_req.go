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
	"github.com/coze-dev/coze-studio/backend/api/model/base"

	publishThird_commion "github.com/coze-dev/coze-studio/backend/api/model/publishThird/commion"
	"github.com/coze-dev/coze-studio/backend/domain/publishThird/entiy"
)

type GetXHSRequest struct {
	ImageContent *entiy.PublishImageContent             `thrift:"ImageContent,1,optional" json:"entity_type,omitempty"`
	PublishType  *publishThird_commion.PublishThirdType `thrift:"PublishType,2,required" json:"publish_type,omitempty"`
}

func NewGetXHSRequest() *GetXHSRequest {
	return &GetXHSRequest{}
}

// vo
type XHSData struct {
	LoginQr    string `thift:"LoginQr,1,optional" json:"login_qr,omitempty"`
	PublishUrl string `thrift:"PublishUrl,2,optional"  json:"publish_url"`
}
type WxData struct {
	Content string `thift:"LoginQr,1,optional" json:"content,omitempty"`
}

// resp
type PublishThirdResponse[T any] struct {
	Code     int32          `thrift:"Code,1,required" form:"code,required" json:"code,required"`
	Message  string         `thrift:"Message,2,required" form:"message,required" json:"message,required"`
	Data     T              `thrift:"Data,3,optional" json:"data,omitempty"` //也许返回小红书也许返回微信公众号
	BaseResp *base.BaseResp `thrift:"BaseResp,255,optional" form:"BaseResp" json:"BaseResp,omitempty" query:"BaseResp"`
}

func (p *GetXHSRequest) GetPublishType() (v publishThird_commion.PublishThirdType) {
	//if !p.PublishType() {
	//	return GetProductListRequest_EntityType_DEFAULT
	//}
	//return *p.EntityType

	if p.PublishType == nil {
		return publishThird_commion.PublishThirdType_XSH
	}
	return *p.PublishType
}
