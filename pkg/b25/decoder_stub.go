// Copyright 2026
// SPDX-License-Identifier: Apache-2.0 OR GPL-2.0-or-later

//go:build !linux || !cgo

package b25

import (
	"errors"
	"io"
)

// Decoder is unavailable on platforms without Linux and cgo.
type Decoder struct{}

func New(_ io.Writer) (*Decoder, error) {
	return nil, errors.New("b25: --b25 requires Linux and a cgo-enabled build")
}

func (*Decoder) Write(_ []byte) (int, error) {
	return 0, errors.New("b25: decoder is unavailable")
}

func (*Decoder) Close() error { return nil }
