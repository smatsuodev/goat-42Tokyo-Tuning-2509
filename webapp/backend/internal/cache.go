package cache

import (
	"backend/internal/model"
	"backend/internal/utils"
	"context"
	"log"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/samber/lo"
)

type cache struct {
	// Products               []model.Product
	ProductsCnt            int
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
	ctx := context.TODO()

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

	var products []model.Product
	err := dbConn.Select(&products, "SELECT * FROM products")
	if err != nil {
		log.Fatal("Failed to get products")
	}
	Cache.ProductsCnt = len(products)
	for _, p := range products {
		Cache.ProductsById.Set(ctx, p.ProductID, p)
	}

	var orders []model.Order
	if err := dbConn.Select(&orders, "SELECT * FROM orders WHERE shipped_status = 'shipping' "); err != nil {
		log.Fatalf("Failed to get shipping orders: %v", err)
	}
	for _, o := range orders {
		Cache.ShippingOrderProductId.Values[o.OrderID] = o.ProductID
	}
}
