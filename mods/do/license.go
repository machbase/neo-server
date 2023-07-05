package do

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

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
	if strings.ContainsRune(path, ';') {
		return nil, errors.New("invalid license file path")
	}
	result := db.Exec("alter system install license='" + path + "'")
	if result.Err() != nil {
		return nil, result.Err()
	}
	return GetLicenseInfo(db)
}

func InstallLicenseData(db spi.Database, licenseFilePath string, content []byte) (*LicenseInfo, error) {
	_, err := os.Stat(licenseFilePath)
	if err == nil {
		// backup existing file
		os.Rename(licenseFilePath, fmt.Sprintf("%s_%s", licenseFilePath, time.Now().Format("20060102_150405")))
	}
	if err := os.WriteFile(licenseFilePath, content, 0640); err != nil {
		return nil, err
	}
	return InstallLicenseFile(db, licenseFilePath)
}
