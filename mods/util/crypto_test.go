package util

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/des"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
)

func TestDecryptString(t *testing.T) {
	// AES test
	keyAES := []byte("1234567890abcdef")
	plainAES := PKCS7Pad([]byte("hello world!!!"), aes.BlockSize)
	blockAES, _ := aes.NewCipher(keyAES)
	ivAES := make([]byte, blockAES.BlockSize())
	modeAES := cipher.NewCBCEncrypter(blockAES, ivAES)
	encAES := make([]byte, len(plainAES))
	modeAES.CryptBlocks(encAES, plainAES)
	b64AES := base64.StdEncoding.EncodeToString(encAES)
	out, err := DecryptString(b64AES, "AES", string(keyAES), "")
	if err != nil || out != "hello world!!!" {
		t.Errorf("AES decrypt failed: got '%v', err=%v", out, err)
	}

	// 3DES test
	key3DES := []byte("123456789012345678901234")
	plain3DES := PKCS7Pad([]byte("hello12345678"), des.BlockSize)
	block3DES, _ := des.NewTripleDESCipher(key3DES)
	iv3DES := make([]byte, block3DES.BlockSize())
	mode3DES := cipher.NewCBCEncrypter(block3DES, iv3DES)
	enc3DES := make([]byte, len(plain3DES))
	mode3DES.CryptBlocks(enc3DES, plain3DES)
	b64_3DES := base64.StdEncoding.EncodeToString(enc3DES)
	out, err = DecryptString(b64_3DES, "3DES", string(key3DES), "")
	if err != nil || out != "hello12345678" {
		t.Errorf("3DES decrypt failed: got '%v', err=%v", out, err)
	}

	// Unsupported algorithm
	_, err = DecryptString("", "FOO", "", "")
	if err == nil || !strings.Contains(err.Error(), "unsupported algorithm") {
		t.Errorf("Unsupported algorithm should return error")
	}

	// Invalid base64
	_, err = DecryptString("notbase64", "AES", string(keyAES), "")
	if err == nil {
		t.Errorf("Invalid base64 should return error")
	}

	// Invalid key length
	_, err = DecryptString(b64AES, "AES", "shortkey", "")
	if err == nil || !strings.Contains(err.Error(), "AES key must") {
		t.Errorf("Invalid AES key length should return error")
	}
}

func TestEncryptString(t *testing.T) {
	// AES test
	keyAES := "1234567890abcdef"
	plainAES := "hello world!!!"
	encAES, err := EncryptString(plainAES, "AES", keyAES)
	if err != nil {
		t.Errorf("AES encrypt failed: %v", err)
	}
	decAES, err := DecryptString(encAES, "AES", keyAES, "")
	if err != nil || decAES != plainAES {
		t.Errorf("AES round-trip failed: got '%v', err=%v", decAES, err)
	}

	// 3DES test
	key3DES := "123456789012345678901234"
	plain3DES := "hello12345678"
	enc3DES, err := EncryptString(plain3DES, "3DES", key3DES)
	if err != nil {
		t.Errorf("3DES encrypt failed: %v", err)
	}
	dec3DES, err := DecryptString(enc3DES, "3DES", key3DES, "")
	if err != nil || dec3DES != plain3DES {
		t.Errorf("3DES round-trip failed: got '%v', err=%v", dec3DES, err)
	}

	// Unsupported algorithm
	_, err = EncryptString("foo", "FOO", "bar")
	if err == nil || !strings.Contains(err.Error(), "unsupported algorithm") {
		t.Errorf("Unsupported algorithm should return error")
	}

	// Invalid AES key length
	_, err = EncryptString("foo", "AES", "shortkey")
	if err == nil || !strings.Contains(err.Error(), "AES key must") {
		t.Errorf("Invalid AES key length should return error")
	}

	// Invalid 3DES key length
	_, err = EncryptString("foo", "3DES", "shortkey")
	if err == nil || !strings.Contains(err.Error(), "3DES key must") {
		t.Errorf("Invalid 3DES key length should return error")
	}
}

func TestValidateCypherKey(t *testing.T) {
	// Valid AES keys
	if err := ValidateCypherKey("AES", "1234567890abcdef"); err != nil {
		t.Errorf("Valid AES key reported as invalid: %v", err)
	}
	if err := ValidateCypherKey("AES", "1234567890abcdef12345678"); err != nil {
		t.Errorf("Valid AES key reported as invalid: %v", err)
	}
	if err := ValidateCypherKey("AES", "12345678901234567890123456789012"); err != nil {
		t.Errorf("Valid AES key reported as invalid: %v", err)
	}

	// Invalid AES keys
	if err := ValidateCypherKey("AES", "shortkey"); err == nil {
		t.Errorf("Invalid AES key length not detected")
	}
	if err := ValidateCypherKey("AES", "toolongkeytoolongkeytoolongkey!"); err == nil {
		t.Errorf("Invalid AES key length not detected")
	}

	// Valid 3DES key
	if err := ValidateCypherKey("3DES", "123456789012345678901234"); err != nil {
		t.Errorf("Valid 3DES key reported as invalid: %v", err)
	}

	// Invalid 3DES keys
	if err := ValidateCypherKey("3DES", "shortkey"); err == nil {
		t.Errorf("Invalid 3DES key length not detected")
	}
	if err := ValidateCypherKey("3DES", "toolongkeytoolongkey!"); err == nil {
		t.Errorf("Invalid 3DES key length not detected")
	}

	// Unsupported algorithm
	if err := ValidateCypherKey("FOO", "somekey"); err == nil {
		t.Errorf("Unsupported algorithm not detected")
	}
}

func TestQueryCypher(t *testing.T) {
	sqlText := "SELECT * FROM TAG LIMIT 3"
	cnd, _ := EncryptString(sqlText, "AES", "1234567890abcdef")
	// SkEWZMD0vnvoKYZWDtFo2alFuMVjkvdEug7JQexO5C8=
	fmt.Println(cnd)
}
