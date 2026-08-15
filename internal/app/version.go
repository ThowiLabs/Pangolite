package app

import "strings"

// Version y VersionCode identifican el mismo build para servidor y pangolite-client.
// Los releases pueden sobrescribir ambos valores con -ldflags -X.
var Version = "0.27"
var VersionCode = "27"

func NormalizedVersion() string {
	v := strings.TrimSpace(Version)
	if v == "" {
		return "dev"
	}
	return strings.TrimPrefix(v, "v")
}

func NormalizedVersionCode() string {
	v := strings.TrimSpace(VersionCode)
	if v == "" {
		return "0"
	}
	return v
}

func VersionSummary(product string) string {
	product = strings.TrimSpace(product)
	if product == "" {
		product = "pangolite"
	}
	return product + " " + NormalizedVersion() + " (code " + NormalizedVersionCode() + ")"
}
