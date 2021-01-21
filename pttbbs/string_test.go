package pttbbs

import "testing"

func TestBrdNameFromAllPostTitle(t *testing.T) {
	for _, test := range []struct {
		desc  string
		title string
		want  string
	}{
		{
			desc:  "normal title with board name",
			title: "測試 (Test)",
			want:  "Test",
		},
		{
			desc:  "empty title with board name",
			title: "(SYSOP)",
			want:  "SYSOP",
		},
		{
			desc:  "normal title but bad board name",
			title: "測試 (SYS%%) ",
		},
		{
			desc:  "bad title",
			title: "(Bad) ",
		},
		{
			desc:  "empty title",
			title: "",
		},
	} {
		t.Run(test.desc, func(t *testing.T) {
			wantOk := test.want != ""

			got, gotOk := BrdNameFromAllPostTitle(test.title)
			if got != test.want || gotOk != wantOk {
				t.Errorf("BrdNameFromAllPostTitle(%q) = (%q, %t); want (%q, %t)",
					test.title, got, gotOk, test.want, wantOk)
			}
		})
	}
}
