package app

import "embed"

//go:embed assets/app/styles.css assets/app/scripts.js assets/app/logo-mark.png assets/app/favicon.ico
var assetsFS embed.FS
