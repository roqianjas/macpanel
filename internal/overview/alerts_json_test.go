package overview

import (
	"encoding/json"
	"testing"
)

// Regresi: daftar kosong harus menjadi "[]". Versi sebelumnya memakai append([]Alert(nil), ...) dan
// menghasilkan "null", yang mengosongkan seluruh UI.
func TestNonNilGivesEmptyArrayNotNull(t *testing.T) {
	for name, in := range map[string][]Alert{"nil": nil, "kosong": {}} {
		b, _ := json.Marshal(map[string]any{"alerts": nonNil(in)})
		if string(b) != `{"alerts":[]}` {
			t.Errorf("%s: %s", name, b)
		}
	}
	src := []Alert{{ID: "a", Level: "bad"}}
	out := nonNil(src)
	out[0].ID = "diubah"
	if src[0].ID != "a" {
		t.Error("hasil harus salinan, bukan irisan yang sama (cache alert tidak boleh ikut berubah)")
	}
}
