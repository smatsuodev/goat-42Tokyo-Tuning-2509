package cache

import (
	"backend/internal/model"
	"backend/internal/utils"
	"context"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/samber/lo"
)

type cache struct {
	Products               []model.Product
	ProductsById           utils.Cache[int, model.Product]
	ProductsOrdered        utils.Cache[string, []model.Product]
	ShippingOrderProductId struct {
		Values map[int64]int
		Mu     sync.RWMutex
	}
}

var Cache cache

func InitCache(dbConn *sqlx.DB) {
	var tmp int

	for {
		err := dbConn.Get(&tmp, "SELECT COUNT(*) FROM cache")
		if err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	Cache = cache{
		ProductsById:    lo.Must(utils.NewInMemoryLRUCache[int, model.Product](300000)),
		ProductsOrdered: lo.Must(utils.NewInMemoryLRUCache[string, []model.Product](300000)),
		ShippingOrderProductId: struct {
			Values map[int64]int
			Mu     sync.RWMutex
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
		Cache.ProductsById.Set(context.Background(), p.ProductID, p)
	}

	for _, key := range []string{"description", "image", "name", "value", "weight", "product_id"} {
		for _, sortOrder := range []string{"", "asc", "desc"} {
			products := make([]model.Product, len(Cache.Products))
			copy(products, Cache.Products)
			sort.SliceStable(products, func(i, j int) bool {
				switch key {
				case "description":
					if sortOrder == "desc" {
						return products[i].Description > products[j].Description
					} else {
						return products[i].Description < products[j].Description
					}
				case "image":
					if sortOrder == "desc" {
						return products[i].Image > products[j].Image
					} else {
						return products[i].Image < products[j].Image
					}
				case "name":
					if sortOrder == "desc" {
						return products[i].Name > products[j].Name
					} else {
						return products[i].Name < products[j].Name
					}
				case "product_id":
					if sortOrder == "desc" {
						return products[i].ProductID > products[j].ProductID
					} else {
						return products[i].ProductID < products[j].ProductID
					}
				case "value":
					if sortOrder == "desc" {
						return products[i].Value > products[j].Value
					} else {
						return products[i].Value < products[j].Value
					}
				case "weight":
					if sortOrder == "desc" {
						return products[i].Weight > products[j].Weight
					} else {
						return products[i].Weight < products[j].Weight
					}
				default:
					// todo: 例外処理
					return true
				}
			})
			if err != nil {
				log.Fatalf("Failed to get products ordered: %v", err)
			}
			Cache.ProductsOrdered.Set(context.TODO(), key+" "+sortOrder, products)
			log.Printf("Cache.ProductsOrdered.Set: key=%s,size=%d", key+" "+sortOrder, len(products))
		}
	}

	var orders []model.Order
	if err := dbConn.Select(&orders, "SELECT * FROM orders WHERE shipped_status = 'shipping' "); err != nil {
		log.Fatalf("Failed to get shipping orders: %v", err)
	}
	for _, o := range orders {
		Cache.ShippingOrderProductId.Values[o.OrderID] = o.ProductID
	}
}
