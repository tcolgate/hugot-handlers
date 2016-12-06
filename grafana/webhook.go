package grafana

import (
	"net/http"
	"strings"
)

func (p *grafH) webHook(w http.ResponseWriter, r *http.Request) {
	pth := strings.TrimRight(p.URL().Path, "/")
	http.StripPrefix(pth, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p.hmux.ServeHTTP(w, r)
	})).ServeHTTP(w, r)
}
