package cache

import (
	"backend/internal/model"
	"backend/internal/utils"

	"github.com/samber/lo"
)

type cache struct {
	Products utils.Cache[string, []model.Product]
}

var Cache cache

func InitCache() {
	Cache = cache{
		Products: lo.Must(utils.NewInMemoryLRUCache[string, []model.Product](30000)),
	}

}
