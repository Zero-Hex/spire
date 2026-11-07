package zoneloot

import "testing"

func TestNamedNPCs(t *testing.T) {
	for name, want := range map[string]bool{"King_Gragnar": true, "#Wilslik_Gissu": true, "The_Muglwump": true,
		"a_kobold": false, "cave_bat_lord": false, "an_undead_rat": false, "#a_herald_of_mischief": true} {
		if isNamed(name) != want {
			t.Errorf("%v: %v", name, !want)
		}
	}
}
