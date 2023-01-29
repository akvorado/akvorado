// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package yaml

import "gopkg.in/yaml.v3"

// Marshal serializes the value provided into a YAML document. The structure of
// the generated document will reflect the structure of the value itself. Maps
// and pointers (to struct, string, int, etc) are accepted as the in value.
func Marshal(in interface{}) (out []byte, err error) {
	return yaml.Marshal(in)
}
