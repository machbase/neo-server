package do

import (
	"os"

	spi "github.com/machbase/neo-spi"
)

type LicenseInfo struct {
	Id          string `json:"id"`
	Type        string `json:"type"`
	Customer    string `json:"customer"`
	Project     string `json:"project"`
	CountryCode string `json:"countryCode"`
	InstallDate string `json:"installDate"`
	IssueDate   string `json:"issueDate"`
}

func GetLicenseInfo(db spi.Database) (*LicenseInfo, error) {
	ret := &LicenseInfo{}
	row := db.QueryRow("select ID, TYPE, CUSTOMER, PROJECT, COUNTRY_CODE, INSTALL_DATE, ISSUE_DATE from v$license_info")
	if err := row.Scan(&ret.Id, &ret.Type, &ret.Customer, &ret.Project, &ret.CountryCode, &ret.InstallDate, &ret.IssueDate); err != nil {
		return nil, err
	}
	return ret, nil
}

func InstallLicenseFile(db spi.Database, path string) (*LicenseInfo, error) {
	// alter system install license='path_to/license.dat';
	result := db.Exec("alter system install license=?", path)
	if result.Err() != nil {
		return nil, result.Err()
	}
	return GetLicenseInfo(db)
}

func InstallLicenseData(db spi.Database, licenseFilePath string, content []byte) (*LicenseInfo, error) {
	_, err := os.Stat(licenseFilePath)
	if err != nil && err != os.ErrNotExist {
		return nil, err
	}
	if err := os.WriteFile(licenseFilePath, content, 0640); err != nil {
		return nil, err
	}
	return InstallLicenseFile(db, licenseFilePath)
}
