package repository

import (
	cache "backend/internal"
	"backend/internal/model"
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync"
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
		products = SearchProducts(cache.Cache.ProductsById, req.Search)

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
func SearchProducts(productsById []model.Product, query string) []model.Product {
	numWorkers := runtime.NumCPU() // 並列度をCPU数に合わせる
	chunkSize := (len(productsById) + numWorkers - 1) / numWorkers

	var wg sync.WaitGroup
	resultCh := make(chan *model.Product, len(productsById))

	for i := 0; i < len(productsById); i += chunkSize {
		end := i + chunkSize
		if end > len(productsById) {
			end = len(productsById)
		}
		wg.Add(1)

		// 部分配列を goroutine で処理
		go func(subset []model.Product) {
			defer wg.Done()
			for _, p := range subset {
				if strings.Contains(p.Name, query) || strings.Contains(p.Description, query) {
					resultCh <- &p
				}
			}
		}(productsById[i:end])
	}

	// 結果をまとめる goroutine
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	// 最終結果を収集
	var results []model.Product
	for p := range resultCh {
		results = append(results, *p)
	}
	return results
}
