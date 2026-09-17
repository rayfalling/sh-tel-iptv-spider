package utils

import (
	"crypto/aes"
	"crypto/md5"
)

type AESForNodejs struct {
	key [16]byte
}

func NewAESForNodejs(key []byte) *AESForNodejs {
	c := AESForNodejs{}
	c.setKey(key)
	return &c
}

func (a *AESForNodejs) setKey(key []byte) {
	a.key = md5.Sum(key)
}

// Decrypt AES-ECB 解密。
//
// 原实现按 len(data) 做循环上界，密文长度不是 block 整数倍时会 data[bs:be] 越界 panic。
// 这里对非法长度直接返回 nil，再由 UnPadding 做填充校验（同样不再越界）。
func (a *AESForNodejs) Decrypt(data []byte) []byte {
	block, err := aes.NewCipher(a.key[:])
	if err != nil {
		return nil
	}
	blockSize := block.BlockSize()
	if len(data) == 0 || len(data)%blockSize != 0 {
		return nil
	}

	decrypted := make([]byte, len(data))
	for off := 0; off+blockSize <= len(data); off += blockSize {
		block.Decrypt(decrypted[off:off+blockSize], data[off:off+blockSize])
	}

	return UnPadding(decrypted)
}

// Encrypt AES-ECB 加密（PKCS7 填充到 AES block 大小）。
//
// 原实现有两个缺陷：
//  1. 填充用的是 PKCS5Padding（补到 8 字节边界），而 AES block 是 16 字节；
//  2. 循环上界写的是**未填充**长度 len(data)，却去索引已填充的 plain，于是
//     plain[bs:be] 在约一半的输入长度上越界 panic（例如 100、215 字节会 panic，
//     当前线上参数凑巧是 223 字节才没触发）。
//     authenticator 的 JSON 长度随 UID/SN/MAC/Token 长度变化，换个机顶盒参数就会踩到。
//
// 现在统一按 AES block 大小填充，并以填充后长度作为加密上界。
// 对于原来能正常工作的输入（满足 padded8 == padded16），输出与旧实现逐字节一致；
// 原来会 panic 的输入则变成一段长度合法的密文（认证方会拒绝，但不再拖垮进程）。
func (a *AESForNodejs) Encrypt(data []byte) []byte {
	block, err := aes.NewCipher(a.key[:])
	if err != nil {
		return nil
	}
	blockSize := block.BlockSize()
	// PKCS7Padding 就是补到 16 字节边界（补满一整块），正是 ECB 需要的填充
	plain := PKCS7Padding(data)

	encrypted := make([]byte, len(plain))
	for off := 0; off+blockSize <= len(plain); off += blockSize {
		block.Encrypt(encrypted[off:off+blockSize], plain[off:off+blockSize])
	}

	return encrypted
}
