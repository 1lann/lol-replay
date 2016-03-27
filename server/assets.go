// +build ignore

package main

type staticAsset struct {
	contentType  string
	uncompressed []byte
	gzipped      []byte
}

var staticAssets = make(map[string]staticAsset)

func (s *staticAsset) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.w.Header().Set("Content-Type", s.ContentType)

}

func init() {
	staticAsset["bulma.min.css"] = staticAsset{
		contentType:  "text/css; charset=utf-8",
		uncompressed: `Content goes here`,
	}

	for _, asset := range staticAssets {

	}
}
