#include "lb.h"
#include <errno.h>
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
SEC("syscall")
int crypto_init(void *args){

	(void)value; // avoid unused variable warning
	bpf_crypto_ctx_t __kptr *cctx;
	struct bpf_crypto_params params = {
		.type = "skcipher",
        .algo = "ecb",
		.key_len = 16,
		.authsize = 0,
	};
	int err = 0;

	status = 0;


	cctx = bpf_crypto_ctx_create(&params, sizeof(params), &err);

	if (!cctx) {
		status = err;
		return 0;
	}

	err = crypto_ctx_insert(cctx);
	if (err && err != -EEXIST)
		status = err;

	return 0;

}
char _license[] SEC("license") = "Dual BSD/GPL";
