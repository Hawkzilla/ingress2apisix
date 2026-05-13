package web

import "embed"

//go:embed static/*
var staticFS embed.FS

//go:embed docs/*
var docsFS embed.FS
