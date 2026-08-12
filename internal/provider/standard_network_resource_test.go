// SPDX-License-Identifier: MPL-2.0

package provider

import "testing"

func TestRegionPayloadFromModel_Nil(t *testing.T) {
	got := regionPayloadFromModel(nil)
	if got.HarmonySaseRegionID != "" || got.ScaleUnits != nil || got.Idle {
		t.Errorf("expected zero payload, got %+v", got)
	}
}
