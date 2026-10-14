package api

import (
	"embed"
	"io/fs"
)

// O build de produção (Dockerfile) sobrescreve internal/api/dist/ com o
// dist/ real do Vite antes do go build; no repo vive só um placeholder.
//
//go:embed all:dist
var distFS embed.FS

// EmbeddedDist devolve o frontend embarcado no binário (D-04).
func EmbeddedDist() fs.FS {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic(err) // embed estático: só falha por bug de build
	}
	return sub
}
