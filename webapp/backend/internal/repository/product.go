package repository

import (
	cache "backend/internal"
	"backend/internal/model"
	"context"
	"sort"
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
		products = cache.Cache.Products
		sort.SliceStable(products, func(i, j int) bool {
			switch req.SortField {
			case "description":
				if req.SortOrder == "DESC" {
					return products[i].Description > products[j].Description
				} else {
					return products[i].Description < products[j].Description
				}
			case "image":
				if req.SortOrder == "DESC" {
					return products[i].Image > products[j].Image
				} else {
					return products[i].Image < products[j].Image
				}
			case "name":
				if req.SortOrder == "DESC" {
					return products[i].Name > products[j].Name
				} else {
					return products[i].Name < products[j].Name
				}
			case "productid":
				if req.SortOrder == "DESC" {
					return products[i].ProductID > products[j].ProductID
				} else {
					return products[i].ProductID < products[j].ProductID
				}
			case "value":
				if req.SortOrder == "DESC" {
					return products[i].Value > products[j].Value
				} else {
					return products[i].Value < products[j].Value
				}
			case "weight":
				if req.SortOrder == "DESC" {
					return products[i].Weight > products[j].Weight
				} else {
					return products[i].Weight < products[j].Weight
				}
			default:
				// todo: 例外処理
				return true
			}
		})
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
