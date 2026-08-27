package app

import "testing"

func validServeConfigForTest() Config {
	return Config{
		Addr:                   "127.0.0.1:2424",
		DataPath:               "pangolite.db",
		InitialAdminUser:       "admin",
		SessionDays:            30,
		TrustedProxyCIDRs:      "127.0.0.1/32,::1/128",
		AdminAccessMode:        "learn",
		AgentHTTPConcurrency:   4,
		AgentStreamConcurrency: 16,
	}
}

func TestAdminAccessModeOnlyAllowsLearnOrOff(t *testing.T) {
	config := validServeConfigForTest()
	if err := config.ValidateForServe(); err != nil {
		t.Fatalf("learn debe ser valido: %v", err)
	}

	config.AdminAccessMode = "off"
	if err := config.ValidateForServe(); err != nil {
		t.Fatalf("off debe ser valido: %v", err)
	}

	config.AdminAccessMode = "allowlist"
	if err := config.ValidateForServe(); err == nil {
		t.Fatal("allowlist ya no debe ser un modo valido")
	}

	config.AdminAccessMode = ""
	if err := config.ValidateForServe(); err == nil {
		t.Fatal("un modo vacio no debe desactivar silenciosamente la proteccion")
	}
}
