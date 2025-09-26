package repository

import (
	cache "backend/internal"
	"backend/internal/model"
	"context"
)

type ProductRepository struct {
	db DBTX
}

func NewProductRepository(db DBTX) *ProductRepository {
	return &ProductRepository{db: db}
}

// 商品一覧を全件取得し、アプリケーション側でページング処理を行う
func (r *ProductRepository) ListProducts(ctx context.Context, userID int, req model.ListRequest) ([]model.Product, int, error) {
	var products []model.Product

	if req.Search == "" {
		cacheKey := req.SortField + "," + req.SortOrder
		res, err := cache.Cache.Products.Get(ctx, cacheKey)

		if err == nil && res.Found {
			products = res.Value
		} else {
			baseQuery := `
		SELECT product_id, name, value, weight, image, description
		FROM products
	` + " ORDER BY " + req.SortField + " " + req.SortOrder + " , product_id ASC"

			err := r.db.SelectContext(ctx, &products, baseQuery)
			if err != nil {
				return nil, 0, err
			}

			cache.Cache.Products.Set(ctx, cacheKey, products)
		}
	} else {
		baseQuery := `
		SELECT product_id, name, value, weight, image, description
		FROM products
	`
		args := []interface{}{}

		baseQuery += " WHERE (name LIKE ? OR description LIKE ?)"
		searchPattern := "%" + req.Search + "%"
		args = append(args, searchPattern, searchPattern)

		baseQuery += " ORDER BY " + req.SortField + " " + req.SortOrder + " , product_id ASC"

		err := r.db.SelectContext(ctx, &products, baseQuery, args...)
		if err != nil {
			return nil, 0, err
		}
	}

	total := len(products)
	start := req.Offset
	end := req.Offset + req.PageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}
	pagedProducts := products[start:end]

	return pagedProducts, total, nil
}
