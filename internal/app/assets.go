package app

import "embed"

//go:embed assets/app/*
var assetsFS embed.FS

//go:embed templates/*/*.html
var templatesFS embed.FS
