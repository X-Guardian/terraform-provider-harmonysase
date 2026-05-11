// SPDX-License-Identifier: MPL-2.0

package provider

import "testing"

func TestJSONUnmarshal(t *testing.T) {
	t.Run("valid object decodes into struct", func(t *testing.T) {
		var out struct {
			ID   string `json:"id"`
			Size int    `json:"size"`
		}
		if err := jsonUnmarshal([]byte(`{"id":"n1","size":3}`), &out); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.ID != "n1" || out.Size != 3 {
			t.Errorf("got %+v, want {ID:n1 Size:3}", out)
		}
	})

	t.Run("invalid json returns error", func(t *testing.T) {
		var out map[string]any
		if err := jsonUnmarshal([]byte(`{not-json`), &out); err == nil {
			t.Fatal("expected error for malformed json, got nil")
		}
	})

	t.Run("empty payload returns error", func(t *testing.T) {
		var out map[string]any
		if err := jsonUnmarshal([]byte{}, &out); err == nil {
			t.Fatal("expected error for empty payload, got nil")
		}
	})
}
