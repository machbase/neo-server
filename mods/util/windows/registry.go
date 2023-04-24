//go:build windows

package windows

import (
	"golang.org/x/sys/windows/registry"
)

type Product struct {
	Name                      string
	BuildLab                  string
	CurrentVersion            string
	CurrentMajorVersionNumber int
	CurrentMinorVersionNumber int
	CurrentBuildNumber        int
}

func ProductName() (Product, error) {
	prd := Product{}

	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows NT\CurrentVersion`, registry.QUERY_VALUE)
	if err != nil {
		return prd, err
	}
	defer k.Close()

	bl, _, err := k.GetStringValue("BuildLab")
	if err != nil {
		return prd, err
	}
	prd.BuildLab = bl

	pn, _, err := k.GetStringValue("ProductName")
	if err != nil {
		return prd, err
	}
	prd.Name = pn

	cv, _, err := k.GetStringValue("CurrentVersion")
	if err != nil {
		return prd, err
	}
	prd.CurrentVersion = cv

	cb, _, err := k.GetIntegerValue("CurrentBuildNumber")
	if err != nil {
		return prd, err
	}
	prd.CurrentBuildNumber = cb

	// supported since Windows 10
	if maj, _, err := k.GetIntegerValue("CurrentMajorVersionNumber"); err == nil {
		prd.CurrentMajorVersionNumber = maj
	}

	// supported since Windows 10
	if min, _, err := k.GetIntegerValue("CurrentMinorVersionNumber"); err == nil {
		prd.CurrentMinorVersionNumber = min
	}

	return prd, nil
}
