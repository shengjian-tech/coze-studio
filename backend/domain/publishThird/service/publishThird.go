package service

import (
	"context"
	"github.com/coze-dev/coze-studio/backend/domain/publishThird/entity"
	"github.com/coze-dev/coze-studio/backend/domain/publishThird/repository"
	"github.com/coze-dev/coze-studio/backend/infra/contract/cache"
	"github.com/coze-dev/coze-studio/backend/infra/contract/chatmodel"
	"github.com/coze-dev/coze-studio/backend/infra/contract/document/nl2sql"
	"github.com/coze-dev/coze-studio/backend/infra/contract/document/ocr"
	"github.com/coze-dev/coze-studio/backend/infra/contract/document/parser"
	"github.com/coze-dev/coze-studio/backend/infra/contract/document/rerank"
	"github.com/coze-dev/coze-studio/backend/infra/contract/document/searchstore"
	"github.com/coze-dev/coze-studio/backend/infra/contract/eventbus"
	"github.com/coze-dev/coze-studio/backend/infra/contract/idgen"
	"github.com/coze-dev/coze-studio/backend/infra/contract/messages2query"
	"github.com/coze-dev/coze-studio/backend/infra/contract/rdb"
	"github.com/coze-dev/coze-studio/backend/infra/contract/storage"
	"github.com/coze-dev/coze-studio/backend/pkg/errorx"
	"github.com/coze-dev/coze-studio/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-studio/backend/types/errno"
	"gorm.io/gorm"
)

func NewKnowledgeSVC(config *ThirdSVCConfig) PublishThird {
	svc := &publishThirdSVC{
		publishThirdRepo:    repository.NewPublishThirdDAO(config.DB),
		idgen:               config.IDGen,
		rdb:                 config.RDB,
		producer:            config.Producer,
		searchStoreManagers: config.SearchStoreManagers,
		parseManager:        config.ParseManager,
		storage:             config.Storage,
		reranker:            config.Reranker,
		rewriter:            config.Rewriter,
		nl2Sql:              config.NL2Sql,
		enableCompactTable:  ptr.FromOrDefault(config.EnableCompactTable, true),
		cacheCli:            config.CacheCli,
		modelFactory:        config.ModelFactory,
	}

	return svc
}

type ThirdSVCConfig struct {
	DB                  *gorm.DB                       // required
	IDGen               idgen.IDGenerator              // required
	RDB                 rdb.RDB                        // Required: Form storage
	Producer            eventbus.Producer              // Required: Document indexing process goes through mq asynchronous processing
	SearchStoreManagers []searchstore.Manager          // Required: Vector/Full Text
	ParseManager        parser.Manager                 // Optional: document segmentation and processing capability, default builtin parser
	Storage             storage.Storage                // required: oss
	ModelFactory        chatmodel.Factory              // Required: Model factory
	Rewriter            messages2query.MessagesToQuery // Optional: Do not overwrite when not configured
	Reranker            rerank.Reranker                // Optional: default rrf when not configured
	NL2Sql              nl2sql.NL2SQL                  // Optional: Not supported by default when not configured
	EnableCompactTable  *bool                          // Optional: Table data compression, default true
	OCR                 ocr.OCR                        // Optional: ocr, ocr function is not available when not provided
	CacheCli            cache.Cmdable                  // Optional: cache implementation
}

type publishThirdSVC struct {
	publishThirdRepo repository.PublishThirdRepo
	modelFactory     chatmodel.Factory

	idgen               idgen.IDGenerator
	rdb                 rdb.RDB
	producer            eventbus.Producer
	searchStoreManagers []searchstore.Manager
	parseManager        parser.Manager
	rewriter            messages2query.MessagesToQuery
	reranker            rerank.Reranker
	storage             storage.Storage
	nl2Sql              nl2sql.NL2SQL
	cacheCli            cache.Cmdable
	enableCompactTable  bool // Table data compression
}

// Publish 发布内容
func (p *publishThirdSVC) PublishArticle(ctx context.Context, request *ThirdRequest) (response *ThirdResponse, err error) {

	return &ThirdResponse{
		msg: "ok",
	}, nil
}

// getInfo 获取点赞量、收藏量、评论量
func (p *publishThirdSVC) GetTweetUrlList(ctx context.Context, request *ThirdRequest) (response *ThirdResponse, err error) {

	if request.UserId != nil {
		return nil, errorx.New(500, errorx.KV("msg", "用户id不能为空"))
	}
	opts := &entity.WherePublishThirdUrlOption{
		Status:   request.Status,
		UserID:   request.UserId,
		Query:    request.Introduction,
		Page:     request.Page,
		PageSize: request.PageSize,
		Order:    request.Order,
	}
	pos, total, err := p.publishThirdRepo.FindXhsUrlByList(ctx, opts)
	if err != nil {
		return nil, errorx.New(errno.ErrKnowledgeDBCode, errorx.KV("msg", err.Error()))
	}
	return &ThirdResponse{
		PublishThirdList: pos,
		Total:            total,
		msg:              "ok",
	}, nil
}
