package imap

import (
	"testing"
)

// --- NumRange tests ---

func TestNumRange_String(t *testing.T) {
	tests := []struct {
		name  string
		r     NumRange
		want  string
	}{
		{"single number", NumRange{Start: 5, Stop: 5}, "5"},
		{"range", NumRange{Start: 1, Stop: 10}, "1:10"},
		{"star range", NumRange{Start: 10, Stop: 0}, "10:*"},
		{"single 1", NumRange{Start: 1, Stop: 1}, "1"},
		{"large range", NumRange{Start: 100, Stop: 200}, "100:200"},
		{"start zero (star)", NumRange{Start: 0, Stop: 0}, "0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.r.String()
			if got != tt.want {
				t.Errorf("NumRange%+v.String() = %q, want %q", tt.r, got, tt.want)
			}
		})
	}
}

func TestNumRange_Contains(t *testing.T) {
	tests := []struct {
		name string
		r    NumRange
		num  uint32
		want bool
	}{
		{"in single", NumRange{Start: 5, Stop: 5}, 5, true},
		{"not in single", NumRange{Start: 5, Stop: 5}, 6, false},
		{"in range low", NumRange{Start: 1, Stop: 10}, 1, true},
		{"in range high", NumRange{Start: 1, Stop: 10}, 10, true},
		{"in range mid", NumRange{Start: 1, Stop: 10}, 5, true},
		{"below range", NumRange{Start: 5, Stop: 10}, 4, false},
		{"above range", NumRange{Start: 5, Stop: 10}, 11, false},
		{"star range contains high", NumRange{Start: 10, Stop: 0}, 999, true},
		{"star range contains start", NumRange{Start: 10, Stop: 0}, 10, true},
		{"star range excludes low", NumRange{Start: 10, Stop: 0}, 9, false},
		{"reversed range in", NumRange{Start: 10, Stop: 1}, 5, true},
		{"reversed range low", NumRange{Start: 10, Stop: 1}, 1, true},
		{"reversed range high", NumRange{Start: 10, Stop: 1}, 10, true},
		{"reversed range out", NumRange{Start: 10, Stop: 1}, 11, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.r.Contains(tt.num)
			if got != tt.want {
				t.Errorf("NumRange%+v.Contains(%d) = %v, want %v", tt.r, tt.num, got, tt.want)
			}
		})
	}
}

// --- ParseSeqSet tests ---

func TestParseSeqSet(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantStr string
		wantErr bool
	}{
		{"single number", "1", "1", false},
		{"multiple singles", "1,2,3", "1,2,3", false},
		{"range", "1:5", "1:5", false},
		{"star range", "10:*", "10:*", false},
		{"mixed", "1,3:5,10:*", "1,3:5,10:*", false},
		{"just star", "*", "0", false},
		{"star colon star", "*:*", "0", false},
		{"empty string", "", "", true},
		{"invalid number", "abc", "", true},
		{"zero value", "0", "", true},
		{"negative-like", "-1", "", true},
		{"trailing comma produces empty range", "1,", "", true},
		{"leading comma produces empty range", ",1", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ss, err := ParseSeqSet(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseSeqSet(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if err != nil {
				return
			}
			got := ss.String()
			if got != tt.wantStr {
				t.Errorf("ParseSeqSet(%q).String() = %q, want %q", tt.input, got, tt.wantStr)
			}
		})
	}
}

func TestSeqSet_Contains(t *testing.T) {
	tests := []struct {
		name  string
		input string
		num   uint32
		want  bool
	}{
		{"single hit", "5", 5, true},
		{"single miss", "5", 6, false},
		{"range hit", "1:10", 5, true},
		{"range miss below", "5:10", 4, false},
		{"range miss above", "5:10", 11, false},
		{"multi range first", "1:3,7:9", 2, true},
		{"multi range second", "1:3,7:9", 8, true},
		{"multi range gap", "1:3,7:9", 5, false},
		{"star range", "10:*", 100, true},
		{"star range miss", "10:*", 9, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ss, err := ParseSeqSet(tt.input)
			if err != nil {
				t.Fatalf("ParseSeqSet(%q) unexpected error: %v", tt.input, err)
			}
			got := ss.Contains(tt.num)
			if got != tt.want {
				t.Errorf("SeqSet(%q).Contains(%d) = %v, want %v", tt.input, tt.num, got, tt.want)
			}
		})
	}
}

func TestSeqSet_Dynamic(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"no star", "1:5", false},
		{"has star", "1:*", true},
		{"just star", "*", true},
		{"star in middle", "1:3,5:*,10:20", true},
		{"all static", "1,2,3", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ss, err := ParseSeqSet(tt.input)
			if err != nil {
				t.Fatalf("ParseSeqSet(%q) unexpected error: %v", tt.input, err)
			}
			got := ss.Dynamic()
			if got != tt.want {
				t.Errorf("SeqSet(%q).Dynamic() = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestSeqSet_AddNum(t *testing.T) {
	ss := &SeqSet{}
	if !ss.IsEmpty() {
		t.Fatal("new SeqSet should be empty")
	}
	ss.AddNum(1, 5, 10)
	if ss.IsEmpty() {
		t.Fatal("SeqSet should not be empty after AddNum")
	}
	want := "1,5,10"
	if got := ss.String(); got != want {
		t.Errorf("SeqSet.String() = %q, want %q", got, want)
	}
	if !ss.Contains(1) {
		t.Error("SeqSet should contain 1")
	}
	if !ss.Contains(5) {
		t.Error("SeqSet should contain 5")
	}
	if ss.Contains(3) {
		t.Error("SeqSet should not contain 3")
	}
}

func TestSeqSet_AddRange(t *testing.T) {
	ss := &SeqSet{}
	ss.AddRange(1, 5)
	ss.AddRange(10, 20)
	want := "1:5,10:20"
	if got := ss.String(); got != want {
		t.Errorf("SeqSet.String() = %q, want %q", got, want)
	}
	if !ss.Contains(3) {
		t.Error("should contain 3")
	}
	if !ss.Contains(15) {
		t.Error("should contain 15")
	}
	if ss.Contains(7) {
		t.Error("should not contain 7")
	}
}

func TestSeqSet_Ranges(t *testing.T) {
	ss, err := ParseSeqSet("1:3,5")
	if err != nil {
		t.Fatal(err)
	}
	ranges := ss.Ranges()
	if len(ranges) != 2 {
		t.Fatalf("expected 2 ranges, got %d", len(ranges))
	}
	if ranges[0].Start != 1 || ranges[0].Stop != 3 {
		t.Errorf("range[0] = %+v, want {1, 3}", ranges[0])
	}
	if ranges[1].Start != 5 || ranges[1].Stop != 5 {
		t.Errorf("range[1] = %+v, want {5, 5}", ranges[1])
	}
}

func TestSeqSet_IsEmpty(t *testing.T) {
	ss := &SeqSet{}
	if !ss.IsEmpty() {
		t.Error("new SeqSet should be empty")
	}
	ss.AddNum(1)
	if ss.IsEmpty() {
		t.Error("SeqSet with element should not be empty")
	}
}

func TestSeqSet_EmptyString(t *testing.T) {
	ss := &SeqSet{}
	if got := ss.String(); got != "" {
		t.Errorf("empty SeqSet.String() = %q, want %q", got, "")
	}
}

// --- ParseUIDSet tests ---

func TestParseUIDSet(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantStr string
		wantErr bool
	}{
		{"single uid", "42", "42", false},
		{"range", "1:100", "1:100", false},
		{"star", "1:*", "1:*", false},
		{"complex", "1,5:10,20:*", "1,5:10,20:*", false},
		{"empty", "", "", true},
		{"zero", "0", "", true},
		{"invalid", "xyz", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			us, err := ParseUIDSet(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseUIDSet(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if err != nil {
				return
			}
			got := us.String()
			if got != tt.wantStr {
				t.Errorf("ParseUIDSet(%q).String() = %q, want %q", tt.input, got, tt.wantStr)
			}
		})
	}
}

func TestUIDSet_Contains(t *testing.T) {
	tests := []struct {
		name  string
		input string
		uid   UID
		want  bool
	}{
		{"hit", "1:10", 5, true},
		{"miss", "1:10", 11, false},
		{"star hit", "100:*", 200, true},
		{"star miss", "100:*", 99, false},
		{"multi hit", "1:5,10:15", 12, true},
		{"multi miss", "1:5,10:15", 7, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			us, err := ParseUIDSet(tt.input)
			if err != nil {
				t.Fatalf("ParseUIDSet(%q) unexpected error: %v", tt.input, err)
			}
			got := us.Contains(tt.uid)
			if got != tt.want {
				t.Errorf("UIDSet(%q).Contains(%d) = %v, want %v", tt.input, tt.uid, got, tt.want)
			}
		})
	}
}

func TestUIDSet_Dynamic(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"static", "1:10", false},
		{"dynamic", "1:*", true},
		{"just star", "*", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			us, err := ParseUIDSet(tt.input)
			if err != nil {
				t.Fatalf("ParseUIDSet(%q) unexpected error: %v", tt.input, err)
			}
			got := us.Dynamic()
			if got != tt.want {
				t.Errorf("UIDSet(%q).Dynamic() = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestUIDSet_AddNum(t *testing.T) {
	us := &UIDSet{}
	if !us.IsEmpty() {
		t.Fatal("new UIDSet should be empty")
	}
	us.AddNum(10, 20, 30)
	if us.IsEmpty() {
		t.Fatal("UIDSet should not be empty after AddNum")
	}
	want := "10,20,30"
	if got := us.String(); got != want {
		t.Errorf("UIDSet.String() = %q, want %q", got, want)
	}
	if !us.Contains(10) {
		t.Error("should contain 10")
	}
	if !us.Contains(20) {
		t.Error("should contain 20")
	}
	if us.Contains(15) {
		t.Error("should not contain 15")
	}
}

func TestUIDSet_AddRange(t *testing.T) {
	us := &UIDSet{}
	us.AddRange(1, 50)
	us.AddRange(100, 0) // 100:*
	want := "1:50,100:*"
	if got := us.String(); got != want {
		t.Errorf("UIDSet.String() = %q, want %q", got, want)
	}
	if !us.Contains(25) {
		t.Error("should contain 25")
	}
	if !us.Contains(500) {
		t.Error("should contain 500 (star range)")
	}
	if us.Contains(75) {
		t.Error("should not contain 75")
	}
}

func TestUIDSet_Ranges(t *testing.T) {
	us, err := ParseUIDSet("10:20,30")
	if err != nil {
		t.Fatal(err)
	}
	ranges := us.Ranges()
	if len(ranges) != 2 {
		t.Fatalf("expected 2 ranges, got %d", len(ranges))
	}
	if ranges[0].Start != 10 || ranges[0].Stop != 20 {
		t.Errorf("range[0] = %+v, want {10, 20}", ranges[0])
	}
	if ranges[1].Start != 30 || ranges[1].Stop != 30 {
		t.Errorf("range[1] = %+v, want {30, 30}", ranges[1])
	}
}

func TestUIDSet_IsEmpty(t *testing.T) {
	us := &UIDSet{}
	if !us.IsEmpty() {
		t.Error("new UIDSet should be empty")
	}
	us.AddNum(1)
	if us.IsEmpty() {
		t.Error("UIDSet with element should not be empty")
	}
}

func TestUIDSet_EmptyString(t *testing.T) {
	us := &UIDSet{}
	if got := us.String(); got != "" {
		t.Errorf("empty UIDSet.String() = %q, want %q", got, "")
	}
}

// --- NumSet interface tests ---

func TestSeqSetImplementsNumSet(t *testing.T) {
	var _ NumSet = &SeqSet{}
}

func TestUIDSetImplementsNumSet(t *testing.T) {
	var _ NumSet = &UIDSet{}
}

// --- Edge cases ---

func TestParseSeqSet_WhitespaceInParts(t *testing.T) {
	// The parser trims spaces on comma-separated parts.
	// However, spaces around ':' inside a range part are NOT trimmed,
	// so "2 : 5" should fail because parseSeqNum gets "2 " which is invalid.
	_, err := ParseSeqSet("1 , 2 : 5")
	if err == nil {
		t.Error("expected error for whitespace around ':', got nil")
	}

	// But spaces only around commas should work after trim
	ss, err := ParseSeqSet(" 1 , 5 , 10 ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ss.Contains(1) {
		t.Error("should contain 1")
	}
	if !ss.Contains(5) {
		t.Error("should contain 5")
	}
	if !ss.Contains(10) {
		t.Error("should contain 10")
	}
}

func TestParseSeqSet_SingleStar(t *testing.T) {
	ss, err := ParseSeqSet("*")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// "*" is parsed as {Start: 0, Stop: 0}, which String() renders as "0"
	// but it should be Dynamic
	if !ss.Dynamic() {
		t.Error("star set should be dynamic")
	}
}
