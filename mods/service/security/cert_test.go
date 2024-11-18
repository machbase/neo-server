package security_test

import (
	"bytes"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"testing"
	"time"

	"github.com/machbase/neo-server/v8/mods/service/security"

	"github.com/stretchr/testify/assert"
)

func TestCert(t *testing.T) {
	commonName := "SIM1_1234567890ABCDE"
	certOut := bytes.Buffer{}
	keyOut := bytes.Buffer{}
	pfxOut := bytes.Buffer{}
	validDays := 1
	format := "pkcs8"

	sshAuthorizedKeyOut := bytes.Buffer{}

	var err error
	var block *pem.Block

	block, _ = pem.Decode([]byte(testServerKey))
	caPrivKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	assert.NoError(t, err)

	block, _ = pem.Decode([]byte(testServerCert))
	caCert, err := x509.ParseCertificate(block.Bytes)
	assert.NoError(t, err)

	notBefore := time.Now()
	notAfter := notBefore.Add(time.Hour * 24 * time.Duration(validDays))

	req := &security.ClientCertReq{
		SubjectName:         commonName,
		NotBefore:           notBefore,
		NotAfter:            notAfter,
		CaCert:              caCert,
		CaPrivateKey:        caPrivKey,
		Format:              format,
		CertOut:             &certOut,
		KeyOut:              &keyOut,
		PfxOut:              &pfxOut,
		SshAuthorizedKeyOut: &sshAuthorizedKeyOut,
	}

	err = security.GenClientCert(req)
	assert.NoError(t, err)

	fmt.Println(sshAuthorizedKeyOut.String())
}

const testServerKey = `
-----BEGIN RSA PRIVATE KEY-----
MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCvnK77qoSorfdS
WiLwMLwfFYqtpQUsF1Bvy+oXengyQ4mnGWO6oayph3skjY9YhL1A6hmBiFJCTguj
Wf4MiFxU9XgwO5cP9IZRSXu3IyuCw124rgEZl1jsGku+u3OK19pDjUzwIx9Io3W6
apo/gQ148UxrpePgUrwDE8ALoQdtqjPfOkK2sYoPYLo2XycZiGBkAMKGAk3B00yD
ZHdHLjKtr1gxhBngl3ylMyXgOW9duuzZBtYHAOuyiTadYtkscAgScs3slIcpDhpZ
qZnRJETnFS3Ah3o6CAcheZFA3fXfAuuRLVSe3C4cNOHSFeZV68USbFrkbgBVpc10
f2WDr1FFAgMBAAECggEABcPfjLyG2WDIr0ftQLRg4KZc5KF3v4BOcDUiDL5FBuVn
tfgj6YMYP4KGnOcWzyGgcuqchr+ab7nPMQAp0nCBk3pxhSfXqDrvU+jVKmh5q7PN
NlxkBdqNnUapuOu/ec3nSPNxFKsaglB3c3S/dpk+f3twdlI+XmVo7bLuyZLyQvvX
BywxbThKCQQGveTXXTJwzpFa2FbcgdHuSF+9uZeWnRrveZjD1bqFULmoj+XOgbYB
sEHRdEYGVna9rmy6GJHFZb022tBeWyH2E06QtkPgVKny5s6iC90FMJ9QAVlK1L90
QwlaXSN1KGfK4XAsHTgw3InAc4GWpBHzAeMxfVUW2wKBgQDUil4TFKYiQBzNPIoA
/yMYP69aE9lTlTwSqXxOzb+gDfJQ040uGB7E5AwdAlWx+uEj6vm/rmf3uCz2bjy5
XX+yUTWHFVzVXt8iWOnKFG9kFUPZ28jz7p57vH//Z9ZMJxPTpoypXwUJ0qkklTjg
nBE2vdScGA9dZ5NM6oEbC2ix2wKBgQDThULrWE570pbyoyQQ3AKkLVd8l1GhJoeg
Yar8tQHLW42aXxD8vU3RXeuvgEM88BYfhVeMQERmU1pjn11Fq52VaVq2+3tgsz2j
jMepkAQ7IDJahr2XEngdMhuVshX6vmKCaSgHOiZ4fJjaa+qUpFcgHxhAlQk+9lTE
vOVRfKtDXwKBgQDO8JI8PcSsYIQqiKFN6xz+hTN0nxLhQNKm0QLJr6a+bhXbAL/b
e3yp8+ifbiCGFGGVmTnmmid8mISexCK30QN+WXemuPQUhDT5ulyXd2IlrlbMDiUQ
7Oq+S4DM6wtKRloVn3ohhvTe5Y/uoKQqfYp9JEOYYAzFww02vLVL4cXkNQKBgEWC
uKAgsAIPDZ4FMNf9hTywzdxa2e+MeuugzREo5sMOfjVp4mo8R7NzGv3ct7vx5kNL
jZ7Ai/nYkI7Gk19O64VrTu1tLXl0zd/OZtr5QfqwNPv85Zcc8a4ehmQmVwTExhi3
N/lQCc50m8LDzh4095DNxymKELTJPMg+j1m9D4cfAoGARi7MrCfRflI5gR1A/iGE
9j81EkoRZVcrBW6UbqXp6CSSIAyKFmRtEslA3+RX51RDV8T6MwOzrMjgINZxWuEx
BWuua15CprJlTGyu7eEsp2kqxS2u0vtBFKsYzIXlPKUr3FXX/B0SzsdMOJql7YWO
lfL1IBKJa/3lpVXGdLwkC+A=
-----END RSA PRIVATE KEY-----
`

const testServerCert = `
-----BEGIN CERTIFICATE-----
MIID5zCCAs+gAwIBAgIGAWzMqSTFMA0GCSqGSIb3DQEBCwUAMIGCMSAwHgYDVQQD
DBdkdGFnLW1xc2QuY2Fycm90aW5zLmNvbTEWMBQGA1UECwwNUGxhdGZvcm0gVGVh
bTETMBEGA1UECgwKQ2Fycm90IEluczEOMAwGA1UEBwwFU2VvdWwxFDASBgNVBAgM
C1NvdXRoIEtvcmVhMQswCQYDVQQGEwJLUjAeFw0xOTA4MjYwNjQyMTJaFw0zOTA4
MjYwNjQyMTJaMIGCMSAwHgYDVQQDDBdkdGFnLW1xc2QuY2Fycm90aW5zLmNvbTEW
MBQGA1UECwwNUGxhdGZvcm0gVGVhbTETMBEGA1UECgwKQ2Fycm90IEluczEOMAwG
A1UEBwwFU2VvdWwxFDASBgNVBAgMC1NvdXRoIEtvcmVhMQswCQYDVQQGEwJLUjCC
ASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBAK+crvuqhKit91JaIvAwvB8V
iq2lBSwXUG/L6hd6eDJDiacZY7qhrKmHeySNj1iEvUDqGYGIUkJOC6NZ/gyIXFT1
eDA7lw/0hlFJe7cjK4LDXbiuARmXWOwaS767c4rX2kONTPAjH0ijdbpqmj+BDXjx
TGul4+BSvAMTwAuhB22qM986Qraxig9gujZfJxmIYGQAwoYCTcHTTINkd0cuMq2v
WDGEGeCXfKUzJeA5b1267NkG1gcA67KJNp1i2SxwCBJyzeyUhykOGlmpmdEkROcV
LcCHejoIByF5kUDd9d8C65EtVJ7cLhw04dIV5lXrxRJsWuRuAFWlzXR/ZYOvUUUC
AwEAAaNhMF8wDgYDVR0PAQH/BAQDAgKEMB0GA1UdJQQWMBQGCCsGAQUFBwMCBggr
BgEFBQcDATAPBgNVHRMBAf8EBTADAQH/MB0GA1UdDgQWBBSVcoVcVGTeztE06Z5h
zWriQNOlcTANBgkqhkiG9w0BAQsFAAOCAQEAi9hL8rhDHLyTAJPLxxy8yjD7YD8p
VM0cHp5Zxh4BH5fxLenKP4JD3FWNamxouL4VtjE4C1Wy+APq6mxe6jBT53QU05X1
NQX2Uevs6AYlC3Vy34s2NlrR/2yA2c4s7w+RJMoSJyqwPvLKeYt9fWyHjcaj+UT1
CQO/Wf2xA9cquoZsB9Rg0VKeeW6xSgYmphejSzqrPMsPICQTkal2w//FPWbQgLkg
P5k65VclYnRH09IdvhhH2fdBbE8+U8Ka6CrpxFHjRH34QWkW0pg+hUZQGppUQ7qn
cculR/QjCcnavJa0ZkYaMVpm29jC9ULNPpig8PRvZAZ7jv5HeTZngKSaGQ==
-----END CERTIFICATE-----
`
