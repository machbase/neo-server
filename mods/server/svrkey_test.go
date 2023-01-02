package server_test

import (
	"fmt"
	"testing"

	. "github.com/machbase/dbms-mach-go/server"
	"github.com/stretchr/testify/require"
)

func TestKeyGen(t *testing.T) {
	ec := NewEllipticCurveP521()
	pri, pub, err := ec.GenerateKeys()
	require.Nil(t, err)
	require.NotNil(t, pri)
	require.NotNil(t, pub)

	pripem, err := ec.EncodePrivate(pri)
	require.Nil(t, err)
	require.NotEmpty(t, pripem)

	pubpem, err := ec.EncodePublic(pub)
	require.Nil(t, err)
	require.NotEmpty(t, pubpem)

	fmt.Println(pripem)
	fmt.Println(pubpem)
}
