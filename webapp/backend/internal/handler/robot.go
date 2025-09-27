package handler

import (
	"backend/internal/model"
	"backend/internal/service"
	"backend/internal/service/utils"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"
)

type RobotHandler struct {
	RobotSvc *service.RobotService
}

func NewRobotHandler(robotSvc *service.RobotService) *RobotHandler {
	return &RobotHandler{RobotSvc: robotSvc}
}

// 配送計画を取得
func (h *RobotHandler) GetDeliveryPlan(w http.ResponseWriter, r *http.Request) {
	capacityStr := r.URL.Query().Get("capacity")
	if capacityStr == "" {
		http.Error(w, "Query parameter 'capacity' is required", http.StatusBadRequest)
		return
	}
	capacity, err := strconv.Atoi(capacityStr)
	if err != nil {
		http.Error(w, "Query parameter 'capacity' must be an integer", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	err = utils.WithTimeout(ctx, func(ctx context.Context) error {
		for {
			hasOrders, err := h.RobotSvc.HasShippingOrders(ctx)
			if err != nil {
				return err
			}
			if hasOrders {
				break
			}

			select {
			case <-ctx.Done():
				log.Printf("Request cancelled while waiting for shipping orders: %v", ctx.Err())
				return nil
			case <-time.After(100 * time.Millisecond):
			}
		}
		return nil
	})
	if err != nil {
		log.Printf("Failed to check shipping orders: %v", err)
		http.Error(w, "Failed to check shipping orders", http.StatusInternalServerError)
	}

	plan, err := h.RobotSvc.GenerateDeliveryPlan(r.Context(), "robot-001", capacity)
	if err != nil {
		log.Printf("Failed to generate delivery plan: %v", err)
		http.Error(w, "Failed to create delivery plan", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(plan)
}

// 配送完了時に注文ステータスを更新
func (h *RobotHandler) UpdateOrderStatus(w http.ResponseWriter, r *http.Request) {
	var req model.UpdateOrderStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	err := h.RobotSvc.UpdateOrderStatus(r.Context(), req.OrderID, req.NewStatus)
	if err != nil {
		log.Printf("Failed to update order status for order %d: %v", req.OrderID, err)
		http.Error(w, "Failed to update order status", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Order status updated"))
}
