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
	ProductsCnt     int
	ProductsById    map[int]*model.Product
	ProductsOrdered utils.Cache[string, []model.Product]

	Order                  sync.RWMutex
	ShippingOrderProductId map[int64]int
	UserOrders             []([]model.Order)
	OrderIdUserId          map[int64]struct {
		UserID int
		Index  int
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
	log.Println("InitCache start")

	var products []model.Product
	err := dbConn.Select(&products, "SELECT * FROM products")
	if err != nil {
		log.Fatal("Failed to get products")
	}
	Cache.ProductsCnt = len(products)

	var users []model.User
	err = dbConn.Select(&users, "SELECT * FROM users")
	if err != nil {
		log.Fatal("Failed to get users")
	}

	Cache = cache{
		ProductsById:           make(map[int]*model.Product, len(products)+1),
		ShippingOrderProductId: make(map[int64]int),
		UserOrders:             make([][]model.Order, len(users)+1),
		OrderIdUserId: make(map[int64]struct {
			UserID int
			Index  int
		}, 0),
	}

	for i := range Cache.UserOrders {
		Cache.UserOrders[i] = make([]model.Order, 0)
	}

	for _, p := range products {
		Cache.ProductsById[p.ProductID] = &p
	}

	var orders []model.Order
	if err := dbConn.Select(&orders, "SELECT * FROM orders"); err != nil {
		log.Fatalf("Failed to get shipping orders: %v", err)
	}
	for _, o := range orders {
		UpdateOrder(o)
	}
	log.Println("InitCache done")
}

func UpdateOrder(order model.Order) {
	if order.ShippedStatus == "shipping" {
		Cache.ShippingOrderProductId[order.OrderID] = order.ProductID
	} else {
		delete(Cache.ShippingOrderProductId, order.OrderID)
	}

	e, ok := Cache.OrderIdUserId[order.OrderID]
	if !ok {
		Cache.UserOrders[order.UserID] = append(Cache.UserOrders[order.UserID], order)
		Cache.OrderIdUserId[order.OrderID] = struct {
			UserID int
			Index  int
		}{order.UserID, len(Cache.UserOrders[order.UserID]) - 1}
	} else {
		Cache.UserOrders[e.UserID][e.Index] = order
	}
}
