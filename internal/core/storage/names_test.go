package storage

import "testing"

func TestReservedName(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"con", true}, {"CON", true}, {"Con", true}, {"prn", true}, {"aux", true}, {"nul", true},
		{"com1", true}, {"com9", true}, {"lpt1", true}, {"LPT9", true},
		{"con.txt", true}, {"con.family.math", true}, {"nul.json", true}, {"con ", true},
		{"com0", false}, {"lpt0", false}, {"com10", false}, {"com", false}, {"lpt", false},
		{"console", false}, {"conn", false}, {"auxiliary", false}, {"nulls", false}, {"mia", false},
		{"", false}, {".con", false}, {"a.con", false}, {"org.example.con", false},
		{"cassette.json", false}, {"scene.svg", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ReservedName(tt.name); got != tt.want {
				t.Errorf("ReservedName(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}
