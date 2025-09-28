package repository

import (
	cache "backend/internal"
	"backend/internal/model"
	"context"
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

// PageStable: 全体安定ソートの s..s+L-1 を高速に返す
func PageStable[T any](arr []T, less func(a, b T) bool, page, L int) []T {
	n := len(arr)
	if L <= 0 || n == 0 {
		return nil
	}
	s := (page - 1) * L
	if s >= n {
		return nil
	}
	e := s + L
	if e > n {
		e = n
	}

	// 安定性担保用にインデックス付与
	type item struct {
		V   T
		idx int
	}
	a := make([]item, n)
	for i, v := range arr {
		a[i] = item{V: v, idx: i}
	}

	stableLess := func(i, j item) bool {
		if less(i.V, j.V) {
			return true
		}
		if less(j.V, i.V) {
			return false
		}
		return i.idx < j.idx // tie -> 元の並び
	}

	// Quickselect（nth 要素を左に寄せる）
	nthSelect := func(lo, hi, k int) {
		for lo < hi {
			// ピボット選択（median-of-three 等を入れても良い）
			mid := lo + (hi-lo)/2
			pivot := a[mid]
			i, j := lo, hi
			for i <= j {
				for stableLess(a[i], pivot) {
					i++
				}
				for stableLess(pivot, a[j]) {
					j--
				}
				if i <= j {
					a[i], a[j] = a[j], a[i]
					i++
					j--
				}
			}
			if k <= j {
				hi = j
			} else if k >= i {
				lo = i
			} else {
				return
			}
		}
	}

	// 1) s 位、2) e-1 位で選択し、[s,e) をブロック化
	nthSelect(0, n-1, s)
	nthSelect(s, n-1, e-1)

	// ブロック境界の同値を巻き取る（必要分）
	// 下側：s-1 から左へ同値を | 上側：e から右へ同値を
	// 同値判定には less を2回使う（equal ⇔ !less(a,b) && !less(b,a)）
	equal := func(x, y item) bool { return !less(x.V, y.V) && !less(y.V, x.V) }

	left, right := s, e
	// 左へ
	for left > 0 && equal(a[left-1], a[s]) {
		left--
	}
	// 右へ
	base := a[e-1]
	for right < n && equal(a[right], base) {
		right++
	}

	// 拡張ブロックを安定比較で整列（小さい範囲だけ）
	blk := a[left:right]
	sort.Slice(blk, func(i, j int) bool { return stableLess(blk[i], blk[j]) })

	// 「全体安定順の s..e-1」に一致する L 件を切り出す
	// ＝ blk から「全体順位 < s」を除去し、先頭から (e-s) 件
	// 全体順位 < s は、blk 要素のうち stableLess(blk[i], a[s]) が true のもの
	first := 0
	lowKey := a[s]
	for first < len(blk) && stableLess(blk[first], lowKey) {
		first++
	}
	last := first + (e - s)
	if last > len(blk) {
		last = len(blk)
	} // 念のため

	out := make([]T, 0, last-first)
	for _, it := range blk[first:last] {
		out = append(out, it.V)
	}
	return out
}

// 注文履歴一覧を取得
func (r *OrderRepository) ListOrders(ctx context.Context, userID int, req model.ListRequest) ([]model.Order, int, error) {
	cache.Cache.Order.RLock()
	ordersRaw := cache.Cache.UserOrders[userID]
	cache.Cache.Order.RUnlock()

	orders := make([]model.Order, 0, len(ordersRaw))
	for _, o := range ordersRaw {
		p := cache.Cache.ProductsById[o.ProductID]
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

	var pagedOrders []model.Order
	sortBy := func(less func(a, b model.Order) bool) {
		pagedOrders = PageStable(orders, less, req.PageSize, req.Offset)
	}

	switch req.SortField {
	case "product_name":
		if strings.ToUpper(req.SortOrder) == "DESC" {
			sortBy(func(i, j model.Order) bool {
				return i.ProductName > j.ProductName
			})
		} else {
			sortBy(func(i, j model.Order) bool {
				return i.ProductName < j.ProductName
			})
		}
	case "created_at":
		if strings.ToUpper(req.SortOrder) == "DESC" {
			sortBy(func(i, j model.Order) bool {
				return i.CreatedAt.After(j.CreatedAt)
			})
		} else {
			sortBy(func(i, j model.Order) bool {
				return i.CreatedAt.Before(j.CreatedAt)
			})
		}
	case "shipped_status":
		if strings.ToUpper(req.SortOrder) == "DESC" {
			sortBy(func(i, j model.Order) bool {
				return i.ShippedStatus > j.ShippedStatus
			})
		} else {
			sortBy(func(i, j model.Order) bool {
				return i.ShippedStatus < j.ShippedStatus
			})
		}
	case "arrived_at":
		if strings.ToUpper(req.SortOrder) == "DESC" {
			sortBy(func(i, j model.Order) bool {
				if i.ArrivedAt.Valid && j.ArrivedAt.Valid {
					return i.ArrivedAt.Time.After(j.ArrivedAt.Time)
				}
				return i.ArrivedAt.Valid
			})
		} else {
			sortBy(func(i, j model.Order) bool {
				if i.ArrivedAt.Valid && j.ArrivedAt.Valid {
					return i.ArrivedAt.Time.Before(j.ArrivedAt.Time)
				}
				return j.ArrivedAt.Valid
			})
		}
	case "order_id":
		fallthrough
	default:
		if strings.ToUpper(req.SortOrder) == "DESC" {
			sortBy(func(i, j model.Order) bool {
				return i.OrderID > j.OrderID
			})
		} else {
			sortBy(func(i, j model.Order) bool {
				return i.OrderID < j.OrderID
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
	pagedOrders = orders[start:end]

	return pagedOrders, total, nil
}
