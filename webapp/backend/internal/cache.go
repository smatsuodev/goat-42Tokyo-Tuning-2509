package cache

import (
	"backend/internal/model"
	"backend/internal/utils"
	"context"
	"log"
	"sort"
	"sync"

	"github.com/jmoiron/sqlx"
	"github.com/samber/lo"
)

type cache struct {
	Products               []model.Product
	ProductsById           utils.Cache[int, *model.Product]
	ProductsOrdered        utils.Cache[string, []model.Product]
	ShippingOrderProductId struct {
		Values map[int64]int
		Mu     sync.RWMutex
		IsInit bool
	}
}

var Cache cache

func InitCache(dbConn *sqlx.DB) {
	Cache = cache{
		ProductsById:    lo.Must(utils.NewInMemoryLRUCache[int, *model.Product](300000)),
		ProductsOrdered: lo.Must(utils.NewInMemoryLRUCache[string, []model.Product](300000)),
		ShippingOrderProductId: struct {
			Values map[int64]int
			Mu     sync.RWMutex
			IsInit bool
		}{Values: make(map[int64]int)},
	}
	err := dbConn.Select(&Cache.Products, "SELECT * FROM products")
	if err != nil {
		log.Fatal("Failed to get products")
	}
	sort.SliceStable(Cache.Products, func(i, j int) bool {
		return Cache.Products[i].ProductID < Cache.Products[j].ProductID
	})

	for _, p := range Cache.Products {
		Cache.ProductsById.Set(context.Background(), p.ProductID, &p)
	}

	for _, key := range []string{"description", "image", "name", "value", "weight", "product_id"} {
		for _, sortOrder := range []string{"", "asc", "desc"} {
			baseQuery := `
		SELECT product_id, name, value, weight, image, description
		FROM products
	`
			baseQuery += " ORDER BY " + key + " " + sortOrder + " , product_id ASC"

			var products []model.Product
			err := dbConn.Select(&products, baseQuery)
			if err != nil {
				log.Fatalf("Failed to get products ordered: %v", err)
			}
			Cache.ProductsOrdered.Set(context.TODO(), key+" "+sortOrder, products)
			log.Printf("Cache.ProductsOrdered.Set: key=%s,size=%d", key+" "+sortOrder, len(products))
		}
	}
}
