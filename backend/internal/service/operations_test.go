package service

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestOperationsRoleDoesNotGrantFullAdmin(t *testing.T) {
	role, err := normalizeUserRole(RoleOperator, RoleUser)
	require.NoError(t, err)
	require.Equal(t, RoleOperator, role)
	require.False(t, (&User{Role: RoleOperator}).IsAdmin())
	_, err = normalizeUserRole("superadmin", RoleUser)
	require.Error(t, err)
}
func TestOperationsRejectsNegativeOrExcessiveEntitlements(t *testing.T) {
	for _, input := range []any{&OperationsExtendInput{Days: -30}, &OperationsExtendInput{Days: 366}, &OperationsAssignInput{GroupID: 12, ValidityDays: 0, Notes: "received"}, &OperationsAssignInput{GroupID: 12, ValidityDays: 30}, &OperationsRechargeInput{Amount: 1000001, Notes: "received"}} {
		require.Error(t, ValidateOperationsInput(input))
	}
	require.NoError(t, ValidateOperationsInput(&OperationsRechargeInput{Amount: 30, Notes: "微信收款"}))
	require.NoError(t, ValidateOperationsInput(&OperationsAssignInput{GroupID: 12, ValidityDays: 30, Notes: "微信收款"}))
}

func TestOperationsCatalogValidation(t *testing.T) {
	valid := OperationsPlanInput{GroupID: 12, Name: "GPT 基础版", Price: 49, ValidityDays: 30, ForSale: true}
	require.NoError(t, ValidateOperationsInput(&valid))
	for _, price := range []float64{-1, 0, 0.001, 1000001} {
		p := valid
		p.Price = price
		require.Error(t, ValidateOperationsInput(&p))
	}
	for _, days := range []int{0, 366} {
		p := valid
		p.ValidityDays = days
		require.Error(t, ValidateOperationsInput(&p))
	}
	p := valid
	p.Name = " "
	require.Error(t, ValidateOperationsInput(&p))
}
