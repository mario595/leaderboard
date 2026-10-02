package config

import "testing"

func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		want    Config
		wantErr bool
	}{
		{
			name: "defaults",
			env:  map[string]string{},
			want: Config{Addr: ":8080", Env: "dev", LogLevel: "info"},
		},
		{
			name: "Addr",
			env:  map[string]string{"ADDR": "8000"},
			want: Config{Addr: "8000", Env: "dev", LogLevel: "info"},
		},
		{
			name: "Env",
			env:  map[string]string{"ENV": "prod"},
			want: Config{Addr: ":8080", Env: "prod", LogLevel: "info"},
		},
		{
			name: "LogLevel",
			env:  map[string]string{"LOG_LEVEL": "debug"},
			want: Config{Addr: ":8080", Env: "dev", LogLevel: "debug"},
		},
		{
			name: "all at once",
			env:  map[string]string{"ADDR": ":8000", "ENV": "prod", "LOG_LEVEL": "debug"},
			want: Config{Addr: ":8000", Env: "prod", LogLevel: "debug"},
		},
		{
			name:    "LogLevel error",
			env:     map[string]string{"LOG_LEVEL": "invalid"},
			want:    Config{},
			wantErr: true,
		},
		{
			name:    "Env error",
			env:     map[string]string{"ENV": "invalid"},
			want:    Config{},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			getenv := func(key string) string { return tc.env[key] }
			got, err := Load(getenv)

			if (err != nil) != tc.wantErr {
				t.Fatalf("Load() error = %v, wantErr %v", err, tc.wantErr)
			}

			if got != tc.want {
				t.Errorf("Load() = %+v, want %+v", got, tc.want)
			}
		})
	}
}
