package api_test

import (
	"context"
	"testing"

	"github.com/machbase/neo-server/api"
	"github.com/machbase/neo-server/api/machsvr"
	"github.com/stretchr/testify/require"
)

func TestLicense(t *testing.T) {
	var db api.Database

	if machsvr_db, err := machsvr.NewDatabase(); err != nil {
		t.Log("Error", err.Error())
		t.Fail()
	} else {
		db = api.NewDatabase(machsvr_db)
	}

	ctx := context.TODO()
	conn, err := db.Connect(ctx, api.WithTrustUser("sys"))
	require.NoError(t, err, "connect fail")
	defer conn.Close()

	lic, err := api.GetLicenseInfo(ctx, conn)
	require.NoError(t, err, "license fail")
	require.Equal(t, "00000000", lic.Id)
	require.Equal(t, "COMMUNITY", lic.Type)
	require.Equal(t, "NONE", lic.Customer)
	require.Equal(t, "NONE", lic.Project)
	require.Equal(t, "KR", lic.CountryCode)
	require.NotEmpty(t, lic.InstallDate)
	require.NotEmpty(t, lic.IssueDate)
}
