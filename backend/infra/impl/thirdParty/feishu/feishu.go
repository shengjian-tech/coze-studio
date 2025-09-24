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

package feishu

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/coze-dev/coze-studio/backend/infra/contract/thirdParty"
	"github.com/coze-dev/coze-studio/backend/pkg/logs"
	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkauth "github.com/larksuite/oapi-sdk-go/v3/service/auth/v3"
	larkauthen "github.com/larksuite/oapi-sdk-go/v3/service/authen/v1"
	"net/http"
	"os"
)

type LarkCli struct {
	client *lark.Client

	AppId     string
	AppSercet string
}

//var FeiShuAppPrifex = fmt.Sprintf("%s_feishu_app_access_token", os.Getenv("FEISHU_APP_ID"))

// 创建client
func NewClient() thirdParty.Lark {
	appId := os.Getenv("FEISHU_APP_ID")
	appSercet := os.Getenv("FEISHU_APP_SECRET")

	//return lark.NewClient(appId, appSercet)
	client1 := lark.NewClient(appId, appSercet)

	return &LarkCli{client: client1, AppId: appId, AppSercet: appSercet}
}

// 获取app_access_token
func (l *LarkCli) GetAppAccessToken(ctx context.Context) ([]byte, error) {
	var appToekn []byte = make([]byte, 0)
	// 创建请求对象
	req := larkauth.NewInternalAppAccessTokenReqBuilder().
		Body(larkauth.NewInternalAppAccessTokenReqBodyBuilder().
			AppId(l.AppId).
			AppSecret(l.AppSercet).
			Build()).
		Build()

	// 发起请求
	resp, err := l.client.Auth.V3.AppAccessToken.Internal(ctx, req)

	// 处理错误
	if err != nil {
		logs.CtxErrorf(ctx, "飞书构建请求失败:%v", err)
		return appToekn, fmt.Errorf(" feishu server errors:[error] ,%w ", err)
	}

	// 服务端错误处理
	if !resp.Success() {
		logs.CtxErrorf(ctx, "logId: %s, error response: \n%s", resp.RequestId(), larkcore.Prettify(resp.CodeError))
		return appToekn, fmt.Errorf(" feishu server errors:[error] ,%w ", err)
	}

	// 保存到缓存中
	appToekn = resp.ApiResp.RawBody
	//l.redisCli.Set(ctx, FeiShuAppPrifex, appToekn, time.Hour*1+time.Minute*50)

	return appToekn, nil

}

// 获取用户访问凭证(官方sdk由于不支持user_access_token,需要自己去托管)
func (l *LarkCli) GetUserAccessToken(ctx context.Context, code string) (*thirdParty.TokenResponse, error) {
	url := "https://open.feishu.cn/open-apis/authen/v2/oauth/token"

	reqBody := thirdParty.TokenRequest{
		GrantType:    "authorization_code",
		ClientID:     l.AppId,
		ClientSecret: l.AppSercet,
		Code:         code,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		logs.CtxErrorf(ctx, "json user_access_token failed: %v", err)
		return nil, err
	}
	cli := &http.Client{}

	request, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		logs.CtxErrorf(ctx, "build user_access_token failed :%v", err)
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")

	resp, err := cli.Do(request)
	if err != nil {
		logs.CtxErrorf(ctx, "user_access_token request feishu failed :%v", err)
	}

	defer resp.Body.Close()

	var tokenResp thirdParty.TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, err
	}

	if tokenResp.Code != 0 {
		return nil, fmt.Errorf("failed to get token: code=%d, msg=%s", tokenResp.Code, tokenResp.Msg)
	}

	return &tokenResp, nil

}

// 获取用户信息
func (l *LarkCli) GetUserInfo(ctx context.Context, userAccessToken string) (*larkauthen.GetUserInfoResp, error) {

	// 发起请求
	resp, err := l.client.Authen.V1.UserInfo.Get(ctx, larkcore.WithUserAccessToken(userAccessToken))

	// 处理错误
	if err != nil {
		logs.CtxErrorf(ctx, "build get feishu userInfo failed: %v", err)
		fmt.Println(err)
		return nil, err
	}

	// 服务端错误处理
	if !resp.Success() {
		logs.CtxErrorf(ctx, "logId: %s, error response: \n%s", resp.RequestId(), larkcore.Prettify(resp.CodeError))

		return nil, fmt.Errorf("logId: %s, error response: \n%s", resp.RequestId(), larkcore.Prettify(resp.CodeError))
	}

	// 业务处理
	return resp, nil

}
