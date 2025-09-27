package repository

import (
	cache "backend/internal"
	"backend/internal/model"
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/jmoiron/sqlx"
	"github.com/samber/lo"
)

type OrderRepository struct {
	db DBTX
}

func NewOrderRepository(db DBTX) *OrderRepository {
	return &OrderRepository{db: db}
}

// 注文を作成し、生成された注文IDを返す
func (r *OrderRepository) Create(ctx context.Context, order *model.Order) (string, error) {
	query := `INSERT INTO orders (user_id, product_id, shipped_status, created_at) VALUES (?, ?, 'shipping', NOW())`
	result, err := r.db.ExecContext(ctx, query, order.UserID, order.ProductID)
	if err != nil {
		return "", err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d", id), nil
}

func (r *OrderRepository) CreateMany(ctx context.Context, orders []*model.Order) ([]string, error) {
	var idStart int64

	cache.Cache.Order.Lock()
	defer cache.Cache.Order.Unlock()
	// TODO: トランザクション貼らないとまずいかも
	err := r.db.GetContext(ctx, &idStart, "SELECT `AUTO_INCREMENT` FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_NAME = 'orders'")
	if err != nil {
		return nil, err
	}

	query := `INSERT INTO orders (user_id, product_id, shipped_status, created_at) VALUES (:user_id, :product_id, 'shipping', NOW())`
	_, err = r.db.NamedExecContext(ctx, query, orders)
	if err != nil {
		return nil, err
	}

	idLast := idStart + int64(len(orders)) - 1

	ids := make([]string, idLast-idStart+1)
	for i := idStart; i <= idLast; i++ {
		ids[i-idStart] = fmt.Sprintf("%d", i)
		cache.UpdateOrder(*orders[i-idStart])
	}

	return ids, nil
}

// 複数の注文IDのステータスを一括で更新
// 主に配送ロボットが注文を引き受けた際に一括更新をするために使用
func (r *OrderRepository) UpdateStatuses(ctx context.Context, orderIDs []int64, newStatus string) error {
	if len(orderIDs) == 0 {
		return nil
	}
	query, args, err := sqlx.In("UPDATE orders SET shipped_status = ? WHERE order_id IN (?)", newStatus, orderIDs)
	if err != nil {
		return err
	}
	query = r.db.Rebind(query)
	_, err = r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	cache.Cache.Order.Lock()
	defer cache.Cache.Order.Unlock()
	for _, orderId := range orderIDs {
		if newStatus != "shipping" {
			delete(cache.Cache.ShippingOrderProductId, orderId)
		}
		e := cache.Cache.OrderIdUserId[orderId]
		cache.Cache.UserOrders[e.UserID][e.Index].ShippedStatus = newStatus
	}
	return nil
}

// 配送中(shipped_status:shipping)の注文一覧を取得
func (r *OrderRepository) GetShippingOrders(ctx context.Context) ([]model.Order, error) {
	// var orders []model.Order
	// query := `
	//     SELECT
	//         o.order_id,
	//         p.weight,
	//         p.value
	//     FROM orders o
	//     JOIN products p ON o.product_id = p.product_id
	//     WHERE o.shipped_status = 'shipping'
	// `

	// err := r.db.SelectContext(ctx, &orders, query)

	cache.Cache.Order.Lock()
	defer cache.Cache.Order.Unlock()
	var err error
	orders := lo.MapToSlice(cache.Cache.ShippingOrderProductId, func(k int64, v int) model.Order {
		p := cache.Cache.ProductsById[v]
		if p == nil {
			err = errors.New("not found")
		}
		return model.Order{
			OrderID: k,
			Weight:  p.Weight,
			Value:   p.Value,
		}
	})

	return orders, err
}

// 配送対象となる(shipping)注文の件数を取得
func (r *OrderRepository) CountShippingOrders(ctx context.Context) (int, error) {
	cache.Cache.Order.Lock()
	defer cache.Cache.Order.Unlock()
	return len(cache.Cache.ShippingOrderProductId), nil
}

// 注文履歴一覧を取得
func (r *OrderRepository) ListOrders(ctx context.Context, userID int, req model.ListRequest) ([]model.Order, int, error) {
	cache.Cache.Order.Lock()
	ordersRaw := cache.Cache.UserOrders[userID]
	cache.Cache.Order.Unlock()

	var orders []model.Order
	for _, o := range ordersRaw {
		p := cache.Cache.ProductsById[o.ProductID]
		if p == nil {
			return nil, 0, errors.New("product not found")
		}
		productName := p.Name
		if req.Search != "" {
			if req.Type == "prefix" {
				if !strings.HasPrefix(productName, req.Search) {
					continue
				}
			} else {
				if !strings.Contains(productName, req.Search) {
					continue
				}
			}
		}
		orders = append(orders, o)
	}

	switch req.SortField {
	case "product_name":
		if strings.ToUpper(req.SortOrder) == "DESC" {
			sort.SliceStable(orders, func(i, j int) bool {
				return orders[i].ProductName > orders[j].ProductName
			})
		} else {
			sort.SliceStable(orders, func(i, j int) bool {
				return orders[i].ProductName < orders[j].ProductName
			})
		}
	case "created_at":
		if strings.ToUpper(req.SortOrder) == "DESC" {
			sort.SliceStable(orders, func(i, j int) bool {
				return orders[i].CreatedAt.After(orders[j].CreatedAt)
			})
		} else {
			sort.SliceStable(orders, func(i, j int) bool {
				return orders[i].CreatedAt.Before(orders[j].CreatedAt)
			})
		}
	case "shipped_status":
		if strings.ToUpper(req.SortOrder) == "DESC" {
			sort.SliceStable(orders, func(i, j int) bool {
				return orders[i].ShippedStatus > orders[j].ShippedStatus
			})
		} else {
			sort.SliceStable(orders, func(i, j int) bool {
				return orders[i].ShippedStatus < orders[j].ShippedStatus
			})
		}
	case "arrived_at":
		if strings.ToUpper(req.SortOrder) == "DESC" {
			sort.SliceStable(orders, func(i, j int) bool {
				if orders[i].ArrivedAt.Valid && orders[j].ArrivedAt.Valid {
					return orders[i].ArrivedAt.Time.After(orders[j].ArrivedAt.Time)
				}
				return orders[i].ArrivedAt.Valid
			})
		} else {
			sort.SliceStable(orders, func(i, j int) bool {
				if orders[i].ArrivedAt.Valid && orders[j].ArrivedAt.Valid {
					return orders[i].ArrivedAt.Time.Before(orders[j].ArrivedAt.Time)
				}
				return orders[j].ArrivedAt.Valid
			})
		}
	case "order_id":
		fallthrough
	default:
		if strings.ToUpper(req.SortOrder) == "DESC" {
			sort.SliceStable(orders, func(i, j int) bool {
				return orders[i].OrderID > orders[j].OrderID
			})
		} else {
			sort.SliceStable(orders, func(i, j int) bool {
				return orders[i].OrderID < orders[j].OrderID
			})
		}
	}

	total := len(orders)
	start := req.Offset
	end := req.Offset + req.PageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}
	pagedOrders := orders[start:end]

	return pagedOrders, total, nil
}
