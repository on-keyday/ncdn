#include <assert.h>
#include <stdint.h>
#include <linux/bpf.h>
#include <linux/if_ether.h>
#include <netinet/in.h>
#include <netinet/ip.h>
#include <netinet/tcp.h>
#include <netinet/udp.h>
#include <netinet/ip_icmp.h>
#include <stdbool.h>


#include <bpf/bpf_helpers.h>
#include "lb.h"

#include <stdint.h>
#include <sys/types.h>
#include <errno.h>

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
  uint64_t tcp_packet_total; // HELP Number of TCP packets received.
  uint64_t mtu_exceeded_total; // HELP Number of packets dropped due to MTU exceeded.

  uint64_t quiclb_no_dest_port_match_total; // HELP Number of QUIC-LB packets received without matching destination port.
  uint64_t quiclb_short_packet_total; // HELP Number of QUIC-LB short packets received.
  uint64_t quiclb_long_packet_total; // HELP Number of QUIC-LB long packets received.
  uint64_t quiclb_initial_routing_packet_total; // HELP Number of QUIC-LB initial routing packets received.
  uint64_t quiclb_too_short_long_packet_total; // HELP Number of QUIC-LB short-long packets received.
  uint64_t quiclb_no_connection_id_total; // HELP Number of QUIC-LB packets received without connection ID.
  uint64_t quiclb_no_dest_entry_total; // HELP Number of QUIC-LB packets received without destination entry.
  uint64_t quiclb_invalid_crypto_context_total; // HELP Number of QUIC-LB packets received with invalid crypto context.
  uint64_t quiclb_encrypt_success_call_total; // HELP Number of QUIC-LB packets that called bpf_crypto_encrypt.
  uint64_t quiclb_decrypt_success_call_total; // HELP Number of QUIC-LB packets that called bpf_crypto_decrypt.
  uint64_t quiclb_connid_cache_hit_total; // HELP Number of QUIC-LB connection ID cache hits.
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
  uint16_t mtu;
  uint16_t quic_dest_port;
  uint32_t flags;
  uint32_t server_id_hash_key; 
} PACKED;

#define LB_CONFIG_FLAG_CONNID_CACHE_ENABLED (1 << 0)

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
  uint32_t hash_key; // hash key for consistent hashing
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

#define QUICLB_CONNECTION_ID_SIZE 17
#define QUICLB_CONNECTION_ID_CONFIG_ROTATION(x) (((x) & 0xe0) >> 5)
#define QUICLB_CONNECTION_ID_RANDOM_VALUE(x) ((x) & 0x1f)
#define QUICLB_CONNECTION_ID_SERVER_ID(x) ((((uint32_t)(x[0]) << 24) | \
   ((uint32_t)(x[1]) << 16) | \
   ((uint32_t)(x[2]) << 8) | \
   ((uint32_t)(x[3]))))

// big-endian format
struct quiclb_connection_id {
  uint8_t first_byte; // 3 bit: config_rotation, 5 bit: random value
  uint8_t server_id[4]; // 4 byte server id, to ensure server id space enough large
  uint8_t nonce[12]; // 12 byte padding, can be used for future extensions
} PACKED; // 17 bytes total, for optimization

// LRUキャッシュの定義
struct {
    __uint(type, BPF_MAP_TYPE_LRU_HASH);
    __uint(max_entries, 1024);
    __type(key, struct quiclb_connection_id);
    __type(value, __u32);
} connid_cache SEC(".maps");


// 普通にメモリコピーのループを使うと最適化により
// error: lb.c:294:15: in function test_decrypt i32 (ptr): A call to built-in function 'memset' is not supported.
// のようなエラーになるためvolatileを使う
#define VOLATILE(x) ((volatile uint8_t*)(x))
#define HALF_LEN(x) (((x) + 1) / 2)
#define CHECK_VOLATILE(x,cmp,ret) if(*VOLATILE(&x) > cmp) { \
    debugk("ASSERTION FAILURE: %s > %d", #x, cmp); \
    return ret; \
}
#define DEFINE_HALF_LEN(ret) const uint8_t half_len = HALF_LEN(full_len);

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
    DEFINE_HALF_LEN();
    const uint8_t right_start = full_len - half_len;
    for (uint8_t i = 0; i < half_len; i++) {
        VOLATILE(left)[i] = input[i];
    }
    for (uint8_t i = 0; i < half_len; i++) {
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

__always_inline void print_full_connection_id(const char* name, const uint8_t* connection_id,uint8_t len) {
    uint64_t high, middle;
    uint32_t low;
#define ASSIGN(VAR,TYPE,SHIFT,INDEX) if(INDEX < len) { VAR |= ((TYPE)(connection_id[INDEX]) << SHIFT); }
    high = 0;
    ASSIGN(high, uint64_t, 56, 0);
    ASSIGN(high, uint64_t, 48, 1);
    ASSIGN(high, uint64_t, 40, 2);
    ASSIGN(high, uint64_t, 32, 3);
    ASSIGN(high, uint64_t, 24, 4);
    ASSIGN(high, uint64_t, 16, 5);
    ASSIGN(high, uint64_t, 8, 6);
    ASSIGN(high, uint64_t, 0, 7);
    middle = 0;
    ASSIGN(middle, uint64_t, 56, 8);
    ASSIGN(middle, uint64_t, 48, 9);
    ASSIGN(middle, uint64_t, 40, 10);
    ASSIGN(middle, uint64_t, 32, 11);
    ASSIGN(middle, uint64_t, 24, 12);
    ASSIGN(middle, uint64_t, 16, 13);
    ASSIGN(middle, uint64_t, 8, 14);
    ASSIGN(middle, uint64_t, 0, 15);
    low = 0;
    ASSIGN(low, uint32_t, 24, 16);
    ASSIGN(low, uint32_t, 16, 17);
    ASSIGN(low, uint32_t, 8, 18);
    ASSIGN(low, uint32_t, 0, 19);
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
    uint8_t rotation = QUICLB_CONNECTION_ID_CONFIG_ROTATION(connection_id_bytes[0]);
    if(rotation == 7) {
        debugk("%s: rotation is 7, which is not supported", context);
        return -ENOTSUP;
    }
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
    print_full_connection_id("encrypted_conn_id", connection_id_bytes, input_len);
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
      print_full_connection_id("decrypted_conn_id", output, input_len);
      return 0;
    }

    // see 4.4.2.  General Case: Four-Pass Encryption
    const int input_is_odd = full_len & 1;
    DEFINE_HALF_LEN();
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
    print_full_connection_id("decrypted_conn_id", output, input_len);
    return 0;
}

#define FNV_PRIME_32 16777619
#define FNV_OFFSET_32 2166136261


// とりあえず使える
__always_inline uint32_t fnv1a_hash(const void* data, size_t size) {
    const uint8_t* p = (const uint8_t*)data;
    uint32_t hash = FNV_OFFSET_32;

    for (size_t i = 0; i < size; ++i) {
        hash ^= p[i];
        hash *= FNV_PRIME_32;
    }

    return hash;
}


__always_inline uint32_t hash_server_id(struct lb_config* config,uint32_t src_ip,uint16_t src_port, uint8_t connid_first) {
    uint8_t buffer[7 + 4];
    buffer[0] = (src_ip >> 24) & 0xFF; // first byte of src_ip
    buffer[1] = (src_ip >> 16) & 0xFF; // second byte of src_ip
    buffer[2] = (src_ip >> 8) & 0xFF;  // third byte of src_ip
    buffer[3] = src_ip & 0xFF;         // fourth byte of src_ip
    buffer[4] = (src_port >> 8) & 0xFF; // first byte of src_port
    buffer[5] = src_port & 0xFF;        // second byte of src_port
    buffer[6] = connid_first;           // first byte of connection id
    buffer[7] = (config->server_id_hash_key >> 24) & 0xFF; // first byte of server_id_hash_key
    buffer[8] = (config->server_id_hash_key >> 16) & 0xFF; // second byte of server_id_hash_key
    buffer[9] = (config->server_id_hash_key >> 8) & 0xFF;  // third byte of server_id_hash_key
    buffer[10] = config->server_id_hash_key & 0xFF;         // fourth byte of server_id_hash_key
    // hash the server id based on src_ip, src_port and first byte of connection id
    uint32_t hash = fnv1a_hash(buffer, sizeof(buffer));
    debugk("DEBUG: hash_server_id: src_ip=%u, src_port=%u, connid_first=%u, hash=%u", src_ip, src_port, connid_first, hash);
    return hash;
}

__always_inline struct destination_entry* search_consistent_hash_based_destination(struct lb_config* config,uint32_t server_id) {
  // destination map is sorted by server_id, so we can use binary search
  // maximum entries is 255, so least 8 times of binary search is enough
  const uint32_t len = config->num_dests;
  uint32_t low = 1; // 1-based index
  uint32_t high = len;
  uint32_t mid = 0;
  size_t counter = 0;
  struct destination_entry* dest = NULL;
  while (low <= high && counter++ < 8) {
    mid = (low + high) / 2;
    dest = bpf_map_lookup_elem(&destinations_map, &mid);
    if (!dest) {
      debugk("BUG: search_consistent_hash_based_destination: no destination entry for mid %u", mid);
      return NULL; // no destination entry found
    }
    if (dest->hash_key == server_id) {
      return dest;
    }
    if (dest->hash_key < server_id) {
      low = mid + 1;
    } else {
      high = mid - 1;
    }
  }
  if(!dest) {
    debugk("search_consistent_hash_based_destination: no destination entry found for server_id %u", server_id);
    return NULL; // no destination entry found
  }
  if(server_id < dest->hash_key) {
    return dest; // return the next higher destination entry
  }
  // mid is [1, len]
  // if 5 elements
  // (1 % 5) + 1 = 2
  // (2 % 5) + 1 = 3
  // (3 % 5) + 1 = 4
  // (4 % 5) + 1 = 5
  // (5 % 5) + 1 = 1
  // so this is a wrap around
  mid = (mid % len) +1; // wrap around to the first entry if needed
  dest = bpf_map_lookup_elem(&destinations_map, &mid);
  if (!dest) {
    debugk("search_consistent_hash_based_destination: no destination entry found for mid %u", mid);
    return NULL; // no destination entry found  
  }
  debugk("search_consistent_hash_based_destination: found destination entry for server_id %u at mid %u", server_id, mid);
  return dest; // return the next higher destination entry
}

__always_inline struct destination_entry* handle_initial(uint32_t server_id,
struct lb_config* config,
struct stat_counters* c,
                                               const char* context) {
    debugk("%s (initial) server_id=%u", context, server_id);
    struct destination_entry* dest = search_consistent_hash_based_destination(config, server_id);
    if (!dest) {
      debugk("ASSERTION FAILURE: no dest entry for %d", server_id);
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
                                               const char* context,
                                               uint32_t src_ip,
                                               uint16_t src_port
                                              ) {
    uint32_t* cached_server_id;
    if(config->flags & LB_CONFIG_FLAG_CONNID_CACHE_ENABLED) {
      cached_server_id = bpf_map_lookup_elem(&connid_cache, conn_id);
    } else {
      cached_server_id = NULL;
    }
    uint32_t server_id;
    if(!cached_server_id) {
      uint8_t output[QUICLB_CONNECTION_ID_SIZE];
      const uint8_t* conn_id_bytes  = (const uint8_t*)conn_id;
      uint8_t rotation = QUICLB_CONNECTION_ID_CONFIG_ROTATION(conn_id_bytes[0]);
      if(rotation == 7) {
        debugk("%s: rotation is 7, handle with original routing info", context);
        return handle_initial(hash_server_id(config, src_ip, src_port, conn_id_bytes[5/*nonce pos*/]), config, c, context);
      }
      int err = connection_id_decrypt(output, conn_id_bytes, QUICLB_CONNECTION_ID_SIZE, false, context, c);
      if(err < 0) {
        ++c->quiclb_invalid_crypto_context_total;
        debugk("%s: connection_id_decrypt failed with %d", context, err);
        return NULL;
      }
      server_id = QUICLB_CONNECTION_ID_SERVER_ID(((struct quiclb_connection_id*)(output))->server_id);
      debugk("%s conn_id server_id=%d", context, server_id);
      // update cache
      int cache_err = bpf_map_update_elem(&connid_cache, conn_id, &server_id, BPF_ANY);
      if(cache_err < 0) {
        debugk("%s: bpf_map_update_elem failed with %d, but continue", context, cache_err);
      }
    } else {
      ++c->quiclb_connid_cache_hit_total;
      server_id = *cached_server_id;
    }

    struct destination_entry* entry = search_consistent_hash_based_destination(config, server_id);
    if(!entry) {
      return NULL;
    }
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

__always_inline void compute_ip_checksum(struct iphdr* ip) {
    // Reset checksum
    ip->check = 0;
    uint32_t  sum = 0;
    for(int i = 0; i < sizeof(struct iphdr) / 2; i++) {
        // Calculate checksum using 16-bit words
        __u16* word = (__u16*)((uint8_t*)ip + i * 2);
        sum += *word;
    }
    // Fold 32-bit sum to 16 bits
    sum = (sum >> 16) + (sum & 0xffff);
    ip->check = ~sum;
}


__always_inline void send_mtu_exceeded(struct xdp_md* md,const struct lb_config* config) {
    void* data = (void*)(uint64_t)md->data;
    void* data_end = (void*)(uint64_t)md->data_end;
    const int hdr_size = sizeof(struct ethhdr) + 
                         sizeof(struct iphdr) + 
                         sizeof(struct icmphdr) +    
                         sizeof(struct iphdr) +
                         8;
    if (data + hdr_size > data_end) {
        debugk("ASSERTION FAILURE: packet too short for MTU exceeded response");
        return;
    }
    struct ethhdr* eth = data;
    uint8_t tmp_mac[ETH_ALEN];
    memcpy(tmp_mac, eth->h_source, ETH_ALEN);
    memcpy(eth->h_source, eth->h_dest, ETH_ALEN);
    memcpy(eth->h_dest, tmp_mac, ETH_ALEN);
    struct iphdr* ip = (struct iphdr*)(eth + 1);
    uint8_t original_ip_header[sizeof(struct iphdr) + 8];
    memcpy(original_ip_header, ip, sizeof(struct iphdr) + 8);
    // Swap source and destination IP addresses
    __be32 temp = ip->saddr;
    ip->id = 0; // Set ID to 0 for MTU exceeded response
    ip->saddr = config->vip_address; // Use VIP as source address
    ip->daddr = temp; // Use original source address as destination address
    ip->protocol = IPPROTO_ICMP; // Change protocol to ICMP
    ip->tot_len = htons(sizeof(struct iphdr) + sizeof(struct icmphdr) + sizeof(struct iphdr) + 8);
    compute_ip_checksum(ip);
    struct icmphdr* icmp = (struct icmphdr*)(ip + 1);
    icmp->type = ICMP_DEST_UNREACH;
    icmp->code = ICMP_FRAG_NEEDED;
    icmp->un.frag.mtu = htons(config->mtu - sizeof(struct iphdr));
    debugk("DEBUG: Sending MTU exceeded response with mtu %d", config->mtu - sizeof(struct iphdr));
    icmp->checksum = 0; // checksum is not needed for XDP programs
    struct iphdr* original_ip = (struct iphdr*)(icmp + 1);
    // Copy the original IP header after the ICMP header
    memcpy(original_ip, original_ip_header, sizeof(struct iphdr) + 8);
    uint32_t sum = 0;
    const int hdr_size_icmp = sizeof(struct icmphdr) + sizeof(struct iphdr) + 8;
    for(int i = 0; i < hdr_size_icmp / 2; i++) {
        // Calculate checksum using 16-bit words
        __u16* word = (__u16*)(icmp) + i;
        sum += *word;
    }
    sum = (sum >> 16) + (sum & 0xffff);
    icmp->checksum =(uint16_t)~sum; // Final checksum
    int current_size = data_end - data;
    if (current_size > hdr_size) {
        bpf_xdp_adjust_tail(md,  hdr_size - current_size);
    }
    int new_size = md->data_end - md->data;
    debugk("DEBUG: MTU exceeded response size adjusted from %d to %d", current_size, new_size);
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
      ++c->tcp_packet_total;
      c->rx_total_size += data_end - data;

      if(data + sizeof(struct ethhdr) + sizeof(struct iphdr) +
            sizeof(struct tcphdr) > data_end) {
        ++c->too_short_packet_total;
        EXIT(XDP_PASS);
      }

      struct tcphdr* tcp = (struct tcphdr*)(ip + 1);

      debugk("incoming packet: ip=%pI4 port=%u", &ip->saddr, ntohs(tcp->source));

      dest = search_consistent_hash_based_destination(config, hash_server_id(config, ip->saddr, ntohs(tcp->source), 0x55));
      if (!dest) {
        EXIT(XDP_DROP);
      }
  
  }
  else if(ip->protocol == IPPROTO_UDP) {
      // Now, we've verified that the packet is a UDP packet destined to the VIP.
      // Record them in the stats as they are eligible for load balancing.
      ++c->rx_packet_total;
      c->rx_total_size += data_end - data;

      if(data + sizeof(struct ethhdr) + sizeof(struct iphdr) +
            sizeof(struct udphdr) + 1 /*for QUIC first byte*/ > data_end) {
        ++c->too_short_packet_total;
        EXIT(XDP_PASS);
      }

      // QUIC fixed bitとして0x40というのがあるがあれはgrease拡張等で自由に反転できてしまうため
      // 判定として信用しちゃだめだと思われる see https://datatracker.ietf.org/doc/html/rfc9287
      struct udphdr* udp = (struct udphdr*)(ip + 1);
      if(config->quic_dest_port != 0) {
        if(udp->dest != config->quic_dest_port) {
          EXIT(XDP_PASS);
        }
      }
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
          uint32_t server_id = hash_server_id(config, ip->saddr, ntohs(udp->source), ((const uint8_t*)(quic+1))[0]) + 1;
          dest = handle_initial(server_id, config, c, "long");
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
          struct quiclb_connection_id* conn_id =
              (struct quiclb_connection_id*)(quic + 1);
     
          dest = handle_connection_id(conn_id, config,c, "long", ip->saddr, ntohs(udp->source));
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

        dest = handle_connection_id(conn_id,config,c,"short", ip->saddr, ntohs(udp->source));

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

  // calculate encapsulation size
  int current_data_len = ctx->data_end - ctx->data;
  uint16_t encap_size = current_data_len + sizeof(struct iphdr) - sizeof(struct ethhdr);
  if(ETH_ZLEN > encap_size) {
    encap_size = ETH_ZLEN;
  }
  debugk("encap_size=%d", encap_size);
  if(encap_size > config->mtu) {
    debugk("encap_size %d > mtu %d, sending ICMP MTU exceeded", encap_size, config->mtu);
    send_mtu_exceeded(ctx,config);
    ++c->mtu_exceeded_total;
    EXIT(XDP_TX);
  }


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
  compute_ip_checksum(ip2);

  // Drop padding of the original packet if needed
  ssize_t padding = ETH_ZLEN - (sizeof(struct ethhdr) + iphdr_tot_len);
  if (padding > 0) {
    if (bpf_xdp_adjust_tail(ctx, -padding)) {
      ++c->failed_adjust_tail_total;
      debugk("ASSERTION FAILURE: failed to adjust tail for padding");
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
