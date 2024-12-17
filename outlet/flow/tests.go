// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

//go:build !release

package flow

import (
	"akvorado/common/pb"
	"akvorado/outlet/flow/decoder/gob"
)

func init() {
	// Add gob decoder for testing purposes only
	availableDecoders[pb.RawFlow_DECODER_GOB] = gob.New
}
