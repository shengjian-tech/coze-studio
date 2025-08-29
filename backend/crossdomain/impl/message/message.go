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
	"fmt"
	"strconv"

	"github.com/cloudwego/eino/schema"

	model "github.com/coze-dev/coze-studio/backend/api/model/crossdomain/message"
	crossagentrun "github.com/coze-dev/coze-studio/backend/crossdomain/contract/agentrun"
	crossmessage "github.com/coze-dev/coze-studio/backend/crossdomain/contract/message"
	agententity "github.com/coze-dev/coze-studio/backend/domain/conversation/agentrun/entity"
	"github.com/coze-dev/coze-studio/backend/domain/conversation/message/entity"
	"github.com/coze-dev/coze-studio/backend/domain/workflow"
	"github.com/coze-dev/coze-studio/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-studio/backend/pkg/sonic"

	message "github.com/coze-dev/coze-studio/backend/domain/conversation/message/service"
)

var defaultSVC crossmessage.Message

type impl struct {
	DomainSVC message.Message
}

func InitDomainService(c message.Message) crossmessage.Message {
	defaultSVC = &impl{
		DomainSVC: c,
	}

	return defaultSVC
}

func (c *impl) MessageList(ctx context.Context, req *crossmessage.MessageListRequest) (*crossmessage.MessageListResponse, error) {
	lm := &entity.ListMeta{
		ConversationID: req.ConversationID,
		Limit:          int(req.Limit), // Since the value of limit is checked inside the node, the type cast here is safe
		UserID:         strconv.FormatInt(req.UserID, 10),
		AgentID:        req.AppID,
		OrderBy:        req.OrderBy,
	}
	if req.BeforeID != nil {
		lm.Cursor, _ = strconv.ParseInt(*req.BeforeID, 10, 64)
		lm.Direction = entity.ScrollPageDirectionNext
	}
	if req.AfterID != nil {
		lm.Cursor, _ = strconv.ParseInt(*req.AfterID, 10, 64)
		lm.Direction = entity.ScrollPageDirectionPrev
	}
	lm.MessageType = []*model.MessageType{ptr.Of(model.MessageTypeQuestion), ptr.Of(model.MessageTypeAnswer)}

	lr, err := c.DomainSVC.ListWithoutPair(ctx, lm)
	if err != nil {
		return nil, err
	}

	response := &crossmessage.MessageListResponse{
		HasMore: lr.HasMore,
	}

	if lr.PrevCursor > 0 {
		response.FirstID = strconv.FormatInt(lr.PrevCursor, 10)
	}
	if lr.NextCursor > 0 {
		response.LastID = strconv.FormatInt(lr.NextCursor, 10)
	}
	if len(lr.Messages) == 0 {
		return response, nil
	}
	messages, _, err := convertToConvAndSchemaMessage(ctx, lr.Messages)
	if err != nil {
		return nil, err
	}
	response.Messages = messages
	return response, nil
}

func (c *impl) GetLatestRunIDs(ctx context.Context, req *crossmessage.GetLatestRunIDsRequest) ([]int64, error) {
	listMeta := &agententity.ListRunRecordMeta{
		ConversationID: req.ConversationID,
		AgentID:        req.AppID,
		Limit:          int32(req.Rounds),
		SectionID:      req.SectionID,
	}

	if req.InitRunID != nil {
		listMeta.BeforeID = *req.InitRunID
	}

	runRecords, err := crossagentrun.DefaultSVC().List(ctx, listMeta)
	if err != nil {
		return nil, err
	}
	runIDs := make([]int64, 0, len(runRecords))
	for _, record := range runRecords {
		runIDs = append(runIDs, record.ID)
	}
	return runIDs, nil
}

func (c *impl) GetMessagesByRunIDs(ctx context.Context, req *crossmessage.GetMessagesByRunIDsRequest) (*crossmessage.GetMessagesByRunIDsResponse, error) {
	responseMessages, err := c.DomainSVC.GetByRunIDs(ctx, req.ConversationID, req.RunIDs)
	if err != nil {
		return nil, err
	}
	// only returns messages of type user/assistant/system role type
	messages := make([]*model.Message, 0, len(responseMessages))
	for _, m := range responseMessages {
		if m.Role == schema.User || m.Role == schema.System || m.Role == schema.Assistant {
			messages = append(messages, m)
		}
	}

	convMessages, scMessages, err := convertToConvAndSchemaMessage(ctx, messages)
	if err != nil {
		return nil, err
	}
	return &crossmessage.GetMessagesByRunIDsResponse{
		Messages:       convMessages,
		SchemaMessages: scMessages,
	}, nil
}

func (c *impl) GetByRunIDs(ctx context.Context, conversationID int64, runIDs []int64) ([]*model.Message, error) {
	return c.DomainSVC.GetByRunIDs(ctx, conversationID, runIDs)
}

func (c *impl) Create(ctx context.Context, msg *model.Message) (*model.Message, error) {
	return c.DomainSVC.Create(ctx, msg)
}

func (c *impl) Edit(ctx context.Context, msg *model.Message) (*model.Message, error) {
	return c.DomainSVC.Edit(ctx, msg)
}

func (c *impl) PreCreate(ctx context.Context, msg *model.Message) (*model.Message, error) {
	return c.DomainSVC.PreCreate(ctx, msg)
}

func (c *impl) List(ctx context.Context, lm *entity.ListMeta) (*entity.ListResult, error) {
	return c.DomainSVC.List(ctx, lm)
}

func (c *impl) Delete(ctx context.Context, req *entity.DeleteMeta) error {
	return c.DomainSVC.Delete(ctx, req)
}

func (c *impl) GetMessageByID(ctx context.Context, id int64) (*entity.Message, error) {
	return c.DomainSVC.GetByID(ctx, id)
}

func (c *impl) ListWithoutPair(ctx context.Context, req *entity.ListMeta) (*entity.ListResult, error) {
	return c.DomainSVC.ListWithoutPair(ctx, req)
}

func convertToConvAndSchemaMessage(ctx context.Context, msgs []*entity.Message) ([]*crossmessage.WfMessage, []*schema.Message, error) {
	messages := make([]*schema.Message, 0)
	convMessages := make([]*crossmessage.WfMessage, 0)
	for _, m := range msgs {
		msg := &schema.Message{}
		err := sonic.UnmarshalString(m.ModelContent, msg)
		if err != nil {
			return nil, nil, err
		}
		msg.Role = m.Role

		covMsg := &crossmessage.WfMessage{
			ID:          m.ID,
			Role:        m.Role,
			ContentType: string(m.ContentType),
			SectionID:   m.SectionID,
		}

		if len(msg.MultiContent) == 0 {
			covMsg.Text = ptr.Of(msg.Content)
		} else {
			covMsg.MultiContent = make([]*crossmessage.Content, 0, len(msg.MultiContent))
			for _, part := range msg.MultiContent {
				switch part.Type {
				case schema.ChatMessagePartTypeText:
					covMsg.MultiContent = append(covMsg.MultiContent, &crossmessage.Content{
						Type: model.InputTypeText,
						Text: ptr.Of(part.Text),
					})

				case schema.ChatMessagePartTypeImageURL:
					if part.ImageURL != nil {
						part.ImageURL.URL, err = workflow.GetRepository().GetObjectUrl(ctx, part.ImageURL.URI)
						if err != nil {
							return nil, nil, err
						}
						covMsg.MultiContent = append(covMsg.MultiContent, &crossmessage.Content{
							Uri:  ptr.Of(part.ImageURL.URI),
							Type: model.InputTypeImage,
							Url:  ptr.Of(part.ImageURL.URL),
						})
					}

				case schema.ChatMessagePartTypeFileURL:

					if part.FileURL != nil {
						part.FileURL.URL, err = workflow.GetRepository().GetObjectUrl(ctx, part.FileURL.URI)
						if err != nil {
							return nil, nil, err
						}

						covMsg.MultiContent = append(covMsg.MultiContent, &crossmessage.Content{
							Uri:  ptr.Of(part.FileURL.URI),
							Type: model.InputTypeFile,
							Url:  ptr.Of(part.FileURL.URL),
						})

					}

				case schema.ChatMessagePartTypeAudioURL:
					if part.AudioURL != nil {
						part.AudioURL.URL, err = workflow.GetRepository().GetObjectUrl(ctx, part.AudioURL.URI)
						if err != nil {
							return nil, nil, err
						}
						covMsg.MultiContent = append(covMsg.MultiContent, &crossmessage.Content{
							Uri:  ptr.Of(part.AudioURL.URI),
							Type: model.InputTypeAudio,
							Url:  ptr.Of(part.AudioURL.URL),
						})

					}
				case schema.ChatMessagePartTypeVideoURL:
					if part.VideoURL != nil {
						part.VideoURL.URL, err = workflow.GetRepository().GetObjectUrl(ctx, part.VideoURL.URI)
						if err != nil {
							return nil, nil, err
						}
						covMsg.MultiContent = append(covMsg.MultiContent, &crossmessage.Content{
							Uri:  ptr.Of(part.VideoURL.URI),
							Type: model.InputTypeVideo,
							Url:  ptr.Of(part.VideoURL.URL),
						})
					}
				default:
					return nil, nil, fmt.Errorf("unknown part type: %s", part.Type)
				}
			}
		}

		messages = append(messages, msg)
		convMessages = append(convMessages, covMsg)
	}
	return convMessages, messages, nil
}
