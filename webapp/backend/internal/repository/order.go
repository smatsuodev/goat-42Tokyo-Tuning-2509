package repository

import (
	cache "backend/internal"
	"backend/internal/model"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
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

	cache.Cache.ShippingOrderProductId.Mu.Lock()
	defer func() {
		cache.Cache.ShippingOrderProductId.Mu.Unlock()
	}()
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
		cache.Cache.ShippingOrderProductId.Values[i] = orders[i-idStart].ProductID
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
	if newStatus != "shipping" {
		cache.Cache.ShippingOrderProductId.Mu.Lock()
		defer func() {
			cache.Cache.ShippingOrderProductId.Mu.Unlock()
		}()
		for _, orderId := range orderIDs {
			delete(cache.Cache.ShippingOrderProductId.Values, orderId)
		}
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

	cache.Cache.ShippingOrderProductId.Mu.Lock()
	defer func() {
		cache.Cache.ShippingOrderProductId.Mu.Unlock()
	}()
	if !cache.Cache.ShippingOrderProductId.IsInit {
		var orders []model.Order
		if err := r.db.SelectContext(ctx, &orders, "SELECT * FROM orders WHERE shipped_status = 'shipping' "); err != nil {
			log.Fatalf("Failed to get shipping orders: %v", err)
		}
		for _, o := range orders {
			cache.Cache.ShippingOrderProductId.Values[o.OrderID] = o.ProductID
		}
		cache.Cache.ShippingOrderProductId.IsInit = true
	}
	orders := lo.MapToSlice(cache.Cache.ShippingOrderProductId.Values, func(k int64, v int) model.Order {
		p, _ := cache.Cache.ProductsById.Get(ctx, v)
		return model.Order{
			OrderID: k,
			Weight:  p.Value.Weight,
			Value:   p.Value.Value,
		}
	})

	return orders, nil
}

// 配送対象となる(shipping)注文の件数を取得
func (r *OrderRepository) CountShippingOrders(ctx context.Context) (int, error) {
	cache.Cache.ShippingOrderProductId.Mu.Lock()
	defer cache.Cache.ShippingOrderProductId.Mu.Unlock()
	if !cache.Cache.ShippingOrderProductId.IsInit {
		var orders []model.Order
		if err := r.db.SelectContext(ctx, &orders, "SELECT * FROM orders WHERE shipped_status = 'shipping' "); err != nil {
			log.Fatalf("Failed to get shipping orders: %v", err)
		}
		for _, o := range orders {
			cache.Cache.ShippingOrderProductId.Values[o.OrderID] = o.ProductID
		}
		cache.Cache.ShippingOrderProductId.IsInit = true
	}
	return len(cache.Cache.ShippingOrderProductId.Values), nil
}

// 注文履歴一覧を取得
func (r *OrderRepository) ListOrders(ctx context.Context, userID int, req model.ListRequest) ([]model.Order, int, error) {
	query := `
        SELECT order_id, product_id, shipped_status, created_at, arrived_at
        FROM orders
        WHERE user_id = ?
    `
	type orderRow struct {
		OrderID       int          `db:"order_id"`
		ProductID     int          `db:"product_id"`
		ShippedStatus string       `db:"shipped_status"`
		CreatedAt     sql.NullTime `db:"created_at"`
		ArrivedAt     sql.NullTime `db:"arrived_at"`
	}
	var ordersRaw []orderRow
	if err := r.db.SelectContext(ctx, &ordersRaw, query, userID); err != nil {
		return nil, 0, err
	}

	var orders []model.Order
	for _, o := range ordersRaw {
		p, err := cache.Cache.ProductsById.Get(ctx, o.ProductID)
		if err != nil {
			return nil, 0, err
		}
		if !p.Found {
			return nil, 0, errors.New("product not found")
		}
		productName := p.Value.Name
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
		orders = append(orders, model.Order{
			OrderID:       int64(o.OrderID),
			ProductID:     o.ProductID,
			ProductName:   productName,
			ShippedStatus: o.ShippedStatus,
			CreatedAt:     o.CreatedAt.Time,
			ArrivedAt:     o.ArrivedAt,
		})
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
