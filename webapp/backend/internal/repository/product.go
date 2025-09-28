package repository

import (
	cache "backend/internal"
	"backend/internal/model"
	"context"
	"fmt"
	"log"
	"strings"
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
		baseQuery := `
		SELECT product_id, name, value, weight, image, description
		FROM products
	` + " ORDER BY " + req.SortField + " " + req.SortOrder + " , product_id ASC LIMIT " + fmt.Sprintf("%d", req.PageSize) + " OFFSET " + fmt.Sprintf("%d", req.Offset)

		err := r.db.SelectContext(ctx, &products, baseQuery)
		if err != nil {
			return nil, 0, err
		}
		return products, cache.Cache.ProductsCnt, nil
	} else {
		products = make([]model.Product, 0, cache.Cache.ProductsCnt)
		for _, p := range cache.Cache.ProductsById {
			log.Printf("%v", p)
			if strings.Contains(p.Name, req.Search) || strings.Contains(p.Description, req.Search) {
				products = append(products, p)
			}
		}

		var paged []model.Product
		sortBy := func(less func(a, b model.Product) bool) {
			paged = PageStable(products, less, req.PageSize, req.Offset)
		}

		switch req.SortField {
		case "name":
			if strings.ToUpper(req.SortOrder) == "DESC" {
				sortBy(func(i, j model.Product) bool {
					return i.Name > j.Name
				})
			} else {
				sortBy(func(i, j model.Product) bool {
					return i.Name < j.Name
				})
			}
		case "description":
			if strings.ToUpper(req.SortOrder) == "DESC" {
				sortBy(func(i, j model.Product) bool {
					return i.Description > j.Description
				})
			} else {
				sortBy(func(i, j model.Product) bool {
					return i.Description < j.Description
				})
			}
		case "value":
			if strings.ToUpper(req.SortOrder) == "DESC" {
				sortBy(func(i, j model.Product) bool {
					return i.Value > j.Value
				})
			} else {
				sortBy(func(i, j model.Product) bool {
					return i.Value < j.Value
				})
			}
		case "image":
			if strings.ToUpper(req.SortOrder) == "DESC" {
				sortBy(func(i, j model.Product) bool {
					return i.Image > j.Image
				})
			} else {
				sortBy(func(i, j model.Product) bool {
					return i.Image < j.Image
				})
			}
		case "weight":
			if strings.ToUpper(req.SortOrder) == "DESC" {
				sortBy(func(i, j model.Product) bool {
					return i.Weight > j.Weight
				})
			} else {
				sortBy(func(i, j model.Product) bool {
					return i.Weight < j.Weight
				})
			}

		case "product_id":
			fallthrough
		default:
			if strings.ToUpper(req.SortOrder) == "DESC" {
				sortBy(func(i, j model.Product) bool {
					return i.ProductID > j.ProductID
				})
			} else {
				sortBy(func(i, j model.Product) bool {
					return i.ProductID < j.ProductID
				})
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
		paged = products[start:end]

		return paged, total, nil
	}
}
