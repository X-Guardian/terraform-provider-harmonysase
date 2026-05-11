// SPDX-License-Identifier: MPL-2.0

package provider

import "testing"

func TestSplitImportID(t *testing.T) {
	cases := []struct {
		in   string
		n    int
		want []string
	}{
		{"a,b", 2, []string{"a", "b"}},
		{"net,gw,extra", 2, []string{"net", "gw,extra"}},
		{"a", 2, nil},
		{",b", 2, nil},
		{"a,", 2, nil},
		{"", 2, nil},
	}
	for _, tc := range cases {
		got := splitImportID(tc.in, tc.n)
		if len(got) != len(tc.want) {
			t.Errorf("splitImportID(%q, %d) = %v want %v", tc.in, tc.n, got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("splitImportID(%q, %d) = %v want %v", tc.in, tc.n, got, tc.want)
				break
			}
		}
	}
}
