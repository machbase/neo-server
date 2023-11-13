package mods

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVersion(t *testing.T) {
	versionString = "v1.2.3-rc1"
	versionGitSHA = "11f32f31"
	buildTimestamp = "2023/08/23T11:22"
	goVersionString = "1.20.2"
	editionString = "standard_edition"

	ver := GetVersion()
	require.NotNil(t, ver)
	require.Equal(t, 1, ver.Major)
	require.Equal(t, 2, ver.Minor)
	require.Equal(t, 3, ver.Patch)
	require.Equal(t, "11f32f31", ver.GitSHA)
	require.Equal(t, "standard_edition", ver.Edition)
	require.Equal(t, "V1.2.3-RC1", DisplayVersion())
	require.Equal(t, "V1.2.3-RC1 (11f32f31 2023/08/23T11:22)", VersionString())
	require.Equal(t, "1.20.2", BuildCompiler())
	require.Equal(t, "2023/08/23T11:22", BuildTimestamp())
	require.Equal(t, "standard_edition", Edition())
}
