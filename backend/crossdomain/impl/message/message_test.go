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

package message

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-studio/backend/api/model/crossdomain/message"
	crossmessage "github.com/coze-dev/coze-studio/backend/crossdomain/contract/message"
	"github.com/coze-dev/coze-studio/backend/domain/conversation/message/entity"
	"github.com/coze-dev/coze-studio/backend/domain/workflow"
	"github.com/coze-dev/coze-studio/backend/infra/contract/storage"
	"github.com/coze-dev/coze-studio/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-studio/backend/pkg/sonic"
)

type mockWorkflowRepo struct {
	workflow.Repository
}

func (m *mockWorkflowRepo) GetObjectUrl(ctx context.Context, uri string, opts ...storage.GetOptFn) (string, error) {
	return uri, nil
}

func Test_convertToConvAndSchemaMessage(t *testing.T) {
	workflow.SetRepository(&mockWorkflowRepo{})

	sm1, err := sonic.MarshalString(&schema.Message{Content: "hello"})
	require.NoError(t, err)

	sm2, err := sonic.MarshalString(&schema.Message{MultiContent: []schema.ChatMessagePart{{Type: schema.ChatMessagePartTypeFileURL, FileURL: &schema.ChatMessageFileURL{URI: "f_uri_1"}}}})
	require.NoError(t, err)

	sm3, err := sonic.MarshalString(&schema.Message{MultiContent: []schema.ChatMessagePart{{Type: schema.ChatMessagePartTypeText, Text: "hello"}, {Type: schema.ChatMessagePartTypeFileURL, FileURL: &schema.ChatMessageFileURL{URI: "f_uri_2"}}}})
	require.NoError(t, err)

	sm4, err := sonic.MarshalString(&schema.Message{MultiContent: []schema.ChatMessagePart{{Type: schema.ChatMessagePartTypeFileURL, FileURL: &schema.ChatMessageFileURL{URI: "f_uri_3"}}, {Type: schema.ChatMessagePartTypeFileURL, FileURL: &schema.ChatMessageFileURL{URI: "f_uri_4"}}}})
	require.NoError(t, err)

	sm5, err := sonic.MarshalString(&schema.Message{Content: ""})
	require.NoError(t, err)

	sm6, err := sonic.MarshalString(&schema.Message{MultiContent: []schema.ChatMessagePart{{Type: schema.ChatMessagePartTypeImageURL, ImageURL: &schema.ChatMessageImageURL{URI: "image_uri_5"}}}})
	require.NoError(t, err)

	sm7, err := sonic.MarshalString(&schema.Message{MultiContent: []schema.ChatMessagePart{{Type: schema.ChatMessagePartTypeImageURL, ImageURL: &schema.ChatMessageImageURL{URI: "file_id_6"}}, {Type: schema.ChatMessagePartTypeImageURL, ImageURL: &schema.ChatMessageImageURL{URI: "file_id_7"}}}})
	require.NoError(t, err)

	sm8, err := sonic.MarshalString(&schema.Message{MultiContent: []schema.ChatMessagePart{{Type: schema.ChatMessagePartTypeText, Text: "hello"}, {Type: schema.ChatMessagePartTypeImageURL, ImageURL: &schema.ChatMessageImageURL{URI: "file_id_8"}}, {Type: schema.ChatMessagePartTypeFileURL, FileURL: &schema.ChatMessageFileURL{URI: "file_id_9"}}}})
	require.NoError(t, err)

	type args struct {
		msgs []*entity.Message
	}
	type want struct {
		convMsgs   []*crossmessage.WfMessage
		schemaMsgs []*schema.Message
	}
	tests := []struct {
		name    string
		args    args
		want    want
		wantErr bool
	}{
		{
			name: "pure text",
			args: args{
				msgs: []*entity.Message{
					{
						ID:           1,
						Role:         schema.User,
						ContentType:  "text",
						ModelContent: sm1,
					},
				},
			},
			want: want{
				convMsgs: []*crossmessage.WfMessage{
					{
						ID:          1,
						Role:        schema.User,
						ContentType: "text",
						Text:        ptr.Of("hello"),
					},
				},
				schemaMsgs: []*schema.Message{
					{
						Role:    schema.User,
						Content: "hello",
					},
				},
			},
		},
		{
			name: "pure file",
			args: args{
				msgs: []*entity.Message{
					{
						ID:           2,
						Role:         schema.User,
						ContentType:  "file",
						ModelContent: sm2,
					},
				},
			},
			want: want{
				convMsgs: []*crossmessage.WfMessage{
					{
						ID:          2,
						Role:        schema.User,
						ContentType: "file",
						MultiContent: []*crossmessage.Content{
							{Type: message.InputTypeFile, Uri: ptr.Of("f_uri_1"), Url: ptr.Of("f_uri_1")},
						},
					},
				},
				schemaMsgs: []*schema.Message{
					{
						Role: schema.User,
						MultiContent: []schema.ChatMessagePart{
							{Type: schema.ChatMessagePartTypeFileURL, FileURL: &schema.ChatMessageFileURL{URI: "f_uri_1", URL: "f_uri_1"}},
						},
					},
				},
			},
		},
		{
			name: "text and file",
			args: args{
				msgs: []*entity.Message{
					{
						ID:           3,
						Role:         schema.User,
						ContentType:  "text_file",
						ModelContent: sm3,
					},
				},
			},
			want: want{
				convMsgs: []*crossmessage.WfMessage{
					{
						ID:          3,
						Role:        schema.User,
						ContentType: "text_file",
						MultiContent: []*crossmessage.Content{
							{Type: message.InputTypeText, Text: ptr.Of("hello")},
							{Type: message.InputTypeFile, Uri: ptr.Of("f_uri_2"), Url: ptr.Of("f_uri_2")},
						},
					},
				},
				schemaMsgs: []*schema.Message{
					{
						Role: schema.User,
						MultiContent: []schema.ChatMessagePart{
							{Type: schema.ChatMessagePartTypeText, Text: "hello"},
							{Type: schema.ChatMessagePartTypeFileURL, FileURL: &schema.ChatMessageFileURL{URI: "f_uri_2", URL: "f_uri_2"}},
						},
					},
				},
			},
		},
		{
			name: "multiple files",
			args: args{
				msgs: []*entity.Message{
					{
						ID:           4,
						Role:         schema.User,
						ContentType:  "file",
						ModelContent: sm4,
					},
				},
			},
			want: want{
				convMsgs: []*crossmessage.WfMessage{
					{
						ID:          4,
						Role:        schema.User,
						ContentType: "file",
						MultiContent: []*crossmessage.Content{
							{Type: message.InputTypeFile, Uri: ptr.Of("f_uri_3"), Url: ptr.Of("f_uri_3")},
							{Type: message.InputTypeFile, Uri: ptr.Of("f_uri_4"), Url: ptr.Of("f_uri_4")},
						},
					},
				},
				schemaMsgs: []*schema.Message{
					{
						Role: schema.User,
						MultiContent: []schema.ChatMessagePart{
							{Type: schema.ChatMessagePartTypeFileURL, FileURL: &schema.ChatMessageFileURL{URI: "f_uri_3", URL: "f_uri_3"}},
							{Type: schema.ChatMessagePartTypeFileURL, FileURL: &schema.ChatMessageFileURL{URI: "f_uri_4", URL: "f_uri_4"}},
						},
					},
				},
			},
		},
		{
			name: "empty text",
			args: args{
				msgs: []*entity.Message{
					{
						ID:           5,
						Role:         schema.User,
						ContentType:  "text",
						ModelContent: sm5,
					},
				},
			},
			want: want{
				convMsgs: []*crossmessage.WfMessage{
					{
						ID:          5,
						Role:        schema.User,
						ContentType: "text",
						Text:        ptr.Of(""),
					},
				},
				schemaMsgs: []*schema.Message{
					{
						Role:    schema.User,
						Content: "",
					},
				},
			},
		},
		{
			name: "pure image",
			args: args{
				msgs: []*entity.Message{
					{
						ID:           6,
						Role:         schema.User,
						ContentType:  "image",
						ModelContent: sm6,
					},
				},
			},
			want: want{
				convMsgs: []*crossmessage.WfMessage{
					{
						ID:          6,
						Role:        schema.User,
						ContentType: "image",
						MultiContent: []*crossmessage.Content{
							{Type: message.InputTypeImage, Uri: ptr.Of("image_uri_5"), Url: ptr.Of("image_uri_5")},
						},
					},
				},
				schemaMsgs: []*schema.Message{
					{
						Role: schema.User,
						MultiContent: []schema.ChatMessagePart{
							{Type: schema.ChatMessagePartTypeImageURL, ImageURL: &schema.ChatMessageImageURL{URI: "image_uri_5", URL: "image_uri_5"}},
						},
					},
				},
			},
		},
		{
			name: "multiple images",
			args: args{
				msgs: []*entity.Message{
					{
						ID:           7,
						Role:         schema.User,
						ContentType:  "image",
						ModelContent: sm7,
					},
				},
			},
			want: want{
				convMsgs: []*crossmessage.WfMessage{
					{
						ID:          7,
						Role:        schema.User,
						ContentType: "image",
						MultiContent: []*crossmessage.Content{
							{Type: message.InputTypeImage, Uri: ptr.Of("file_id_6"), Url: ptr.Of("file_id_6")},
							{Type: message.InputTypeImage, Uri: ptr.Of("file_id_7"), Url: ptr.Of("file_id_7")},
						},
					},
				},
				schemaMsgs: []*schema.Message{
					{
						Role: schema.User,
						MultiContent: []schema.ChatMessagePart{
							{Type: schema.ChatMessagePartTypeImageURL, ImageURL: &schema.ChatMessageImageURL{URI: "file_id_6", URL: "file_id_6"}},
							{Type: schema.ChatMessagePartTypeImageURL, ImageURL: &schema.ChatMessageImageURL{URI: "file_id_7", URL: "file_id_7"}},
						},
					},
				},
			},
		},
		{
			name: "mixed content",
			args: args{
				msgs: []*entity.Message{
					{
						ID:           8,
						Role:         schema.User,
						ContentType:  "mix",
						ModelContent: sm8,
					},
				},
			},
			want: want{
				convMsgs: []*crossmessage.WfMessage{
					{
						ID:          8,
						Role:        schema.User,
						ContentType: "mix",
						MultiContent: []*crossmessage.Content{
							{Type: message.InputTypeText, Text: ptr.Of("hello")},
							{Type: message.InputTypeImage, Uri: ptr.Of("file_id_8"), Url: ptr.Of("file_id_8")},
							{Type: message.InputTypeFile, Uri: ptr.Of("file_id_9"), Url: ptr.Of("file_id_9")},
						},
					},
				},
				schemaMsgs: []*schema.Message{
					{
						Role: schema.User,
						MultiContent: []schema.ChatMessagePart{
							{Type: schema.ChatMessagePartTypeText, Text: "hello"},
							{Type: schema.ChatMessagePartTypeImageURL, ImageURL: &schema.ChatMessageImageURL{URI: "file_id_8", URL: "file_id_8"}},
							{Type: schema.ChatMessagePartTypeFileURL, FileURL: &schema.ChatMessageFileURL{URI: "file_id_9", URL: "file_id_9"}},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			convMsgs, schemaMsgs, err := convertToConvAndSchemaMessage(context.Background(), tt.args.msgs)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want.convMsgs, convMsgs)
			assert.Equal(t, tt.want.schemaMsgs, schemaMsgs)
		})
	}
}
