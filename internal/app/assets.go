package app

import "embed"

//go:embed assets/app/styles.css assets/app/scripts.js
var assetsFS embed.FS
