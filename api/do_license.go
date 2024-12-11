package api

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

func GetLicenseInfo(ctx context.Context, conn Conn) (*LicenseInfo, error) {
	ret := &LicenseInfo{}
	row := conn.QueryRow(ctx, "select ID, TYPE, CUSTOMER, PROJECT, COUNTRY_CODE, INSTALL_DATE, ISSUE_DATE from v$license_info")
	if err := row.Scan(&ret.Id, &ret.Type, &ret.Customer, &ret.Project, &ret.CountryCode, &ret.InstallDate, &ret.IssueDate); err != nil {
		return nil, err
	}
	// TODO: get license status
	ret.LicenseStatus = "Valid"
	return ret, nil
}

func InstallLicenseFile(ctx context.Context, conn Conn, path string) (*LicenseInfo, error) {
	if strings.ContainsRune(path, ';') {
		return nil, errors.New("invalid license file path")
	}
	result := conn.Exec(ctx, "alter system install license='"+path+"'")
	if result.Err() != nil {
		return nil, result.Err()
	}
	return GetLicenseInfo(ctx, conn)
}

func InstallLicenseData(ctx context.Context, conn Conn, licenseFilePath string, content []byte) (*LicenseInfo, error) {
	_, err := os.Stat(licenseFilePath)
	if err == nil {
		// backup existing file
		os.Rename(licenseFilePath, fmt.Sprintf("%s_%s", licenseFilePath, time.Now().Format("20060102_150405")))
	}
	if err := os.WriteFile(licenseFilePath, content, 0640); err != nil {
		return nil, err
	}
	return InstallLicenseFile(ctx, conn, licenseFilePath)
}
