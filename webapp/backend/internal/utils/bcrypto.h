// bcrypto.h
#pragma once
#include <stddef.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C"
{
#endif

    // 実行:
    //  password: 任意バイナリ（NUL可）
    //  cost: 4..31 くらいを想定 (1<<cost ラウンド)
    //  salt_b64: bcrypt Base64("./A-Za-z0-9") でエンコードされたソルト（Go側と同等）
    // 出力:
    //  out: bcrypt Base64で23バイト分をエンコードした文字列（NUL終端無し）
    //  *out_len には書き込んだ長さが入る（23バイト→最大31文字程度）
    //  errbuf: エラー時にメッセージ（NUL終端）
    //  戻り値: 0=成功, 非0=失敗
    int bcrypto(
        const uint8_t *password, size_t pwlen,
        int cost,
        const uint8_t *salt_b64, size_t salt_b64_len,
        uint8_t *out, size_t *out_len,
        char *errbuf, size_t errbuf_len);

#ifdef __cplusplus
}
#endif

extern const uint32_t p_init[18];
extern const uint32_t s0_init[256];
extern const uint32_t s1_init[256];
extern const uint32_t s2_init[256];
extern const uint32_t s3_init[256];