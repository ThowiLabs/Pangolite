package app

import "embed"

//go:embed assets/sb-admin-pro/styles.css assets/sb-admin-pro/scripts.js
var assetsFS embed.FS
