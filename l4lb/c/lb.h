#pragma once
#include <assert.h>
#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>

#include <stdint.h>

struct bpf_crypto_params {
	char type[14];
	uint8_t reserved[2];
	char algo[128];
	uint8_t key[256];
	uint32_t key_len;
	uint32_t authsize;
};
struct callback_head {
        struct callback_head *next;
        void (*func)(struct callback_head *);
};
typedef struct {
        int counter;
} atomic_t;

struct refcount_struct {
        atomic_t refs;
};
typedef struct refcount_struct refcount_t;
struct bpf_crypto_type {
        void * (*alloc_tfm)(const char *);
        void (*free_tfm)(void *);
        int (*has_algo)(const char *);
        int (*setkey)(void *, const uint8_t *, unsigned int);
        int (*setauthsize)(void *, unsigned int);
        int (*encrypt)(void *, const uint8_t *, uint8_t *, unsigned int, uint8_t *);
        int (*decrypt)(void *, const uint8_t *, uint8_t *, unsigned int, uint8_t *);
        unsigned int (*ivsize)(void *);
        unsigned int (*statesize)(void *);
        uint32_t (*get_flags)(void *);
        void *owner;
        char name[14];
};
typedef struct bpf_crypto_ctx {
        const struct bpf_crypto_type *type;
        void *tfm;
        uint32_t siv_len;
        struct callback_head rcu;
        refcount_t usage;
} bpf_crypto_ctx_t;

// https://github.com/torvalds/linux/blob/master/tools/testing/selftests/bpf/progs/crypto_common.h#L47

bpf_crypto_ctx_t *bpf_crypto_ctx_create(const struct bpf_crypto_params *params,
					     uint32_t params__sz, int *err) __ksym;
int bpf_crypto_decrypt(bpf_crypto_ctx_t *ctx, const struct bpf_dynptr *src,
            const struct bpf_dynptr *dst, const struct bpf_dynptr *iv) __ksym;
int bpf_crypto_encrypt(bpf_crypto_ctx_t *ctx, const struct bpf_dynptr *src,
            const struct bpf_dynptr *dst, const struct bpf_dynptr *iv) __ksym;
void bpf_crypto_ctx_release(bpf_crypto_ctx_t *ctx) __ksym;

struct __crypto_ctx_value {
	struct bpf_crypto_ctx  __kptr* ctx;
};


struct array_map {
	__uint(type, BPF_MAP_TYPE_ARRAY);
	__type(key, int);
	__type(value, struct __crypto_ctx_value);
	__uint(max_entries, 1);
} __crypto_ctx_map SEC(".maps");
