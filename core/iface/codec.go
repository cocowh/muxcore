// Copyright (c) 2025 cocowh. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package iface

type Codec interface {
	Encode(v any) ([]byte, error)
	Decode([]byte) (any, error)
}
