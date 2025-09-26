package cache

import (
	"backend/internal/model"
	"log"
	"sort"

	"github.com/jmoiron/sqlx"
)

type cache struct {
	Products []model.Product
}

var Cache cache

func InitCache(dbConn *sqlx.DB) {
	Cache = cache{}
	err := dbConn.Select(&Cache.Products, "SELECT * FROM products")
	if err != nil {
		log.Fatal("Failed to get products")
	}
	sort.SliceStable(Cache.Products, func(i, j int) bool {
		return Cache.Products[i].ProductID < Cache.Products[j].ProductID
	})
}
