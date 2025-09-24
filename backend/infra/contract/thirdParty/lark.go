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

package thirdParty

import (
	"context"
	larkauthen "github.com/larksuite/oapi-sdk-go/v3/service/authen/v1"
)

type TokenRequest struct {
	GrantType    string `json:"grant_type"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	Code         string `json:"code"`
}
type TokenResponse struct {
	Code             int    `json:"code"`
	Msg              string `json:"error_description"`
	AccessToken      string `json:"access_token"`
	ExpiresIn        int    `json:"expires_in"`
	RefreshToken     string `json:"refresh_token,omitempty"`
	RefreshExpiresIn int    `json:"refresh_token_expires_in,omitempty"`
}

type Lark interface {
	GetUserAccessToken(ctx context.Context, code string) (*TokenResponse, error)

	GetUserInfo(ctx context.Context, userAccessToken string) (*larkauthen.GetUserInfoResp, error)
}
