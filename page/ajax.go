package page

import (
	"encoding/json"
	"net/http"
)

type ArticlePollResp struct {
	ContentHtml string `json:"contentHtml"`
	PollUrl     string `json:"pollUrl"`
	Success     bool   `json:"success"`
}

func WriteAjaxResp(w http.ResponseWriter, obj interface{}) error {
	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(obj)
}
