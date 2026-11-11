package service

import (
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

func TestPasswordPolicy(t *testing.T) {
	for _, password := range []string{"six123", "short-password", strings.Repeat("a", 20), "passwordpassword", strings.Repeat("界", 25)} {
		require.Error(t, ValidateNewPassword(password))
	}
	for _, password := range []string{"unique river amber planet", "Correct-Horse-29!", "春夏秋冬天地山川风云日月星辰海"} {
		require.NoError(t, ValidateNewPassword(password))
	}
	u := &User{}
	require.Error(t, u.SetPassword("six123"))
	require.NoError(t, u.SetPassword("unique river amber planet"))
	require.True(t, u.CheckPassword("unique river amber planet"))
}
