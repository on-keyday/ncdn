#include <assert.h>
#include <stdint.h>
#include <linux/bpf.h>
#include <linux/if_ether.h>
#include <netinet/in.h>
#include <netinet/ip.h>
#include <netinet/tcp.h>
#include <netinet/udp.h>
#include <stdbool.h>


#include <bpf/bpf_helpers.h>
#include "lb.h"

#include <stdint.h>
#include <sys/types.h>
#include "errno.h"

#define PACKED __attribute__((__packed__))
#define ALIGN8 __attribute__((aligned(8)))

#define ENABLE_XDPCAP
#include "xdpcap.h"

// We use clang built-in memcpy, but need a function signature to provoke it.
void* memcpy(void*, const void*, unsigned long);

#define DEBUG_LB_MAIN 1

// clang-format off
struct stat_counters { /* go:Add,String */
  uint64_t rx_packet_total; // HELP Number of packets received against known VIPs.
  uint64_t rx_total_size; // HELP Total size of packets received against known VIPs.

  uint64_t too_short_packet_total; // HELP Number of packets dropped due to being too short.
  uint64_t non_ipv4_packet_total; // HELP Number of packets dropped due to their IP protocol version not v4.
  uint64_t ip_option_packet_total; // HELP Number of packets dropped due to their IP header having options. (currently not supported)
  uint64_t non_supported_proto_packet_total; // HELP Number of packets dropped due to their protocol not being TCP.
  uint64_t no_vip_match_total; // HELP Number of packets dropped due to their dest IP address not matching any known VIP.
  uint64_t failed_adjust_head_total; // HELP Number of xdp_adjust_head failures.
  uint64_t failed_adjust_tail_total; // HELP Number of xdp_adjust_tail failures.

  uint64_t quiclb_short_packet_total; // HELP Number of QUIC-LB short packets received.
  uint64_t quiclb_long_packet_total; // HELP Number of QUIC-LB long packets received.
  uint64_t quiclb_initial_routing_packet_total; // HELP Number of QUIC-LB initial routing packets received.
  uint64_t quiclb_too_short_long_packet_total; // HELP Number of QUIC-LB short-long packets received.
  uint64_t quiclb_no_connection_id_total; // HELP Number of QUIC-LB packets received without connection ID.
  uint64_t quiclb_no_dest_entry_total; // HELP Number of QUIC-LB packets received without destination entry.
  uint64_t quiclb_invalid_crypto_context_total; // HELP Number of QUIC-LB packets received with invalid crypto context.
  uint64_t quiclb_encrypt_success_call_total; // HELP Number of QUIC-LB packets that called bpf_crypto_encrypt.
  uint64_t quiclb_decrypt_success_call_total; // HELP Number of QUIC-LB packets that called bpf_crypto_decrypt.
} ALIGN8;
// clang-format on

struct {
  __uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
  __uint(max_entries, 1);
  __type(key, uint32_t);
  __type(value, struct stat_counters);
} stat_counters_map SEC(".maps");

// lb_config contains the global configuration of the load balancer, which is
// universal to all flows that it handles.
struct lb_config { /* go: */
  uint32_t vip_address;
  uint32_t num_dests;
} PACKED;

struct {
  __uint(type, BPF_MAP_TYPE_ARRAY);
  __uint(max_entries, 1);
  __type(key, uint32_t);
  __type(value, struct lb_config);
} lb_config_map SEC(".maps");

// destination_entry carries a info that is needed to construct an encap packet
// to the destination.
struct destination_entry {
  uint32_t ip_address;
  uint8_t mac_address[ETH_ALEN];
} PACKED;

#define DESTINATIONS_SIZE 255 /* go: */

// destinations_map is a map that contains the destination_entry for each
// destination that the load balancer can send packets to.
// `destinations_map[0]` is a special entry that contains the info for filling
// the source header fields.
struct {
  __uint(type, BPF_MAP_TYPE_ARRAY);
  __uint(max_entries, (DESTINATIONS_SIZE + 1));
  __type(key, uint32_t);
  __type(value, struct destination_entry);
} destinations_map SEC(".maps");

#if DEBUG_LB_MAIN
#define debugk(fmt, ...) bpf_printk(fmt, ##__VA_ARGS__)
#else
#define debugk(fmt, ...) \
  do {                   \
  } while (0)
#endif

struct quic_long_packet {
   uint8_t first_byte;
   uint8_t version[4];
   uint8_t dst_conn_id_len;
} PACKED;

struct quic_short_packet {
   uint8_t first_byte;
} PACKED;

#define QUICLB_CONNECTION_ID_SIZE 20
#define QUICLB_CONNECTION_ID_CONFIG_ROTATION(x) (((x) & 0xe0) >> 5)
#define QUICLB_CONNECTION_ID_RANDOM_VALUE(x) ((x) & 0x1f)
#define QUICLB_CONNECTION_ID_SERVER_ID(x) (((x)[0] >> 4) & 0x0f)
struct quiclb_connection_id {
  uint8_t first_byte; // 3 bit: config_rotation, 5 bit: random value
  uint8_t connection_id[3]; // first 4 bits is server id and remaining 20 bits is connection id
  uint8_t nonce_message_authentication_code[16]; // nonce padding 7 bit + 121 bit MAC
} PACKED; // 20 bytes total

/*
//clang-format off
uint8_t inv_sbox[256] = {
0x52,0x09,0x6a,0xd5,0x30,0x36,0xa5,0x38,0xbf,0x40,0xa3,0x9e,0x81,0xf3,0xd7,0xfb,
0x7c,0xe3,0x39,0x82,0x9b,0x2f,0xff,0x87,0x34,0x8e,0x43,0x44,0xc4,0xde,0xe9,0xcb,
0x54,0x7b,0x94,0x32,0xa6,0xc2,0x23,0x3d,0xee,0x4c,0x95,0x0b,0x42,0xfa,0xc3,0x4e,
0x08,0x2e,0xa1,0x66,0x28,0xd9,0x24,0xb2,0x76,0x5b,0xa2,0x49,0x6d,0x8b,0xd1,0x25,
0x72,0xf8,0xf6,0x64,0x86,0x68,0x98,0x16,0xd4,0xa4,0x5c,0xcc,0x5d,0x65,0xb6,0x92,
0x6c,0x70,0x48,0x50,0xfd,0xed,0xb9,0xda,0x5e,0x15,0x46,0x57,0xa7,0x8d,0x9d,0x84,
0x90,0xd8,0xab,0x00,0x8c,0xbc,0xd3,0x0a,0xf7,0xe4,0x58,0x05,0xb8,0xb3,0x45,0x06,
0xd0,0x2c,0x1e,0x8f,0xca,0x3f,0x0f,0x02,0xc1,0xaf,0xbd,0x03,0x01,0x13,0x8a,0x6b,
0x3a,0x91,0x11,0x41,0x4f,0x67,0xdc,0xea,0x97,0xf2,0xcf,0xce,0xf0,0xb4,0xe6,0x73,
0x96,0xac,0x74,0x22,0xe7,0xad,0x35,0x85,0xe2,0xf9,0x37,0xe8,0x1c,0x75,0xdf,0x6e,
0x47,0xf1,0x1a,0x71,0x1d,0x29,0xc5,0x89,0x6f,0xb7,0x62,0x0e,0xaa,0x18,0xbe,0x1b,
0xfc,0x56,0x3e,0x4b,0xc6,0xd2,0x79,0x20,0x9a,0xdb,0xc0,0xfe,0x78,0xcd,0x5a,0xf4,
0x1f,0xdd,0xa8,0x33,0x88,0x07,0xc7,0x31,0xb1,0x12,0x10,0x59,0x27,0x80,0xec,0x5f,
0x60,0x51,0x7f,0xa9,0x19,0xb5,0x4a,0x0d,0x2d,0xe5,0x7a,0x9f,0x93,0xc9,0x9c,0xef,
0xa0,0xe0,0x3b,0x4d,0xae,0x2a,0xf5,0xb0,0xc8,0xeb,0xbb,0x3c,0x83,0x53,0x99,0x61,
0x17,0x2b,0x04,0x7e,0xba,0x77,0xd6,0x26,0xe1,0x69,0x14,0x63,0x55,0x21,0x0c,0x7d,
};
//clang-format on

__always_inline void inv_subbytes(uint8_t* s0) {
    s0[0] = inv_sbox[s0[0]];
    s0[1] = inv_sbox[s0[1]];
    s0[2] = inv_sbox[s0[2]];
    s0[3] = inv_sbox[s0[3]];
    s0[4] = inv_sbox[s0[4]];
    s0[5] = inv_sbox[s0[5]];  
    s0[6] = inv_sbox[s0[6]];
    s0[7] = inv_sbox[s0[7]];
    s0[8] = inv_sbox[s0[8]];
    s0[9] = inv_sbox[s0[9]];
    s0[10] = inv_sbox[s0[10]];
    s0[11] = inv_sbox[s0[11]];
    s0[12] = inv_sbox[s0[12]];
    s0[13] = inv_sbox[s0[13]];
    s0[14] = inv_sbox[s0[14]];
    s0[15] = inv_sbox[s0[15]];
}

__always_inline void inv_shiftrows(uint8_t* s0) {
    uint8_t s[4];
    for (int r = 0; r < 4; r++) {
        s[0] = s0[0*4 + r];
        s[1] = s0[1*4 + r];
        s[2] = s0[2*4 + r];
        s[3] = s0[3*4 + r];
        s0[0*4 + r] = s[(4-r) % 4];
        s0[1*4 + r] = s[(5-r) % 4];
        s0[2*4 + r] = s[(6-r) % 4];
        s0[3*4 + r] = s[(7-r) % 4];
    }
}

__always_inline uint8_t xtime(uint8_t x) {
    // GF(2^8)上でのx * {02}の乗算
    return (x << 1) ^ (((x >> 7) & 1) * 0x1b);
}

// ミックスカラムは行列積で実装
__always_inline void inv_mix_columns(uint8_t* state) {
    for (int c = 0; c < 4; c++) {
        uint8_t s0 = state[4*c + 0];
        uint8_t s1 = state[4*c + 1];
        uint8_t s2 = state[4*c + 2];
        uint8_t s3 = state[4*c + 3];

        state[4*c + 0] = xtime(xtime(xtime(s0))) ^ xtime(xtime(s1)) ^ xtime(xtime(xtime(s1))) ^ s1 ^ xtime(xtime(s2)) ^ xtime(xtime(xtime(s2))) ^ s2 ^ xtime(xtime(xtime(s3))) ^ xtime(s3);
        state[4*c + 1] = xtime(xtime(xtime(s0))) ^ xtime(s0) ^ xtime(xtime(xtime(s1))) ^ xtime(xtime(s1)) ^ s1 ^ xtime(xtime(xtime(s2))) ^ xtime(s2) ^ xtime(xtime(xtime(s3))) ^ xtime(xtime(s3)) ^ s3;
        state[4*c + 2] = xtime(xtime(s0)) ^ s0 ^ xtime(xtime(xtime(s1))) ^ xtime(s1) ^ xtime(xtime(s2)) ^ xtime(xtime(xtime(s2))) ^ s2 ^ xtime(xtime(s3)) ^ xtime(xtime(xtime(s3))) ^ s3;
        state[4*c + 3] = xtime(s0) ^ xtime(xtime(s0)) ^ s0 ^ xtime(s1) ^ xtime(xtime(s1)) ^ s1 ^ xtime(s2) ^ xtime(xtime(s2)) ^ s2 ^ xtime(s3) ^ xtime(xtime(s3)) ^ s3;
    }
}

__always_inline void add_round_key(uint8_t* state, uint8_t* round_key) {
    state[0] ^= round_key[0];
    state[1] ^= round_key[1];
    state[2] ^= round_key[2];
    state[3] ^= round_key[3];
    state[4] ^= round_key[4];
    state[5] ^= round_key[5];
    state[6] ^= round_key[6];
    state[7] ^= round_key[7];
    state[8] ^= round_key[8];
    state[9] ^= round_key[9];
    state[10] ^= round_key[10];
    state[11] ^= round_key[11];
    state[12] ^= round_key[12];
    state[13] ^= round_key[13];
    state[14] ^= round_key[14];
    state[15] ^= round_key[15];
}

__always_inline void aes128_decrypt(uint8_t state[16], uint8_t* round_keys) {
    // 最後のラウンドキーを適用
    add_round_key(state, &round_keys[10 * 16]);

    // 逆のラウンドを実行
    for (int r = 9; r > 0; r--) {
        inv_shiftrows(state);
        inv_subbytes(state);
        add_round_key(state, &round_keys[r * 16]);
        inv_mix_columns(state);
    }

    // 最初のラウンドキーを適用
    inv_shiftrows(state);
    inv_subbytes(state);
    add_round_key(state, round_keys);
}


*/



/*
__always_inline void expand_result(uint8_t(*out)[16],uint8_t input_10[10],uint8_t pass){
   (*out)[0] = input_10[0];
   (*out)[1] = input_10[1];
   (*out)[2] = input_10[2];
   (*out)[3] = input_10[3];
   (*out)[4] = input_10[4];
   (*out)[5] = input_10[5];
   (*out)[6] = input_10[6];
   (*out)[7] = input_10[7];
   (*out)[8] = input_10[8];
   (*out)[9] = input_10[9];
   (*out)[10] = 0;
   (*out)[11] = 0;
   (*out)[12] = 0;
   (*out)[13] = 0;
   (*out)[14] = 19; // length
   (*out)[15] = pass;
}
*/

/*
__always_inline void split_19(uint8_t(*left)[10],uint8_t(*right)[10],const uint8_t* input19) {
    (*left)[0] = input19[0];
    (*left)[1] = input19[1];
    (*left)[2] = input19[2];
    (*left)[3] = input19[3];
    (*left)[4] = input19[4];
    (*left)[5] = input19[5];
    (*left)[6] = input19[6];
    (*left)[7] = input19[7];
    (*left)[8] = input19[8];
    (*left)[9] = input19[9] & 0xf0;
  
    (*right)[0] = input19[9] & 0x0f;
    (*right)[1] = input19[10];
    (*right)[2] = input19[11];
    (*right)[3] = input19[12];
    (*right)[4] = input19[13];
    (*right)[5] = input19[14];
    (*right)[6] = input19[15];
    (*right)[7] = input19[16];
    (*right)[8] = input19[17];
    (*right)[9] = input19[18];
}
*/

// 普通にメモリコピーのループを使うと最適化により
// error: lb.c:294:15: in function test_decrypt i32 (ptr): A call to built-in function 'memset' is not supported.
// のようなエラーになるためvolatileを使う
#define VOLATILE(x) ((volatile uint8_t*)(x))
#define HALF_LEN(x) ((((x) / 2) + ((x) % 2)))
#define CHECK_VOLATILE(x,cmp,ret) if(*VOLATILE(&x) > cmp) { \
    debugk("ASSERTION FAILURE: %s > %d", #x, cmp); \
    return ret; \
}
#define DEFINE_HALF_LEN(ret) uint8_t half_len_ = HALF_LEN(full_len);/*CHECK_VOLATILE(half_len_,10,ret)*/\
const uint8_t half_len = half_len_;

__always_inline void expand(uint8_t out[16],uint8_t* input, uint8_t full_len,uint8_t pass) {
  DEFINE_HALF_LEN();
  for(int i = 0; i < half_len; i++) {
     VOLATILE(out)[i] = input[i];
  }
  for(int i = half_len; i < 16; i++) {
     VOLATILE(out)[i] = 0;
  }
  out[14] = full_len; // length
  out[15] = pass;
}

__always_inline int split(uint8_t* left,uint8_t* right,const uint8_t* input,uint8_t full_len) {
    if(full_len < 5 || full_len > 19) {
      return -EINVAL;
    }
    DEFINE_HALF_LEN(-EINVAL);
    const uint8_t right_start = full_len - half_len;
    //CHECK_VOLATILE(right_start, 10, -EINVAL);
    for (uint8_t i = 0; i < half_len; i++) {
        VOLATILE(left)[i] = input[i];
    }
    for (uint8_t i = 0; i < half_len; i++) {
        // uint8_t offset = ();
        //CHECK_VOLATILE(offset, 18, -EINVAL);
        VOLATILE(right)[i] = input[i + right_start];
    }
    if(full_len & 1) {
        left[half_len - 1] &= 0xf0;
        right[0] &= 0x0f;
    }
    return 0;
}

__always_inline void concat(uint8_t* output, const uint8_t* left, const uint8_t* right,uint8_t full_len) {
    DEFINE_HALF_LEN();
    const int right_start = full_len - half_len;
    for (int i = 0; i < half_len; i++) {
       VOLATILE(output)[i] = left[i];
    }
    for (int i = 0; i < half_len; i++) {
       VOLATILE(output)[i + right_start] = right[i];
    }
    if(full_len & 1) {
        output[half_len - 1] = (left[half_len - 1] & 0xf0) | (right[0] & 0x0f);
    }
}

__always_inline void xor_assign(uint8_t* out, uint8_t* in,uint8_t len) {
    for (int i = 0; i < len; i++) {
        out[i] ^= in[i];
    }
}

// from https://github.com/torvalds/linux/blob/master/kernel/bpf/crypto.c#L30
// GPL-2.0

__always_inline struct __crypto_ctx_value *crypto_ctx_value_lookup(void)
{
    uint32_t key = 0;

    return bpf_map_lookup_elem(&__crypto_ctx_map, &key);
}


__always_inline void print_half(const char* name, uint8_t* data) {
    uint64_t high;
    uint16_t low;
    high = (uint64_t)(data[0]) << 56 |
           (uint64_t)(data[1]) << 48 |
           (uint64_t)(data[2]) << 40 |
           (uint64_t)(data[3]) << 32 |
           (uint64_t)(data[4]) << 24 |
           (uint64_t)(data[5]) << 16 |
           (uint64_t)(data[6]) << 8 |
           (uint64_t)(data[7]);
    low = (data[8] << 8) | data[9];
    debugk("DEBUG: %s=%016lx%04x", name, high, low);
}

__always_inline void print_key(const char* name, const uint8_t* key) {
    uint64_t high, low;
    high = (uint64_t)(key[0]) << 56 |
           (uint64_t)(key[1]) << 48 |
           (uint64_t)(key[2]) << 40 |
           (uint64_t)(key[3]) << 32 |
           (uint64_t)(key[4]) << 24 |
           (uint64_t)(key[5]) << 16 |
           (uint64_t)(key[6]) << 8 |
           (uint64_t)(key[7]);
    low = ((uint64_t)(key[8]) << 56) |
          ((uint64_t)(key[9]) << 48) |
          ((uint64_t)(key[10]) << 40) |
          ((uint64_t)(key[11]) << 32) |
          ((uint64_t)(key[12]) << 24) |
          ((uint64_t)(key[13]) << 16) |
          ((uint64_t)(key[14]) << 8) |
          ((uint64_t)(key[15]));
    debugk("DEBUG: %s=%016lx%016lx", name, high, low);
}

__always_inline void print_full_connection_id(const char* name, const uint8_t* connection_id) {
    uint64_t high, middle;
    uint32_t low;
    high = (uint64_t)(connection_id[0]) << 56 |
           (uint64_t)(connection_id[1]) << 48 |
           (uint64_t)(connection_id[2]) << 40 |
           (uint64_t)(connection_id[3]) << 32 |
           (uint64_t)(connection_id[4]) << 24 |
           (uint64_t)(connection_id[5]) << 16 |
           (uint64_t)(connection_id[6]) << 8 |
           (uint64_t)(connection_id[7]);
    middle = (uint64_t)(connection_id[8]) << 56 |
             (uint64_t)(connection_id[9]) << 48 |
             (uint64_t)(connection_id[10]) << 40 |
             (uint64_t)(connection_id[11]) << 32 |
             (uint64_t)(connection_id[12]) << 24 |
             (uint64_t)(connection_id[13]) << 16 |
             (uint64_t)(connection_id[14]) << 8 |
             (uint64_t)(connection_id[15]);
    low = ((uint32_t)(connection_id[16]) << 24) |
          ((uint32_t)(connection_id[17]) << 16) |
          ((uint32_t)(connection_id[18]) << 8) |
          ((uint32_t)(connection_id[19]));
    debugk("DEBUG: %s=%016lx%016lx%08x",name, high, middle, low);
}

uint8_t temporary[16];

// output length should be same as input length
__always_inline int connection_id_decrypt(uint8_t* output, const uint8_t* connection_id_bytes,uint8_t input_len,bool short_server_id, const char* context,struct stat_counters* c) {
    if(input_len == 0 || input_len > 20) {
        debugk("%s: input_len %d > 20, invalid for current implementation", context, input_len);
        return -EINVAL;
    }
    debugk("%s: input_len=%d, short_server_id=%d", context, input_len, short_server_id);
    struct __crypto_ctx_value *v = crypto_ctx_value_lookup();
    if (!v) {
        debugk("%s: no crypto context found", context);
        return -ENOENT;
    }
    struct bpf_crypto_ctx *ctx = (struct bpf_crypto_ctx*) v->ctx;
    if(!ctx) {
        debugk("%s: crypto context is NULL", context);
        return -ENOENT;
    }
    debugk("%s: crypto context found, ctx=%p", context, ctx);
    print_full_connection_id("encrypted_conn_id", connection_id_bytes);
    struct bpf_dynptr temporary_dynptr;
    int ret = bpf_dynptr_from_mem(&temporary,sizeof(temporary), 0, &temporary_dynptr);
    if (ret < 0) {
        debugk("%s: bpf_dynptr_from_mem failed with %d", context, ret);
        return ret;
    }
    int aes_result = 0;
  #define CHECK_AES_RESULT()    \
  if(aes_result < 0) { \
    debugk("%s: bpf_crypto_encrypt failed with %d", context, aes_result);  \
    return aes_result; \
  }
#define DO_AES_ECB()    \
 aes_result =  bpf_crypto_encrypt(ctx, &temporary_dynptr, &temporary_dynptr, NULL);\
  CHECK_AES_RESULT();\
 c->quiclb_encrypt_success_call_total++;

    const int full_len = input_len - 1;
    // see 4.4.1.  Special Case: Single Pass Encryption
    if(full_len == 16) { // special case for 16 byte connection ID see 
      debugk("%s: full_len is 16, using direct encryption", context);
      __builtin_memcpy(temporary, &connection_id_bytes[1], 16);
      aes_result = bpf_crypto_decrypt(ctx, &temporary_dynptr, &temporary_dynptr, NULL);
      CHECK_AES_RESULT();
      c->quiclb_decrypt_success_call_total++;
      output[0] = connection_id_bytes[0]; // first byte is not encrypted
      __builtin_memcpy(&output[1], temporary, 16);
      print_full_connection_id("decrypted_conn_id", output);
      return 0;
    }

    // see 4.4.2.  General Case: Four-Pass Encryption
    const int input_is_odd = full_len & 1;
    DEFINE_HALF_LEN(-EINVAL);
    debugk("%s: full_len=%d, half_len=%d, input_is_odd=%d", context, full_len, half_len, input_is_odd);
    uint8_t left[10],right[10];
    int err =  split(left, right, &connection_id_bytes[1],full_len);
    if(err < 0) {
        debugk("%s: split failed with %d", context, err);
        return err;
    }
    print_half("left_2", left);
    print_half("right_2", right);
#define ROUND(n,input,output) \
    expand(temporary, input, full_len , n);\
    print_key("pass_" #n "_key", temporary); \
    DO_AES_ECB(); \
    xor_assign(output, temporary, half_len);\
    if(input_is_odd) { output[n%2 == 0? half_len - 1: 0] &= n%2 == 0? 0xf0 : 0x0f; }

    // right_2 -> left_1
    ROUND(4,right,left);
    print_half("left_1", left);
    // left_1 -> right_1
    ROUND(3,left,right);
    print_half("right_1", right);
    // right_1 -> left_0
    ROUND(2,right,left);
    print_half("left_0", left);
    // the internet draft says:
    // As the load balancer has no need for the nonce, it can conclude after
    // 3 passes as long as the server ID is entirely contained in left_0
    // (i.e., the nonce is at least as large as the server ID).  If the
    // server ID is longer, a fourth pass is necessary:
    if(!short_server_id) {
       // left_0 -> right_0
       ROUND(1,left,right);
       print_half("right_0", right);
    }

    output[0] = connection_id_bytes[0]; // first byte is not encrypted
    concat(&output[1], left, right, full_len);
    print_full_connection_id("decrypted_conn_id", output);
    return 0;
}


__always_inline struct destination_entry* handle_initial(struct quiclb_connection_id* conn_id,
struct lb_config* config,
struct stat_counters* c,
                                               const char* context) {
     // fast path to roundrobin based on connection ID TODO: improve for security
    const char* conn_id_bytes = (const char*)(conn_id);
    uint32_t key = conn_id_bytes[0];
    uint32_t dest_idx = (key % config->num_dests) + 1;
    debugk("%s (initial) dest_idx=%d",context, dest_idx);
    struct destination_entry* dest = bpf_map_lookup_elem(&destinations_map, &dest_idx);
    if (!dest) {
      debugk("ASSERTION FAILURE: no dest entry for %d", dest_idx);
      ++c->quiclb_no_dest_entry_total;
      return NULL;
    }
    ++c->quiclb_initial_routing_packet_total;
    return dest;
}

// https://github.com/torvalds/linux/blob/01a412d06bc5786eb4e44a6c8f0f4659bd4c9864/kernel/bpf/crypto.c#L146
__always_inline struct destination_entry* handle_connection_id(struct quiclb_connection_id* conn_id,
                                               struct lb_config* config,
                                               struct stat_counters* c,
                                               const char* context
                                              ) {
    uint8_t output[QUICLB_CONNECTION_ID_SIZE];
    int err = connection_id_decrypt(output, (const uint8_t*)conn_id,20,false, context,c);
    if(err < 0) {
      ++c->quiclb_invalid_crypto_context_total;
      debugk("%s: connection_id_decrypt failed with %d", context, err);
      return NULL;
    }
    const int dest_idx_ = QUICLB_CONNECTION_ID_SERVER_ID(((struct quiclb_connection_id*)(output))->connection_id);
    const int dest_idx = dest_idx_ + 1; // dest_idx is 1-based index
    debugk("%s conn_id dest_idx=%d", context, dest_idx);
    if(dest_idx > config->num_dests) {
       debugk("idx %d >= num_dests %d", dest_idx, config->num_dests);
       return handle_initial(conn_id, config, c, context);
    }

    struct destination_entry* entry = bpf_map_lookup_elem(&destinations_map, &dest_idx);
    if(!entry) {
      debugk("no destination entry for %d", dest_idx);
      return NULL;
    }
    debugk("found dest entry for %d", dest_idx);
    return entry;
}

struct test_decrypt { /* go: */
  uint8_t connection_id[20];
  uint8_t len;
  uint8_t is_short_server_id; // 1 if short server id, 0 if long server id
} PACKED;

SEC("xdp")
int test_decrypt(struct xdp_md* ctx) {
  void* data_raw = (void*)(uint64_t)ctx->data;
  void* data_end = (void*)(uint64_t)ctx->data_end;
  if (data_raw + sizeof(struct test_decrypt) >
      data_end) {
    return -EINVAL;
  }
  struct test_decrypt* data = (struct test_decrypt*)data_raw;
  struct stat_counters c;
  uint8_t input_output[QUICLB_CONNECTION_ID_SIZE + 1];
  for(int i = 0; i < QUICLB_CONNECTION_ID_SIZE; i++) {
    input_output[i] = data->connection_id[i];
  }
  int res;
#define DO_DECRYPT(n) case n: res = connection_id_decrypt(input_output, input_output,  n ,data->is_short_server_id,"test", &c);break;
  switch(data->len) {
    DO_DECRYPT(1)
    DO_DECRYPT(2)
    DO_DECRYPT(3)
    DO_DECRYPT(4)
    DO_DECRYPT(5)
    DO_DECRYPT(6)
    DO_DECRYPT(7)
    DO_DECRYPT(8)
    DO_DECRYPT(9)
    DO_DECRYPT(10)
    DO_DECRYPT(11)
    DO_DECRYPT(12)
    DO_DECRYPT(13)
    DO_DECRYPT(14)
    DO_DECRYPT(15)
    DO_DECRYPT(16)
    DO_DECRYPT(17)
    DO_DECRYPT(18)
    DO_DECRYPT(19)
    DO_DECRYPT(20)
    default:
      debugk("ASSERTION FAILURE: len %d is not in range [1,20]", data->len);
      return -EINVAL;
  }
  for(int i = 0; i < QUICLB_CONNECTION_ID_SIZE; i++) {
    data->connection_id[i] = input_output[i];
  }
  return res;
}

SEC("xdp")
int lb_main(struct xdp_md* ctx) {
  
  void* data = (void*)(uint64_t)ctx->data;
  void* data_end = (void*)(uint64_t)ctx->data_end;

  // Get pointer to the `stat_counters`. The stats are stored per CPU,
  // and the driver code will sum them up upon read.
  const uint32_t map_key_zero = 0;
  struct stat_counters* c =
      bpf_map_lookup_elem(&stat_counters_map, &map_key_zero);
  if (!c) {
    EXIT(XDP_PASS);
  }

  // Get pointer to the `config`. Since the XDP prog can only access BPF maps,
  // we use an BPF map (actually a `BPF_MAP_TYPE_ARRAY`) with a single entry.
  struct lb_config* config = bpf_map_lookup_elem(&lb_config_map, &map_key_zero);
  if (!config) {
    EXIT(XDP_PASS);
  }

  // Lookup ip address and mac address to be used for the source header fields.
  struct destination_entry* src_entry =
      bpf_map_lookup_elem(&destinations_map, &map_key_zero);
  if (!src_entry) {
    EXIT(XDP_PASS);
  }

  // Check if the packet is long enough to contain the headers we need.
  if (data + sizeof(struct ethhdr) + sizeof(struct iphdr) >
      data_end) {
    ++c->too_short_packet_total;
    EXIT(XDP_PASS);
  }

  struct ethhdr* eth = data;
  struct iphdr* ip = (struct iphdr*)(eth + 1);

  // Check if the packet is IPv4, has no IP options, is destined to the VIP,
  // and is a TCP packet.
  if (ip->version != 0x4) {
    ++c->non_ipv4_packet_total;
    EXIT(XDP_PASS);
  }
  if (ip->ihl != 0x5) {
    ++c->ip_option_packet_total;
    EXIT(XDP_PASS);
  }
  if (ip->daddr != config->vip_address) {
    ++c->no_vip_match_total;
    EXIT(XDP_PASS);
  }

  struct destination_entry* dest;
  if (ip->protocol == IPPROTO_TCP) {
      // Now, we've verified that the packet is a TCP packet destined to the VIP.
      // Record them in the stats as they are eligible for load balancing.
      ++c->rx_packet_total;
      c->rx_total_size += data_end - data;

      if(data + sizeof(struct ethhdr) + sizeof(struct iphdr) +
            sizeof(struct tcphdr) > data_end) {
        ++c->too_short_packet_total;
        EXIT(XDP_PASS);
      }

      struct tcphdr* tcp = (struct tcphdr*)(ip + 1);

      uint32_t key = ip->saddr + tcp->source;
      debugk("incoming packet: ip=%pI4 port=%u", &ip->saddr, ntohs(tcp->source));

      uint32_t dest_idx = (key % config->num_dests) + 1;
      debugk("dest_idx=%d", dest_idx);
      dest = bpf_map_lookup_elem(&destinations_map, &dest_idx);
      if (!dest) {
        debugk("ASSERTION FAILURE: no dest entry for %d", dest_idx);
        EXIT(XDP_DROP);
      }
  
  }
  else if(ip->protocol == IPPROTO_UDP) {
     // Now, we've verified that the packet is a TCP packet destined to the VIP.
      // Record them in the stats as they are eligible for load balancing.
      ++c->rx_packet_total;
      c->rx_total_size += data_end - data;

      if(data + sizeof(struct ethhdr) + sizeof(struct iphdr) +
            sizeof(struct udphdr) + 1 /*for QUIC first byte*/ > data_end) {
        ++c->too_short_packet_total;
        EXIT(XDP_PASS);
      }

      // TODO: validate destination port?
      // QUIC fixed bitとして0x40というのがあるがあれはgrease拡張等で自由に反転できてしまうため
      // 判定として信用しちゃだめだと思われる see https://datatracker.ietf.org/doc/html/rfc9287
      struct udphdr* udp = (struct udphdr*)(ip + 1);
      const char* quic_first_byte = (const char*)(udp + 1);
      const int is_long_header = (quic_first_byte[0] & 0x80) == 0x80 ? 1 : 0;
      if(is_long_header) {
        ++c->quiclb_long_packet_total;
        // Long header
        /*RFC 9000 says least 8 byte for initial dst connection id so this least 1 byte requirements for destionation connection id is always satisfied*/
        if(data + sizeof(struct ethhdr) + sizeof(struct iphdr) +
              sizeof(struct udphdr) + sizeof(struct quic_long_packet) + 1 > data_end) {
          ++c->quiclb_too_short_long_packet_total;
          EXIT(XDP_PASS);
        }

        struct quic_long_packet* quic = (struct quic_long_packet*)(udp + 1);
        if(quic->dst_conn_id_len != QUICLB_CONNECTION_ID_SIZE) {
          if(quic->dst_conn_id_len == 0) {
            // This is a connection ID rotation packet, which we do not support.
            ++c->quiclb_no_connection_id_total;
            EXIT(XDP_PASS);
          }
          dest = handle_initial((struct quiclb_connection_id*)(quic+1) , config, c, "long");
          if (!dest) {
            ++c->quiclb_no_dest_entry_total;
            EXIT(XDP_DROP);
          }
        }
        else {
          if(data + sizeof(struct ethhdr) + sizeof(struct iphdr) +
              sizeof(struct udphdr) + sizeof(struct quic_long_packet) + sizeof(struct quiclb_connection_id) > data_end) {
            ++c->quiclb_too_short_long_packet_total;
            EXIT(XDP_PASS);
          }
          // TODO: add validation of mac or other validation logic?
          struct quiclb_connection_id* conn_id =
              (struct quiclb_connection_id*)(quic + 1);
     
          dest = handle_connection_id(conn_id, config,c, "long");
          if (!dest) {
            EXIT(XDP_DROP);
          }
        }
      } else {
        ++c->quiclb_short_packet_total;
        // Short header
        if(data + sizeof(struct ethhdr) + sizeof(struct iphdr) +
              sizeof(struct udphdr) + sizeof(struct quic_short_packet) + sizeof(struct quiclb_connection_id) > data_end) {
          ++c->too_short_packet_total;
          EXIT(XDP_PASS);
        }

        struct quic_short_packet* quic = (struct quic_short_packet*)(udp + 1);
        struct quiclb_connection_id* conn_id =
            (struct quiclb_connection_id*)(quic + 1);

        dest = handle_connection_id(conn_id,config,c,"short");

        if (!dest) {
          EXIT(XDP_DROP);
        }
      }
      debugk("handled QUIC packet: ip=%pI4 port=%u form=%s", &ip->saddr, ntohs(udp->source),
             is_long_header ? "long" : "short");
  } else {
    ++c->non_supported_proto_packet_total;
    EXIT(XDP_PASS);
  }

  debugk("dest ip=%pI4", &dest->ip_address);
  debugk("dest mac=%02x:%02x:%02x", dest->mac_address[0], dest->mac_address[1], dest->mac_address[2]);
  debugk("         %02x:%02x:%02x", dest->mac_address[3], dest->mac_address[4], dest->mac_address[5]);

  // make room for the additional IP header (IPIP encapsulation)
  if (bpf_xdp_adjust_head(ctx, -(int)sizeof(struct iphdr))) {
    ++c->failed_adjust_head_total;
    EXIT(XDP_DROP);
  }

  // make verifier happy - this is guaranteed by the `bpf_xdp_adjust_head`
  // success, but the verifier is not currently smart enough to know that.
  if (ctx->data + sizeof(struct ethhdr) + sizeof(struct iphdr) +
          sizeof(struct iphdr) >
      ctx->data_end) {
    debugk("NOT REACHED!!!");
    EXIT(XDP_DROP);
  }

  // Construct new eth header - the encap packet is from the LB to the
  // destination cache node.
  eth = (void*)(uint64_t)ctx->data;
  eth->h_proto = htons(ETH_P_IP);

  memcpy(eth->h_source, src_entry->mac_address, sizeof(src_entry->mac_address));
  memcpy(eth->h_dest, dest->mac_address, sizeof(dest->mac_address));

  // Construct the IPIP header.
  struct iphdr* ip2 = (void*)(eth + 1);

  ip = (void*)(ip2 + 1);
  uint16_t iphdr_tot_len = ntohs(ip->tot_len); // FIXME - should be always fixed.

  ip2->version = 4;
  ip2->ihl = 0x5;
  ip2->tos = 0;
  ip2->tot_len = htons(iphdr_tot_len + sizeof(struct iphdr));
  ip2->id = ~ip->id;
  ip2->frag_off = htons(IP_DF);
  ip2->ttl = 64;
  ip2->protocol = IPPROTO_IPIP;
  ip2->check = 0;
  ip2->saddr = src_entry->ip_address;
  ip2->daddr = dest->ip_address;

  // Calculate the checksum of the IPIP header.
  uint32_t sum = 0;
  for (int i = 0; i < sizeof(struct iphdr) / 2; i++) {
    sum += ((uint16_t*)ip2)[i];
  }
  sum = (sum & 0xffff) + (sum >> 16);
  ip2->check = ~sum;

  // Drop padding of the original packet if needed
  ssize_t padding = ETH_ZLEN - (sizeof(struct ethhdr) + iphdr_tot_len);
  if (padding > 0) {
    if (bpf_xdp_adjust_tail(ctx, -padding)) {
      ++c->failed_adjust_tail_total;
      EXIT(XDP_DROP);
    }
  }

  // Redirect the packet to the destination.
  // FIXME: depending on encap_size, it is possible that the encaped needs
  // padding back again too.
  EXIT(XDP_TX);
}

// see DEBUG_LB_MAIN
#undef debugk

char _license[] SEC("license") = "Dual BSD/GPL";
