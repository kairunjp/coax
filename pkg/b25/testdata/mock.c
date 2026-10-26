// Mock of the public libaribb25 ABI, intentionally independent of libaribb25.
// It transforms ASCII bytes, rather than implementing broadcast decryption.
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

typedef struct { uint8_t *data; uint32_t size; } BUFFER;
typedef struct B_CAS {
    void *private_data;
    void (*release)(void *);
    int (*init)(void *);
} B_CAS;
typedef struct ARIB_B25 {
    void *private_data;
    void (*release)(void *);
    int (*set_round)(void *, int32_t);
    int (*set_strip)(void *, int32_t);
    int (*set_emm)(void *, int32_t);
    int (*set_simd)(void *, int32_t);
    int32_t (*get_simd)(void *);
    int (*set_card)(void *, B_CAS *);
    int (*set_size)(void *, int);
    int (*reset)(void *);
    int (*flush)(void *);
    int (*put)(void *, BUFFER *);
    int (*get)(void *, BUFFER *);
} ARIB_B25;

typedef struct { ARIB_B25 table; uint8_t *p; uint32_t n; } MOCK;

static void release_b25(void *p) { MOCK *d = p; free(d->p); free(d); }
static int config(void *p, int32_t v) { (void)p; (void)v; return 0; }
static int set_card(void *p, B_CAS *c) { (void)p; return c ? 0 : -1; }
static int b25_flush(void *p) { MOCK *d = p; d->p = realloc(d->p, d->n+1); d->p[d->n++] = '!'; return 0; }
static int b25_put(void *p, BUFFER *b) {
    MOCK *d = p;
    free(d->p);
    d->p = malloc(b->size);
    if (!d->p) return -2;
    d->n = b->size;
    for (uint32_t i=0; i<b->size; i++) d->p[i] = b->data[i] + 1;
    return 0;
}
static int b25_get(void *p, BUFFER *b) {
    MOCK *d = p;
    b->data = d->p;
    b->size = d->n;
    d->n = 0;
    return 0;
}
static void release_card(void *p) { free(p); }
static int init_card(void *p) { (void)p; return getenv("MOCK_B25_CARD_FAIL") ? -42 : 0; }

ARIB_B25 *create_arib_std_b25(void) {
    MOCK *d = calloc(1, sizeof(MOCK));
    if (!d) return NULL;
    d->table.release = release_b25;
    d->table.set_round = config;
    d->table.set_strip = config;
    d->table.set_emm = config;
    d->table.set_simd = config;
    d->table.set_card = set_card;
    d->table.set_size = config;
    d->table.flush = b25_flush;
    d->table.put = b25_put;
    d->table.get = b25_get;
    return &d->table;
}
B_CAS *create_b_cas_card(void) {
    B_CAS *card = calloc(1, sizeof(B_CAS));
    if (!card) return NULL;
    card->release = release_card;
    card->init = init_card;
    return card;
}
