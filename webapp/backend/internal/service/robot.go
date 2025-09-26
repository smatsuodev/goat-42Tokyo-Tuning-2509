package service

import (
	"backend/internal/model"
	"backend/internal/repository"
	"backend/internal/service/utils"
	"context"
	"log"
)

type RobotService struct {
	store *repository.Store
}

func NewRobotService(store *repository.Store) *RobotService {
	return &RobotService{store: store}
}

func (s *RobotService) GenerateDeliveryPlan(ctx context.Context, robotID string, capacity int) (*model.DeliveryPlan, error) {
	var plan model.DeliveryPlan

	err := utils.WithTimeout(ctx, func(ctx context.Context) error {
		return s.store.ExecTx(ctx, func(txStore *repository.Store) error {
			orders, err := txStore.OrderRepo.GetShippingOrders(ctx)
			if err != nil {
				return err
			}
			plan, err = selectOrdersForDelivery(ctx, orders, robotID, capacity)
			if err != nil {
				return err
			}
			if len(plan.Orders) > 0 {
				orderIDs := make([]int64, len(plan.Orders))
				for i, order := range plan.Orders {
					orderIDs[i] = order.OrderID
				}

				if err := txStore.OrderRepo.UpdateStatuses(ctx, orderIDs, "delivering"); err != nil {
					return err
				}
				log.Printf("Updated status to 'delivering' for %d orders", len(orderIDs))
			}
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	return &plan, nil
}

func (s *RobotService) UpdateOrderStatus(ctx context.Context, orderID int64, newStatus string) error {
	return utils.WithTimeout(ctx, func(ctx context.Context) error {
		return s.store.OrderRepo.UpdateStatuses(ctx, []int64{orderID}, newStatus)
	})
}

func selectOrdersForDelivery(ctx context.Context, orders []model.Order, robotID string, robotCapacity int) (model.DeliveryPlan, error) {
	n := len(orders)
	steps := 0
	checkEvery := 16384
	canceled := false

	// dp[i][w] = i番目までの品物を使って、重さw以下で実現できる最大価値
	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, robotCapacity+1)
	}

Loop:
	// robotCapacityがそんなに大きくならないのであれば、timeoutのチェックは外のループでやった方がいいかも
	// 大きくないの基準は、robotCapacityが16384以下とか
	// ぴったり16384回に1回チェックではなく、16384の倍数を超えたら1回チェックするようにするような修正を加えると良さそう
	for i := 1; i <= n; i++ {
		order := orders[i-1]
		for w := 0; w <= robotCapacity; w++ {
			// timeoutのチェック
			steps++
			if checkEvery > 0 && steps%checkEvery == 0 {
				select {
				case <-ctx.Done():
					canceled = true
					break Loop
				default:
				}
			}

			// i番目の品物を選ばない場合
			dp[i][w] = dp[i-1][w]

			// i番目の品物を選ぶ場合
			if w >= order.Weight && dp[i][w] < dp[i-1][w-order.Weight]+order.Value {
				dp[i][w] = dp[i-1][w-order.Weight] + order.Value
			}
		}
	}

	if canceled {
		return model.DeliveryPlan{}, ctx.Err()
	}

	bestValue := dp[n][robotCapacity]
	useForBest := make([]bool, n+1)
	var totalWeight int

	// DPテーブルから最適な組み合わせを復元
	w := robotCapacity
	for i := n; i > 0; i-- {
		// i番目の品物が選ばれたか判定
		order := orders[i-1]
		// dp[i][w]の値は、dp[i-1][w]とdp[i-1][w-order.Weight]+order.Valueのどちらか大きい方なので、
		// dp[i][w] != dp[i-1][w]ならi番目の品物が選ばれたことになる
		if dp[i][w] != dp[i-1][w] {
			useForBest[i] = true
			w -= order.Weight
			totalWeight += order.Weight
		}
	}
	bestSet := make([]model.Order, 0, n)

	index := 0
	for i := 1; i <= n; i++ {
		if useForBest[i] {
			bestSet = append(bestSet, orders[index])
			index++
		}
	}

	return model.DeliveryPlan{
		RobotID:     robotID,
		TotalWeight: totalWeight,
		TotalValue:  bestValue,
		Orders:      bestSet,
	}, nil
}

// func selectOrdersForDelivery(ctx context.Context, orders []model.Order, robotID string, robotCapacity int) (model.DeliveryPlan, error) {
// 	n := len(orders)
// 	bestValue := 0
// 	var bestSet []model.Order
// 	steps := 0
// 	checkEvery := 16384

// 	var dfs func(i, curWeight, curValue int, curSet []model.Order) bool
// 	dfs = func(i, curWeight, curValue int, curSet []model.Order) bool {
// 		if curWeight > robotCapacity {
// 			return false
// 		}
// 		steps++
// 		if checkEvery > 0 && steps%checkEvery == 0 {
// 			select {
// 			case <-ctx.Done():
// 				return true
// 			default:
// 			}
// 		}
// 		if i == n {
// 			if curValue > bestValue {
// 				bestValue = curValue
// 				bestSet = append([]model.Order{}, curSet...)
// 			}
// 			return false
// 		}

// 		if dfs(i+1, curWeight, curValue, curSet) {
// 			return true
// 		}

// 		order := orders[i]
// 		return dfs(i+1, curWeight+order.Weight, curValue+order.Value, append(curSet, order))
// 	}

// 	canceled := dfs(0, 0, 0, nil)
// 	if canceled {
// 		return model.DeliveryPlan{}, ctx.Err()
// 	}

// 	var totalWeight int
// 	for _, o := range bestSet {
// 		totalWeight += o.Weight
// 	}

// 	return model.DeliveryPlan{
// 		RobotID:     robotID,
// 		TotalWeight: totalWeight,
// 		TotalValue:  bestValue,
// 		Orders:      bestSet,
// 	}, nil
// }
