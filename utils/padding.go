package utils

import (
	"bytes"
)

func _padding(cipherText []byte, blockSize int, padInt int) []byte {
	length := (len(cipherText) + blockSize) / blockSize
	padLen := blockSize*length - len(cipherText)
	if padInt == -1 {
		padInt = padLen
	}
	padText := bytes.Repeat([]byte{byte(padInt)}, padLen)
	return append(cipherText, padText...)
}

// _unPadding 去除填充。
//
// 注意：原实现直接信任密文最后一个字节作为填充长度，
// data[:length-unPaddingLen] 在填充值大于数据长度时会越界 panic
// （密钥不匹配、密文被截断、对端返回错误数据时都可能发生）；
// data[length-blockSize:] 在数据短于一个 block 时同样会 panic。
// 这里对非法填充直接原样返回，交由调用方按业务判断。
func _unPadding(data []byte, blockSize int) []byte {
	length := len(data)
	if length == 0 || blockSize <= 0 || length < blockSize {
		return data
	}
	unPaddingLen := int(data[length-1])
	// 零填充的情况
	if unPaddingLen == 0 {
		padding := data[length-blockSize:]
		for i := len(padding) - 1; i >= 0; i-- {
			if padding[i] != 0 {
				break
			}
			unPaddingLen++
		}
	}
	if unPaddingLen < 0 || unPaddingLen > length {
		// 非法填充长度：保持原样，避免切片越界 panic
		return data
	}
	return data[:length-unPaddingLen]
}

func ZeroPadding(cipherText []byte, blockSize int) []byte {
	return _padding(cipherText, blockSize, 0)
}

func PKCS7Padding(cipherText []byte) []byte {
	return _padding(cipherText, 16, -1)
}

func PKCS5Padding(cipherText []byte) []byte {
	return _padding(cipherText, 8, -1)
}

func UnPadding(data []byte) []byte {
	return _unPadding(data, 16)
}
