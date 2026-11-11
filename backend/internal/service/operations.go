package service

import (
	"fmt"
	"math"
	"net/mail"
	"strings"
)

type OperationsCreateInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Username string `json:"username"`
	Notes    string `json:"notes"`
}
type OperationsUpdateInput struct {
	Username    *string `json:"username"`
	Notes       *string `json:"notes"`
	Status      string  `json:"status"`
	Concurrency *int    `json:"concurrency"`
}
type OperationsRechargeInput struct {
	Amount float64 `json:"amount"`
	Notes  string  `json:"notes"`
}
type OperationsAssignInput struct {
	PlanID       int64  `json:"plan_id"`
	GroupID      int64  `json:"group_id"`
	ValidityDays int    `json:"validity_days"`
	Notes        string `json:"notes"`
}
type OperationsExtendInput struct {
	Days int `json:"days"`
}

func ValidateOperationsInput(input any) error {
	switch r := input.(type) {
	case *OperationsPlanInput:
		if strings.TrimSpace(r.Name) == "" || len(r.Name) > 100 || len(r.Description) > 2000 {
			return fmt.Errorf("请填写套餐名称（最多100字节）和有效说明")
		}
		if r.GroupID <= 0 || math.IsNaN(r.Price) || math.IsInf(r.Price, 0) || r.Price <= 0 || r.Price > 1000000 || math.Abs(r.Price*100-math.Round(r.Price*100)) > 0.000001 {
			return fmt.Errorf("价格需大于0，最多两位小数")
		}
		if r.ValidityDays < 1 || r.ValidityDays > 365 || r.SortOrder < 0 || r.SortOrder > 10000 {
			return fmt.Errorf("有效期为1至365天，排序为0至10000")
		}

	case *OperationsCreateInput:
		a, err := mail.ParseAddress(r.Email)
		if err != nil || a.Address != r.Email || len(r.Email) > 254 {
			return fmt.Errorf("请填写有效邮箱")
		}
		if err := ValidateNewPassword(r.Password); err != nil {
			return err
		}
		if len(r.Username) > 100 || len(r.Notes) > 2000 {
			return fmt.Errorf("客户资料过长")
		}
	case *OperationsUpdateInput:
		if r.Status != "" && r.Status != StatusActive && r.Status != StatusDisabled {
			return fmt.Errorf("客户状态无效")
		}
		if r.Concurrency != nil && (*r.Concurrency < 1 || *r.Concurrency > 2) {
			return fmt.Errorf("客户并发只能为 1 或 2")
		}
		if r.Username != nil && len(*r.Username) > 100 || r.Notes != nil && len(*r.Notes) > 2000 {
			return fmt.Errorf("客户资料过长")
		}
	case *OperationsRechargeInput:
		if math.IsNaN(r.Amount) || math.IsInf(r.Amount, 0) || r.Amount <= 0 || r.Amount > 1000000 {
			return fmt.Errorf("充值金额需大于 0 且不超过 100 万")
		}
		if strings.TrimSpace(r.Notes) == "" || len(r.Notes) > 2000 {
			return fmt.Errorf("请填写充值收款备注（最多 2000 字节）")
		}
	case *OperationsAssignInput:
		if r.GroupID <= 0 || r.ValidityDays < 1 || r.ValidityDays > 365 {
			return fmt.Errorf("套餐及有效天数无效（1 至 365 天）")
		}
		if strings.TrimSpace(r.Notes) == "" || len(r.Notes) > 2000 {
			return fmt.Errorf("请填写套餐收款备注（最多 2000 字节）")
		}
	case *OperationsExtendInput:
		if r.Days < 1 || r.Days > 365 {
			return fmt.Errorf("续期天数需为 1 至 365 天")
		}
	default:
		return fmt.Errorf("不支持的运营操作")
	}
	return nil
}

// Catalog authors may change commercial terms, never upstream or group permissions.
type OperationsPlanInput struct {
	GroupID      int64   `json:"group_id"`
	Name         string  `json:"name"`
	Description  string  `json:"description"`
	Price        float64 `json:"price"`
	ValidityDays int     `json:"validity_days"`
	ForSale      bool    `json:"for_sale"`
	SortOrder    int     `json:"sort_order"`
}
