package db

import (
	"embed"
	"io/fs"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Migrations devolve o FS com os arquivos NNNN_nome.sql embarcados no binário.
func Migrations() fs.FS {
	sub, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		panic(err) // embed estático: só falha por bug de build
	}
	return sub
}
