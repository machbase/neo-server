package api

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

func GetUsers(ctx context.Context, conn Conn) ([]string, error) {
	rows, err := conn.Query(ctx, "SELeCT NAME FROM M$SYS_USERS")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []string
	for rows.Next() {
		var user string
		if err := rows.Scan(&user); err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, nil
}

func GetLicenseInfo(ctx context.Context, conn Conn) (*LicenseInfo, error) {
	ret := &LicenseInfo{}
	var violateStatus int
	row := conn.QueryRow(ctx, "select ID, TYPE, CUSTOMER, PROJECT, COUNTRY_CODE, INSTALL_DATE, ISSUE_DATE, VIOLATE_STATUS, VIOLATE_MSG from v$license_info")
	if err := row.Scan(&ret.Id, &ret.Type, &ret.Customer, &ret.Project, &ret.CountryCode, &ret.InstallDate, &ret.IssueDate, &violateStatus, &ret.LicenseStatus); err != nil {
		return nil, err
	}
	if violateStatus == 0 {
		ret.LicenseStatus = "Valid"
	}
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
