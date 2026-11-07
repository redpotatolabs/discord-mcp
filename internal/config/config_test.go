package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestFromEnv(t *testing.T) {
	tests := []struct {
		name        string
		env         map[string]string
		wantErr     bool
		wantAllowed []string
		wantDefault string
		wantRO      bool
	}{
		{name: "missing token", env: map[string]string{}, wantErr: true},
		{name: "token only = unrestricted", env: map[string]string{"DISCORD_TOKEN": "t"}},
		{name: "default guild locks allowlist",
			env:         map[string]string{"DISCORD_TOKEN": "t", "DISCORD_GUILD_ID": "1"},
			wantAllowed: []string{"1"}, wantDefault: "1"},
		{name: "star unlocks",
			env:         map[string]string{"DISCORD_TOKEN": "t", "DISCORD_GUILD_ID": "1", "DISCORD_ALLOWED_GUILD_IDS": "*"},
			wantDefault: "1"},
		{name: "single allowed becomes default",
			env:         map[string]string{"DISCORD_TOKEN": "t", "DISCORD_ALLOWED_GUILD_IDS": " 5 "},
			wantAllowed: []string{"5"}, wantDefault: "5"},
		{name: "list",
			env:         map[string]string{"DISCORD_TOKEN": "t", "DISCORD_ALLOWED_GUILD_IDS": "5,6", "DISCORD_GUILD_ID": "6"},
			wantAllowed: []string{"5", "6"}, wantDefault: "6"},
		{name: "default outside allowlist",
			env:     map[string]string{"DISCORD_TOKEN": "t", "DISCORD_ALLOWED_GUILD_IDS": "5", "DISCORD_GUILD_ID": "6"},
			wantErr: true},
		{name: "read only",
			env:    map[string]string{"DISCORD_TOKEN": "t", "DISCORD_READ_ONLY": "true"},
			wantRO: true},
		{name: "bad read only", env: map[string]string{"DISCORD_TOKEN": "t", "DISCORD_READ_ONLY": "maybe"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := fromEnv(env(tt.env))
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if !reflect.DeepEqual(cfg.AllowedGuilds, tt.wantAllowed) {
				t.Errorf("allowed = %v, want %v", cfg.AllowedGuilds, tt.wantAllowed)
			}
			if cfg.DefaultGuild != tt.wantDefault || cfg.ReadOnly != tt.wantRO {
				t.Errorf("default=%q ro=%v", cfg.DefaultGuild, cfg.ReadOnly)
			}
		})
	}
}

func TestTokenFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "tok")
	if err := os.WriteFile(p, []byte("secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := fromEnv(env(map[string]string{"DISCORD_TOKEN_FILE": p}))
	if err != nil || cfg.Token != "secret" {
		t.Fatalf("token=%q err=%v", cfg.Token, err)
	}
}

func TestLoadEnvFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "env")
	if err := os.WriteFile(p, []byte("DISCORD_TOKEN=fromfile\nDISCORD_GUILD_ID=9\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DISCORD_MCP_ENV_FILE", p)
	t.Setenv("DISCORD_TOKEN", "")
	t.Setenv("DISCORD_GUILD_ID", "")
	_ = os.Unsetenv("DISCORD_TOKEN")
	_ = os.Unsetenv("DISCORD_GUILD_ID")
	cfg, err := Load()
	if err != nil || cfg.Token != "fromfile" || cfg.DefaultGuild != "9" {
		t.Fatalf("cfg=%+v err=%v", cfg, err)
	}
}
