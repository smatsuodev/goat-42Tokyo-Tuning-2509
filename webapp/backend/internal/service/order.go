package service

import (
	"backend/internal/model"
	"backend/internal/repository"
	"context"
)

type OrderService struct {
	store *repository.Store
}

func NewOrderService(store *repository.Store) *OrderService {
	return &OrderService{store: store}
}

// ユーザーの注文履歴を取得
func (s *OrderService) FetchOrders(ctx context.Context, userID int, req model.ListRequest) ([]model.Order, int, error) {
	var orders []model.Order
	var total int
	orders, total, fetchErr := s.store.OrderRepo.ListOrders(ctx, userID, req)
	if fetchErr != nil {
		return nil, 0, fetchErr
	}
	return orders, total, nil
}
