package spi

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestIssueTokenAndVerifyToken(t *testing.T) {
	token := IssueToken()
	require.NotEmpty(t, token)
	require.Contains(t, token, ":")

	valid := VerifyToken(token, 10*time.Second)
	require.True(t, valid)
}

func TestVerifyTokenMalformedToken(t *testing.T) {
	require.False(t, VerifyToken("not-a-token", 10*time.Second))
	require.False(t, VerifyToken("1234567890:", 10*time.Second))
}

func TestVerifyTokenTamperedSignature(t *testing.T) {
	token := IssueToken()
	require.NotEmpty(t, token)

	parts := strings.SplitN(token, ":", 2)
	require.Len(t, parts, 2)

	tampered := parts[0] + ":" + parts[1] + "a"
	require.False(t, VerifyToken(tampered, 10*time.Second))
}

func TestVerifyTokenExpired(t *testing.T) {
	token := IssueToken()
	require.NotEmpty(t, token)

	time.Sleep(5 * time.Millisecond)
	require.False(t, VerifyToken(token, 1*time.Millisecond))
}
