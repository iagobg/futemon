package app

import "embed"

//go:embed templates/*.html static/*.js static/*.css
var embeddedFiles embed.FS

func mustReadEmbeddedText(path string) string {
	data, err := embeddedFiles.ReadFile(path)
	if err != nil {
		panic(err)
	}
	return string(data)
}
