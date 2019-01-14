package chat

import (
	"context"

	api "github.com/jhayotte/chat/api/v1/chatd"
)

type Service struct {
}

func NewChatService() *Service {
	return &Service{}
}

func (s *Service) PublishMessage(ctx context.Context, req *api.PublishMessageRequest) (resp *api.PublishMessageResponse, err error) {
	return &api.PublishMessageResponse{}, nil
}
