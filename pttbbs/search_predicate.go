package pttbbs

import apipb "github.com/ptt/pttweb/proto/api"

type SearchPredicate interface {
	toSearchFilter() *apipb.SearchFilter
}

func WithTitle(title string) SearchPredicate {
	return &searchPredicate{&apipb.SearchFilter{
		Type:       apipb.SearchFilter_TYPE_TITLE,
		StringData: title,
	}}
}

func WithExactTitle(title string) SearchPredicate {
	return &searchPredicate{&apipb.SearchFilter{
		Type:       apipb.SearchFilter_TYPE_EXACT_TITLE,
		StringData: title,
	}}
}

func WithAuthor(author string) SearchPredicate {
	return &searchPredicate{&apipb.SearchFilter{
		Type:       apipb.SearchFilter_TYPE_AUTHOR,
		StringData: author,
	}}
}

func WithRecommend(n int) SearchPredicate {
	if n < -100 {
		n = -100
	}
	if n > 100 {
		n = 100
	}
	return &searchPredicate{&apipb.SearchFilter{
		Type:       apipb.SearchFilter_TYPE_RECOMMEND,
		NumberData: int64(n),
	}}
}

type searchPredicate struct {
	*apipb.SearchFilter
}

func (p *searchPredicate) toSearchFilter() *apipb.SearchFilter {
	return p.SearchFilter
}
