// SPDX-License-Identifier: MPL-2.0

package provider

// splitImportID splits a comma-separated composite ID. Used for resources
// whose primary key is (parent, child).
func splitImportID(s string, n int) []string {
	out := make([]string, 0, n)
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			out = append(out, s[start:i])
			start = i + 1
			if len(out) == n-1 {
				break
			}
		}
	}
	out = append(out, s[start:])
	if len(out) != n {
		return nil
	}
	for _, p := range out {
		if p == "" {
			return nil
		}
	}
	return out
}
