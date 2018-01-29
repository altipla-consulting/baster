package proxy

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/julienschmidt/httprouter"
)

func HealthHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintln(w, "baster is ok")
}

func RedirectHandler(w http.ResponseWriter, r *http.Request) {
	u := new(url.URL)
	*u = *r.URL
	u.Scheme = "https"
	u.Host = r.Host
	http.Redirect(w, r, u.String(), http.StatusMovedPermanently)
}
