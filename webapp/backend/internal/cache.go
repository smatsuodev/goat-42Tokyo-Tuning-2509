package cache

import (
	"backend/internal/model"
	"backend/internal/utils"
	"log"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
)

type cache struct {
	// Products               []model.Product
	ProductsCnt            int
	ProductsById           map[int]*model.Product
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
	dbConn.Exec("DROP TABLE cache")

	var products []model.Product
	err := dbConn.Select(&products, "SELECT * FROM products")
	if err != nil {
		log.Fatal("Failed to get products")
	}
	Cache.ProductsCnt = len(products)

	log.Println("InitCache start")
	Cache = cache{
		ProductsById: make(map[int]*model.Product, len(products)),
		ShippingOrderProductId: struct {
			Values map[int64]int
			Mu     sync.RWMutex
		}{Values: make(map[int64]int)},
	}

	for _, p := range products {
		Cache.ProductsById[p.ProductID] = &p
	}

	var orders []model.Order
	if err := dbConn.Select(&orders, "SELECT * FROM orders WHERE shipped_status = 'shipping' "); err != nil {
		log.Fatalf("Failed to get shipping orders: %v", err)
	}
	for _, o := range orders {
		Cache.ShippingOrderProductId.Values[o.OrderID] = o.ProductID
	}
	log.Println("InitCache done")
}
