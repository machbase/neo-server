package testsuite

import (
	"context"
	"testing"

	"github.com/machbase/neo-server/v8/api"
	"github.com/stretchr/testify/require"
)

func License(t *testing.T, db api.Database, ctx context.Context) {
	conn, err := db.Connect(ctx, api.WithPassword("sys", "manager"))
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
	require.NotEmpty(t, lic.LicenseStatus)
}
