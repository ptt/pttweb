package richcontent

import (
	"testing"
)

func TestUrlMatches(t *testing.T) {
	check(t, "Xhttp://example.com")
	check(t, "http://example.com/安安", "http://example.com/")
	check(t, "http://example.com", "http://example.com")
	check(t, "http://example.com/index?y=1#yea", "http://example.com/index?y=1#yea")
	check(t, "http://foo.com/? http://example.com", "http://foo.com/?", "http://example.com")
	check(t, "[http://example.com/[({hmm})]", "http://example.com/[({hmm})]")
	check(t, "(http://example.com/(Hi))", "http://example.com/(Hi))")
}

func check(t *testing.T, input string, truths ...string) {
	outputs := FindAllUrls([]byte(input))
	for i, truth := range truths {
		if i >= len(outputs) {
			t.Error("Not matched: No output. Truth:", truth)
		} else if string(outputs[i]) != truth {
			t.Error("Not matched: Output:", string(outputs[i]), "Truth:", truth)
		}
	}
	for i := len(truths); i < len(outputs); i++ {
		t.Error("Too many outputs:", string(outputs[i]))
	}

	indices := FindAllUrlsIndex([]byte(input))
	for i, truth := range truths {
		if i >= len(indices) {
			t.Error("Not matched: No output. Truth:", truth)
		} else if output := string(input[indices[i][0]:indices[i][1]]); output != truth {
			t.Error("Not matched: Output:", output, "Truth:", truth)
		}
	}
	for i := len(truths); i < len(indices); i++ {
		output := string(input[indices[i][0]:indices[i][1]])
		t.Error("Too many outputs:", output)
	}
}
