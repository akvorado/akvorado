// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package decoder

import (
	"io"
	"testing"

	"akvorado/common/helpers"
)

func TestNetFlowSplit(t *testing.T) {
	message := make([]byte, 20)
	copy(message, []byte{0, 10, 0, 20})
	cases := []struct {
		Description string
		Data        []byte
		AtEOF       bool
		Advance     int
		Token       []byte
		Error       error
	}{
		{
			Description: "empty",
			Data:        []byte{},
		}, {
			Description: "empty at EOF",
			Data:        []byte{},
			AtEOF:       true,
		}, {
			Description: "partial header",
			Data:        []byte{0, 10, 0},
		}, {
			Description: "partial header at EOF",
			Data:        []byte{0, 10, 0},
			AtEOF:       true,
			Error:       io.ErrUnexpectedEOF,
		}, {
			Description: "partial message",
			Data:        message[:10],
		}, {
			Description: "partial message at EOF",
			Data:        message[:10],
			AtEOF:       true,
			Error:       io.ErrUnexpectedEOF,
		}, {
			Description: "complete message",
			Data:        message,
			Advance:     20,
			Token:       message,
		}, {
			Description: "complete message with extra data",
			Data:        append(message, 0, 10),
			Advance:     20,
			Token:       message,
		},
	}
	for _, tc := range cases {
		t.Run(tc.Description, func(t *testing.T) {
			advance, token, err := netflow{}.Split(tc.Data, tc.AtEOF)
			if err != tc.Error {
				t.Fatalf("Split() error:\n%+v", err)
			}
			if diff := helpers.Diff(advance, tc.Advance); diff != "" {
				t.Errorf("Split() advance (-got, +want):\n%s", diff)
			}
			if diff := helpers.Diff(token, tc.Token); diff != "" {
				t.Errorf("Split() token (-got, +want):\n%s", diff)
			}
		})
	}

	// Invalid headers
	for _, data := range [][]byte{
		{0, 9, 0, 20},
		{0, 10, 0, 4},
	} {
		if _, _, err := (netflow{}).Split(data, false); err == nil {
			t.Errorf("Split(%v) did not error", data)
		}
	}
}
