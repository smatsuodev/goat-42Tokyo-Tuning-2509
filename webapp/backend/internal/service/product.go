package service

import (
	"context"
	"log"

	"backend/internal/model"
	"backend/internal/repository"
)

type ProductService struct {
	store *repository.Store
}

func NewProductService(store *repository.Store) *ProductService {
	return &ProductService{store: store}
}

func (s *ProductService) CreateOrders(ctx context.Context, userID int, items []model.RequestItem) ([]string, error) {
	var insertedOrderIDs []string

	orders := make([]*model.Order, 0)

	for _, item := range items {
		if item.Quantity > 0 {
			for i := 0; i < item.Quantity; i++ {
				order := &model.Order{
					UserID:    userID,
					ProductID: item.ProductID,
				}
				orders = append(orders, order)
			}
		}
	}

	if len(orders) != 0 {
		ids, err := s.store.OrderRepo.CreateMany(ctx, orders)
		if err != nil {
			return nil, err
		}
		insertedOrderIDs = ids
	}

	log.Printf("Created %d orders for user %d", len(insertedOrderIDs), userID)
	return insertedOrderIDs, nil
}

func (s *ProductService) FetchProducts(ctx context.Context, userID int, req model.ListRequest) ([]model.Product, int, error) {
	products, total, err := s.store.ProductRepo.ListProducts(ctx, userID, req)
	return products, total, err
}
