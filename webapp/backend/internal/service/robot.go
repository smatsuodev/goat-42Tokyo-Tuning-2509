package service

import (
	"backend/internal/model"
	"backend/internal/repository"
	"backend/internal/service/utils"
	"context"
	"log"
)

const (
	smallProblemThreshold = 20000
)

type RobotService struct {
	store *repository.Store
}

func NewRobotService(store *repository.Store) *RobotService {
	return &RobotService{store: store}
}

func (s *RobotService) GenerateDeliveryPlan(ctx context.Context, robotID string, capacity int) (*model.DeliveryPlan, error) {
	var plan model.DeliveryPlan

	err := s.store.ExecTx(ctx, func(txStore *repository.Store) error {
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

func (s *RobotService) HasShippingOrders(ctx context.Context) (bool, error) {
	var hasOrders bool
	err := utils.WithTimeout(ctx, func(ctx context.Context) error {
		count, err := s.store.OrderRepo.CountShippingOrders(ctx)
		if err != nil {
			return err
		}
		hasOrders = count > 0
		return nil
	})
	if err != nil {
		return false, err
	}
	return hasOrders, nil
}

func selectOrdersForDelivery(ctx context.Context, orders []model.Order, robotID string, robotCapacity int) (model.DeliveryPlan, error) {
	_ = ctx
	prunedOrders := make([]model.Order, 0, len(orders))
	for _, order := range orders {
		if order.Weight <= robotCapacity {
			prunedOrders = append(prunedOrders, order)
		}
	}

	wTotal := 0
	for _, order := range prunedOrders {
		wTotal += order.Weight
	}
	if wTotal <= robotCapacity {
		vTotal := 0
		for _, order := range prunedOrders {
			vTotal += order.Value
		}
		return model.DeliveryPlan{
			RobotID:     robotID,
			TotalWeight: wTotal,
			TotalValue:  vTotal,
			Orders:      orders,
		}, nil
	}

	bestSet := findBestSetRecursive(prunedOrders, robotCapacity)
	bestValue := 0
	totalWeight := 0
	for _, order := range bestSet {
		bestValue += order.Value
		totalWeight += order.Weight
	}

	return model.DeliveryPlan{
		RobotID:     robotID,
		TotalWeight: totalWeight,
		TotalValue:  bestValue,
		Orders:      bestSet,
	}, nil
}

// 再帰的に最適な品物セットを見つける関数
func findBestSetRecursive(orders []model.Order, capacity int) []model.Order {
	n := len(orders)

	// ベースケース: 品物が1つ以下なら、容量に入るか単純に判断
	if n <= 1 {
		if n == 1 && orders[0].Weight <= capacity {
			return orders
		}
		return []model.Order{}
	}

	if n*capacity <= smallProblemThreshold {
		return solveKnapsackIterative(orders, capacity)
	}

	// 1. 分割
	mid := n / 2
	firstHalf := orders[:mid]
	secondHalf := orders[mid:]

	// 2a. 順方向DP (空間計算量 O(capacity))
	dpForward := calculateMaxValues(firstHalf, capacity)

	// 2b. 逆方向DP (空間計算量 O(capacity))
	dpBackward := calculateMaxValues(secondHalf, capacity)

	// 3. 最適分割点 w_split を発見
	bestValue := -1
	wSplit := -1
	for w := 0; w <= capacity; w++ {
		currentValue := dpForward[w] + dpBackward[capacity-w]
		if currentValue > bestValue {
			bestValue = currentValue
			wSplit = w
		}
	}

	// 4. 統治 (再帰)
	solution1 := findBestSetRecursive(firstHalf, wSplit)
	solution2 := findBestSetRecursive(secondHalf, capacity-wSplit)

	return append(solution1, solution2...)
}

func solveKnapsackIterative(orders []model.Order, capacity int) []model.Order {
	n := len(orders)
	if n == 0 || capacity <= 0 {
		return []model.Order{}
	}

	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, capacity+1)
	}

	for i := 1; i <= n; i++ {
		order := orders[i-1]
		for w := 0; w <= capacity; w++ {
			dp[i][w] = dp[i-1][w]
			if order.Weight <= w {
				candidate := dp[i-1][w-order.Weight] + order.Value
				if candidate > dp[i][w] {
					dp[i][w] = candidate
				}
			}
		}
	}

	result := make([]model.Order, 0)
	w := capacity
	for i := n; i > 0 && w >= 0; i-- {
		if dp[i][w] != dp[i-1][w] {
			order := orders[i-1]
			result = append(result, order)
			w -= order.Weight
		}
	}

	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}
	return result
}

// 空間計算量 O(capacity) で最大価値の配列を計算するヘルパー関数
func calculateMaxValues(orders []model.Order, capacity int) []int {
	dp := make([]int, capacity+1)
	for _, order := range orders {
		for w := capacity; w >= order.Weight; w-- {
			// 価値を更新
			if dp[w] < dp[w-order.Weight]+order.Value {
				dp[w] = dp[w-order.Weight] + order.Value
			}
		}
	}
	return dp
}
