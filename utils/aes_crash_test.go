package utils

import (
	"bytes"
	"testing"
)

// TestAESForNodejs_EncryptAllLengths 回归测试：
// 原 Encrypt 用未填充长度做循环上界，却索引已填充缓冲区，
// 在约一半的输入长度上 plain[bs:be] 越界 panic（例如 100 / 215 字节）。
// authenticator 的 JSON 长度随 UID/SN/MAC/Token 变化，换个参数就会踩到。
func TestAESForNodejs_EncryptAllLengths(t *testing.T) {
	aes := NewAESForNodejs([]byte("123456"))
	for l := 0; l <= 3000; l++ {
		data := bytes.Repeat([]byte{'x'}, l)

		var got []byte
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("Encrypt(len=%d) panicked: %v", l, r)
				}
			}()
			got = aes.Encrypt(data)
		}()

		// 输出必须是 AES block（16 字节）的整数倍，且补满一整块
		wantLen := (l/16 + 1) * 16
		if len(got) != wantLen {
			t.Fatalf("Encrypt(len=%d) length = %d, want %d", l, len(got), wantLen)
		}
	}
}

// TestAESForNodejs_ZeroLength 空输入按 PKCS7 补满一整块 0x10
func TestAESForNodejs_ZeroLength(t *testing.T) {
	aes := NewAESForNodejs([]byte("123456"))
	if got := len(aes.Encrypt(nil)); got != 16 {
		t.Fatalf("Encrypt(nil) length = %d, want 16", got)
	}
}

// TestAESForNodejs_RoundTrip 各长度加解密往返一致
func TestAESForNodejs_RoundTrip(t *testing.T) {
	aes := NewAESForNodejs([]byte("123456"))
	for l := 0; l <= 512; l++ {
		data := bytes.Repeat([]byte{byte('a' + l%26)}, l)
		enc := aes.Encrypt(data)
		dec := aes.Decrypt(enc)
		if !bytes.Equal(dec, data) {
			t.Fatalf("round trip mismatch at len=%d: got %q want %q", l, dec, data)
		}
	}
}

// TestAESForNodejs_DecryptBadLength 非 block 整数倍的密文不再越界 panic
func TestAESForNodejs_DecryptBadLength(t *testing.T) {
	aes := NewAESForNodejs([]byte("123456"))
	for _, n := range []int{0, 1, 7, 15, 17, 31, 33} {
		// 只要不 panic 即可，返回 nil 或原样数据都算可接受
		_ = aes.Decrypt(bytes.Repeat([]byte{0xAB}, n))
	}
}

// TestUnPadding_Invalid 非法填充长度不再导致切片越界
func TestUnPadding_Invalid(t *testing.T) {
	// 短于一个 block
	short := bytes.Repeat([]byte{0x00}, 15)
	// 填充值 255 > 长度 16
	badPad := append(bytes.Repeat([]byte{0x00}, 15), 0xFF)
	cases := [][]byte{nil, {}, {0x01}, short, badPad}
	for i, c := range cases {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("UnPadding(case %d) panicked: %v", i, r)
				}
			}()
			_ = UnPadding(c)
		}()
	}
}
