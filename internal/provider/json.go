// SPDX-License-Identifier: MPL-2.0

package provider

import "encoding/json"

// jsonUnmarshal is a thin wrapper kept in this package so resource files can
// decode async-status `result` payloads without importing encoding/json
// directly (which would muddy go imports across many files).
func jsonUnmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}
