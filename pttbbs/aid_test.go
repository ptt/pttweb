package pttbbs

import (
	"testing"
)

func TestAidConvert(t *testing.T) {
	checkMatch(t, "", "M.0.A.000")
	checkMatch(t, "1HNXB7zo", "M.1365119687.A.F72")
	checkMatch(t, "53HvTzpa", "G.1128765309.A.CE4")
	checkMatch(t, "1KLschD-", "M.1415014827.A.37E")
	checkMatch(t, "1KL8iv_2", "M.1414826809.A.FC2")
}

func checkMatch(t *testing.T, aidc, fn string) {
	aid, err := ParseAid(aidc)
	if err != nil {
		t.Error(err)
	}
	if tfn := aid.Filename(); tfn != fn {
		t.Error(aidc, "expected", fn, "got", tfn)
	}
	if str := aid.String(); str != aidc {
		t.Error(aidc, "convert back", "got", str)
	}
}

func TestAidTooLong(t *testing.T) {
	_, err := ParseAid("1234567890a")
	if err == nil {
		t.Fail()
	}
}

func TestAidInvalid(t *testing.T) {
	_, err := ParseAid("!@#$%^")
	if err == nil {
		t.Fail()
	}
}
