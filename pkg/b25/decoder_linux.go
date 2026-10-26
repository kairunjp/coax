// Copyright 2026
// SPDX-License-Identifier: Apache-2.0 OR GPL-2.0-or-later

//go:build linux && cgo

package b25

/*
#cgo LDFLAGS: -ldl
#include <dlfcn.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>

// Prefixes of the public libaribb25 structures. Their order and signatures
// match aribb25/{arib_std_b25,b_cas_card}.h, without requiring development headers.
typedef struct {
    uint8_t *data;
    uint32_t size;
} COAX_B25_BUFFER;

typedef struct COAX_BCAS {
    void *private_data;
    void (*release)(void *);
    int (*init)(void *);
} COAX_BCAS;

typedef struct COAX_ARIB_B25 {
    void *private_data;
    void (*release)(void *);
    int (*set_multi2_round)(void *, int32_t);
    int (*set_strip)(void *, int32_t);
    int (*set_emm_proc)(void *, int32_t);
    int (*set_simd_mode)(void *, int32_t);
    int32_t (*get_simd_mode)(void *);
    int (*set_b_cas_card)(void *, COAX_BCAS *);
    int (*set_unit_size)(void *, int);
    int (*reset)(void *);
    int (*flush)(void *);
    int (*put)(void *, COAX_B25_BUFFER *);
    int (*get)(void *, COAX_B25_BUFFER *);
} COAX_ARIB_B25;

typedef struct {
    void *library;
    COAX_ARIB_B25 *decoder;
    COAX_BCAS *card;
} COAX_B25_CONTEXT;

static void coax_b25_release(COAX_B25_CONTEXT *ctx) {
    if (!ctx) return;
    if (ctx->decoder) ctx->decoder->release(ctx->decoder);
    if (ctx->card) ctx->card->release(ctx->card);
    if (ctx->library) dlclose(ctx->library);
    free(ctx);
}

static COAX_B25_CONTEXT *coax_b25_open(const char *path, char **error, int *code) {
    *error = NULL;
    *code = 0;
    COAX_B25_CONTEXT *ctx = calloc(1, sizeof(*ctx));
    if (!ctx) {
        *error = strdup("out of memory");
        return NULL;
    }
    ctx->library = dlopen(path, RTLD_NOW | RTLD_LOCAL);
    if (!ctx->library) {
        const char *why = dlerror();
        *error = strdup(why ? why : "dlopen failed");
        goto fail;
    }
    // Resolve both symbols from the explicitly requested library.
    dlerror();
    COAX_ARIB_B25 *(*make_decoder)(void) =
        (COAX_ARIB_B25 *(*)(void))dlsym(ctx->library, "create_arib_std_b25");
    const char *why = dlerror();
    if (why || !make_decoder) {
        *error = strdup(why ? why : "create_arib_std_b25 unavailable");
        goto fail;
    }
    dlerror();
    COAX_BCAS *(*make_card)(void) =
        (COAX_BCAS *(*)(void))dlsym(ctx->library, "create_b_cas_card");
    why = dlerror();
    if (why || !make_card) {
        *error = strdup(why ? why : "create_b_cas_card unavailable");
        goto fail;
    }
    ctx->decoder = make_decoder();
    ctx->card = make_card();
    if (!ctx->decoder || !ctx->card) {
        *error = strdup("libaribb25 could not create its decoder or card object");
        goto fail;
    }
    *code = ctx->decoder->set_multi2_round(ctx->decoder, 4);
    if (*code < 0) goto config_fail;
    *code = ctx->decoder->set_strip(ctx->decoder, 0);
    if (*code < 0) goto config_fail;
    *code = ctx->decoder->set_emm_proc(ctx->decoder, 0);
    if (*code < 0) goto config_fail;
    *code = ctx->card->init(ctx->card);
    if (*code < 0) {
        *error = strdup("B-CAS card initialization failed (check PC/SC and card access)");
        goto fail;
    }
    *code = ctx->decoder->set_b_cas_card(ctx->decoder, ctx->card);
    if (*code < 0) goto config_fail;
    return ctx;

config_fail:
    *error = strdup("libaribb25 initialization failed");
fail:
    coax_b25_release(ctx);
    return NULL;
}

static int coax_b25_put(COAX_B25_CONTEXT *ctx, void *bytes, uint32_t size) {
    COAX_B25_BUFFER in = {(uint8_t *)bytes, size};
    return ctx->decoder->put(ctx->decoder, &in);
}

static int coax_b25_get(COAX_B25_CONTEXT *ctx, COAX_B25_BUFFER *out) {
    memset(out, 0, sizeof(*out));
    return ctx->decoder->get(ctx->decoder, out);
}

static int coax_b25_flush(COAX_B25_CONTEXT *ctx) {
    return ctx->decoder->flush(ctx->decoder);
}
*/
import "C"

import (
	"errors"
	"fmt"
	"io"
	"unsafe"
)

const libraryPath = "/usr/local/lib/libaribb25.so"
const maxInputChunk = 64 * 1024

// Decoder passes TS data through libaribb25, writing the decoded TS to dst.
// It is not safe for concurrent use. Call Close once the input ends to flush
// packets buffered for PAT/PMT/ECM detection. Close does not close dst.
type Decoder struct {
	dst    io.Writer
	native *C.COAX_B25_CONTEXT
	closed bool
	failed error
}

func New(dst io.Writer) (*Decoder, error) {
	return newWithPath(dst, libraryPath)
}

func newWithPath(dst io.Writer, sharedLibrary string) (*Decoder, error) {
	if dst == nil {
		return nil, errors.New("b25: nil destination")
	}
	path := C.CString(sharedLibrary)
	defer C.free(unsafe.Pointer(path))
	var message *C.char
	var code C.int
	native := C.coax_b25_open(path, &message, &code)
	if message != nil {
		defer C.free(unsafe.Pointer(message))
	}
	if native == nil {
		reason := "unknown error"
		if message != nil {
			reason = C.GoString(message)
		}
		if code < 0 {
			return nil, fmt.Errorf("b25: %s: %s (code %d)", sharedLibrary, reason, int(code))
		}
		return nil, fmt.Errorf("b25: %s: %s", sharedLibrary, reason)
	}
	return &Decoder{dst: dst, native: native}, nil
}

// Write consumes a TS chunk, drains all currently available decoded bytes,
// and propagates decoding and output errors instead of silently writing raw TS.
func (d *Decoder) Write(p []byte) (int, error) {
	if d.closed {
		return 0, errors.New("b25: write after close")
	}
	if d.failed != nil {
		return 0, d.failed
	}
	consumed := 0
	for len(p) != 0 {
		chunk := len(p)
		if chunk > maxInputChunk {
			chunk = maxInputChunk
		}
		// Allocate C-owned memory; libaribb25 may buffer input internally.
		input := C.CBytes(p[:chunk])
		if input == nil {
			d.failed = errors.New("b25: could not allocate input buffer")
			return consumed, d.failed
		}
		code := C.coax_b25_put(d.native, input, C.uint32_t(chunk))
		C.free(input)
		if code < 0 {
			d.failed = fmt.Errorf("b25: put failed (code %d)", int(code))
			return consumed, d.failed
		}
		consumed += chunk
		p = p[chunk:]
		if err := d.drain(); err != nil {
			d.failed = err
			return consumed, err
		}
	}
	return consumed, nil
}

func (d *Decoder) drain() error {
	var output C.COAX_B25_BUFFER
	code := C.coax_b25_get(d.native, &output)
	if code < 0 {
		return fmt.Errorf("b25: get failed (code %d)", int(code))
	}
	if output.size == 0 {
		return nil
	}
	if output.data == nil {
		return errors.New("b25: get returned a nil data pointer")
	}
	// libaribb25 owns the buffer until the next put/get/flush/release call.
	// io.Writer promises not to retain the supplied slice.
	data := unsafe.Slice((*byte)(unsafe.Pointer(output.data)), int(output.size))
	n, err := d.dst.Write(data)
	if err == nil && n != len(data) {
		err = io.ErrShortWrite
	}
	if err != nil {
		return fmt.Errorf("b25: write decoded TS: %w", err)
	}
	return nil
}

func (d *Decoder) Close() error {
	if d.closed {
		return nil
	}
	d.closed = true
	defer func() {
		C.coax_b25_release(d.native)
		d.native = nil
	}()
	if d.failed != nil {
		return d.failed
	}
	if code := C.coax_b25_flush(d.native); code < 0 {
		return fmt.Errorf("b25: flush failed (code %d)", int(code))
	}
	return d.drain()
}
