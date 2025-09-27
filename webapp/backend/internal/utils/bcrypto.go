package utils

/*
#cgo CFLAGS: -std=c11 -Ofast
#cgo LDFLAGS:
#include <stdlib.h>
#include "bcrypto.h"
*/
import "C"
import (
	"crypto/subtle"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unsafe"
)

// 返すエラー（bcryptパッケージ風の名前）
var (
	ErrHashTooShort              = errors.New("bcrypt: hashed secret too short")
	ErrInvalidHashPrefix         = errors.New("bcrypt: invalid hash prefix")
	ErrInvalidCost               = errors.New("bcrypt: invalid cost")
	ErrMismatchedHashAndPassword = errors.New("bcrypt: hashedPassword is not the hash of the given password")
)

// CompareHashAndPassword は、bcryptのハッシュ文字列（$2a$/$2b$/$2y$形式, 長さ60）と生パスワードを比較する。
// 一致すればnil、そうでなければ ErrMismatchedHashAndPassword を返す。
// パースエラー等は適切なエラーを返す。
func CompareHashAndPassword(hashedPassword []byte, password []byte) error {
	version, cost, saltB64, wantHashB64, err := parseBcryptString(string(hashedPassword))
	if err != nil {
		return err
	}

	// version は現状 2a/2b/2y を許容。必要なら将来拡張。
	switch version {
	case "2a", "2b", "2y":
	default:
		return fmt.Errorf("bcrypt: unsupported version %q", version)
	}

	// C実装で再計算
	got, err := BCryptC(password, cost, saltB64)
	if err != nil {
		return fmt.Errorf("bcrypt: core compute failed: %w", err)
	}

	// 定数時間比較
	if subtle.ConstantTimeCompare(got, wantHashB64) == 1 {
		return nil
	}
	return ErrMismatchedHashAndPassword
}

// parseBcryptString は "$2b$12$<22charsalt><31charhash>" をパースする。
// 戻り値: version ("2b" 等), cost(int), saltB64([]byte), hashB64([]byte)
func parseBcryptString(s string) (version string, cost int, saltB64 []byte, hashB64 []byte, err error) {
	if len(s) < 4 || s[0] != '$' {
		err = ErrHashTooShort
		return
	}

	// 形式: $2x$cc$[22-byte-salt][31-byte-hash]
	// まず "$" で3つに分ける: "", "2x", "cc", 残り
	parts := strings.SplitN(s, "$", 4)
	// parts[0] == "", parts[1] == "2b" など, parts[2] == "12", parts[3] == 22+31 文字の連結
	if len(parts) != 4 {
		err = ErrInvalidHashPrefix
		return
	}
	if !strings.HasPrefix(parts[1], "2") || len(parts[1]) != 2 {
		err = ErrInvalidHashPrefix
		return
	}
	version = parts[1] // "2a","2b","2y"

	// cost は2桁
	if len(parts[2]) != 2 {
		err = ErrInvalidCost
		return
	}
	c, convErr := strconv.Atoi(parts[2])
	if convErr != nil || c < 4 || c > 31 {
		err = ErrInvalidCost
		return
	}
	cost = c

	rest := parts[3]
	// salt 22文字 + hash 31文字 = 53文字が通常。全体は60文字のことが多い（"$"含む）。
	if len(rest) != 53 {
		// 一部実装はパディング等で差が出る可能性があるが、ここでは厳密に合わせる。
		err = fmt.Errorf("bcrypt: invalid salt+hash length: got %d, want 53", len(rest))
		return
	}
	salt := rest[:22]
	hash := rest[22:] // 31

	if len(hash) != 31 {
		err = fmt.Errorf("bcrypt: invalid hash length: %d", len(hash))
		return
	}

	// そのまま bcrypt Base64 の文字列バイト列として返す（C側は文字列で受けてデコードする）
	saltB64 = []byte(salt)
	hashB64 = []byte(hash)
	return
}

// BCryptC は、提示の Go 実装と同じ仕様で、C実装を呼び出して
// 23バイト分のbcrypt Base64を返す。
// salt は bcrypt Base64("./A-Za-z0-9") で与える（Go側の expensiveBlowfishSetup 相当）。
func BCryptC(password []byte, cost int, saltB64 []byte) ([]byte, error) {
	if len(password) == 0 {
		return nil, errors.New("empty password")
	}
	if len(saltB64) == 0 {
		return nil, errors.New("empty salt")
	}

	outCap := 64
	out := make([]byte, outCap)
	outLen := C.size_t(outCap)

	errbuf := make([]byte, 128)

	rc := C.bcrypto(
		(*C.uchar)(unsafe.Pointer(&password[0])), C.size_t(len(password)),
		C.int(cost),
		(*C.uchar)(unsafe.Pointer(&saltB64[0])), C.size_t(len(saltB64)),
		(*C.uchar)(unsafe.Pointer(&out[0])), (*C.size_t)(unsafe.Pointer(&outLen)),
		(*C.char)(unsafe.Pointer(&errbuf[0])), C.size_t(len(errbuf)),
	)
	if rc != 0 {
		// もし outLen に必要サイズが入っていれば再試行する
		if outLen > C.size_t(outCap) && outLen < 1024 {
			out = make([]byte, int(outLen))
			rc = C.bcrypto(
				(*C.uchar)(unsafe.Pointer(&password[0])), C.size_t(len(password)),
				C.int(cost),
				(*C.uchar)(unsafe.Pointer(&saltB64[0])), C.size_t(len(saltB64)),
				(*C.uchar)(unsafe.Pointer(&out[0])), (*C.size_t)(unsafe.Pointer(&outLen)),
				(*C.char)(unsafe.Pointer(&errbuf[0])), C.size_t(len(errbuf)),
			)
		}
	}
	if rc != 0 {
		// CのerrbufはNUL終端想定
		n := 0
		for n < len(errbuf) && errbuf[n] != 0 {
			n++
		}
		return nil, errors.New(string(errbuf[:n]))
	}
	return out[:int(outLen)], nil
}
