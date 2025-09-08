package util

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/des"
	"encoding/base64"
	"errors"
	"strings"
)

func ValidateCypherKey(alg string, key string) error {
	switch strings.ToUpper(alg) {
	case "3-DES", "3DES", "DES3":
		k := []byte(key)
		if len(k) != 24 {
			return errors.New("3DES key must be 24 bytes")
		}
		return nil
	case "AES":
		k := []byte(key)
		if len(k) != 16 && len(k) != 24 && len(k) != 32 {
			return errors.New("AES key must be 16, 24, or 32 bytes")
		}
		return nil
	default:
		return errors.New("unsupported algorithm: " + alg)
	}
}

func EncryptString(plainStr string, alg string, key string) (string, error) {
	data := []byte(plainStr)
	switch strings.ToUpper(alg) {
	case "3-DES", "3DES", "DES3":
		k := []byte(key)
		if len(k) != 24 {
			return "", errors.New("3DES key must be 24 bytes")
		}
		block, err := des.NewTripleDESCipher(k)
		if err != nil {
			return "", err
		}
		blockSize := block.BlockSize()
		padded := PKCS7Pad(data, blockSize)
		iv := make([]byte, blockSize)
		mode := cipher.NewCBCEncrypter(block, iv)
		encrypted := make([]byte, len(padded))
		mode.CryptBlocks(encrypted, padded)
		return base64.StdEncoding.EncodeToString(encrypted), nil
	case "AES":
		k := []byte(key)
		if len(k) != 16 && len(k) != 24 && len(k) != 32 {
			return "", errors.New("AES key must be 16, 24, or 32 bytes")
		}
		block, err := aes.NewCipher(k)
		if err != nil {
			return "", err
		}
		blockSize := block.BlockSize()
		padded := PKCS7Pad(data, blockSize)
		iv := make([]byte, blockSize)
		mode := cipher.NewCBCEncrypter(block, iv)
		encrypted := make([]byte, len(padded))
		mode.CryptBlocks(encrypted, padded)
		return base64.StdEncoding.EncodeToString(encrypted), nil
	default:
		return "", errors.New("unsupported algorithm: " + alg)
	}
}

// decypherString
// secStr: base64 encoded string that encoded in alg cypher
// alg: 3-DES, AES
func DecryptString(secStr string, alg string, key string, pad string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(secStr)
	if err != nil {
		return "", err
	}
	switch strings.ToUpper(alg) {
	case "3-DES", "3DES", "DES3":
		// key: 24 bytes for 3DES
		k := []byte(key)
		if len(k) != 24 {
			return "", errors.New("3DES key must be 24 bytes")
		}
		block, err := des.NewTripleDESCipher(k)
		if err != nil {
			return "", err
		}
		if len(data)%block.BlockSize() != 0 {
			return "", errors.New("3DES: input not full blocks")
		}
		iv := make([]byte, block.BlockSize())
		mode := cipher.NewCBCDecrypter(block, iv)
		decrypted := make([]byte, len(data))
		mode.CryptBlocks(decrypted, data)
		if strings.ToUpper(pad) == "PKCS5" {
			decrypted, err = PKCS5Unpad(decrypted, block.BlockSize())
		} else {
			decrypted, err = PKCS7Unpad(decrypted, block.BlockSize())
		}
		if err != nil {
			return "", err
		}
		return string(decrypted), nil
	case "AES":
		k := []byte(key)
		if len(k) != 16 && len(k) != 24 && len(k) != 32 {
			return "", errors.New("AES key must be 16, 24, or 32 bytes")
		}
		block, err := aes.NewCipher(k)
		if err != nil {
			return "", err
		}
		if len(data) < block.BlockSize() || len(data)%block.BlockSize() != 0 {
			return "", errors.New("AES: input not full blocks")
		}
		iv := make([]byte, block.BlockSize())
		mode := cipher.NewCBCDecrypter(block, iv)
		decrypted := make([]byte, len(data))
		mode.CryptBlocks(decrypted, data)
		if strings.ToUpper(pad) == "PKCS5" {
			decrypted, err = PKCS5Unpad(decrypted, block.BlockSize())
		} else {
			decrypted, err = PKCS7Unpad(decrypted, block.BlockSize())
		}
		if err != nil {
			return "", err
		}
		return string(decrypted), nil
	default:
		return "", errors.New("unsupported algorithm: " + alg)
	}
}

// PKCS7Unpad removes PKCS#7 padding
func PKCS7Unpad(data []byte, blockSize int) ([]byte, error) {
	if len(data) == 0 || len(data)%blockSize != 0 {
		return nil, errors.New("invalid padding size")
	}
	padLen := int(data[len(data)-1])
	if padLen == 0 || padLen > blockSize {
		return nil, errors.New("invalid padding")
	}
	for i := 0; i < padLen; i++ {
		if data[len(data)-1-i] != byte(padLen) {
			return nil, errors.New("invalid padding")
		}
	}
	return data[:len(data)-padLen], nil
}

// PKCS7Pad adds PKCS#7 padding
func PKCS7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	pad := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(data, pad...)
}

func PKCS5Pad(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	pad := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(data, pad...)
}

func PKCS5Unpad(data []byte, blockSize int) ([]byte, error) {
	if len(data) == 0 || len(data)%blockSize != 0 {
		return nil, errors.New("invalid padding size")
	}
	padLen := int(data[len(data)-1])
	if padLen == 0 || padLen > blockSize {
		return nil, errors.New("invalid padding")
	}
	for i := 0; i < padLen; i++ {
		if data[len(data)-1-i] != byte(padLen) {
			return nil, errors.New("invalid padding")
		}
	}
	return data[:len(data)-padLen], nil
}
