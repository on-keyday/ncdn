#include "lb.h"
#include <bpf/bpf_helpers.h>
#include <errno.h>
#include <bpf/bpf_tracing.h>
#include <stdint.h>
__always_inline int crypto_ctx_insert(bpf_crypto_ctx_t * __kptr ctx)
{
	struct __crypto_ctx_value local, *v;
	bpf_crypto_ctx_t *old;
	uint32_t key = 0;
	int err;

	local.ctx = NULL;
	err = bpf_map_update_elem(&__crypto_ctx_map, &key, &local, 0);
	if (err) {
		bpf_crypto_ctx_release(ctx);
		return err;
	}

	v = bpf_map_lookup_elem(&__crypto_ctx_map, &key);
	if (!v) {
		bpf_crypto_ctx_release(ctx);
		return -ENOENT;
	}
	old = bpf_kptr_xchg(&v->ctx, ctx);
	if (old) {
		bpf_crypto_ctx_release(old);
		return -EEXIST;
	}

	return 0;
}

int status;
volatile bpf_crypto_ctx_t value;
volatile struct __crypto_ctx_value dummy;

struct quiclb_shared_key { /* go: */
	uint8_t key[16]; // 128 bits
} __attribute__((aligned(8)));

SEC("syscall")
int crypto_init(struct quiclb_shared_key *args){
	bpf_printk("crypto_init called\n");
	if(!args) {
		bpf_printk("crypto_init: args or args->key is NULL");
		return 0;
	}

	(void)value; // avoid unused variable warning
	bpf_crypto_ctx_t __kptr *cctx;
	struct bpf_crypto_params params = {
		.type = "skcipher",
        .algo = "ecb(aes)",
		.key_len = 16,
		.authsize = 0,
	};

	__builtin_memcpy(params.key, args->key, 16);

	uint64_t high,low;
	high = (uint64_t)(args->key[0]) << 56 |
		   (uint64_t)(args->key[1]) << 48 |
		   (uint64_t)(args->key[2]) << 40 |
		   (uint64_t)(args->key[3]) << 32 |
		   (uint64_t)(args->key[4]) << 24 |
		   (uint64_t)(args->key[5]) << 16 |
		   (uint64_t)(args->key[6]) << 8 |
		   (uint64_t)(args->key[7]);
	low = ((uint64_t)(args->key[8]) << 56) |
		  ((uint64_t)(args->key[9]) << 48) |
		  ((uint64_t)(args->key[10]) << 40) |
		  ((uint64_t)(args->key[11]) << 32) |
		  ((uint64_t)(args->key[12]) << 24) |
		  ((uint64_t)(args->key[13]) << 16) |
		  ((uint64_t)(args->key[14]) << 8) |
		  ((uint64_t)(args->key[15]));
	bpf_printk("DEBUG: key=%016lx%016lx", high, low);


	int err = 0;

	status = 0;


	cctx = bpf_crypto_ctx_create(&params, sizeof(params), &err);

	if (!cctx) {
		status = err;
		bpf_printk("crypto_init: bpf_crypto_ctx_create failed with %d\n", err);
		return 0;
	}

	err = crypto_ctx_insert(cctx);
	if (err && err != -EEXIST)	
		status = err;

	bpf_printk("crypto_init: err %d\n", err);

	return 0;

}
char _license[] SEC("license") = "Dual BSD/GPL";
