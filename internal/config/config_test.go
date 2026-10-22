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
			want: Config{
				Addr:               ":8080",
				Env:                "dev",
				LogLevel:           "info",
				DBPath:             "data/leaderboard.db",
				StravaClientID:     "",
				StravaClientSecret: "",
				BaseURL:            "http://localhost:8080",
			},
		},
		{
			name: "Addr",
			env:  map[string]string{"ADDR": ":8000"},
			want: Config{
				Addr:               ":8000",
				Env:                "dev",
				LogLevel:           "info",
				DBPath:             "data/leaderboard.db",
				StravaClientID:     "",
				StravaClientSecret: "",
				BaseURL:            "http://localhost:8080",
			},
		},
		{
			name: "Env",
			env:  map[string]string{"ENV": "prod"},
			want: Config{
				Addr:               ":8080",
				Env:                "prod",
				LogLevel:           "info",
				DBPath:             "data/leaderboard.db",
				StravaClientID:     "",
				StravaClientSecret: "",
				BaseURL:            "http://localhost:8080",
			},
		},
		{
			name: "StravaClientID",
			env: map[string]string{
				"STRAVA_CLIENT_ID": "strava-client-id",
			},
			want: Config{
				Addr:               ":8080",
				Env:                "dev",
				LogLevel:           "info",
				DBPath:             "data/leaderboard.db",
				StravaClientID:     "strava-client-id",
				StravaClientSecret: "",
				BaseURL:            "http://localhost:8080",
			},
		},
		{
			name: "StravaClientSecret",
			env:  map[string]string{"STRAVA_CLIENT_SECRET": "strava-client-secret"},
			want: Config{
				Addr:               ":8080",
				Env:                "dev",
				LogLevel:           "info",
				DBPath:             "data/leaderboard.db",
				StravaClientID:     "",
				StravaClientSecret: "strava-client-secret",
				BaseURL:            "http://localhost:8080",
			},
		},
		{
			name: "BaseURL",
			env:  map[string]string{"BASE_URL": "http://leaderboard.com"},
			want: Config{
				Addr:     ":8080",
				Env:      "dev",
				LogLevel: "info",
				DBPath:   "data/leaderboard.db",
				BaseURL:  "http://leaderboard.com",
			},
		},
		{
			name: "LogLevel",
			env:  map[string]string{"LOG_LEVEL": "debug"},
			want: Config{
				Addr:     ":8080",
				Env:      "dev",
				LogLevel: "debug",
				DBPath:   "data/leaderboard.db",
				BaseURL:  "http://localhost:8080",
			},
		},
		{
			name: "all at once",
			env: map[string]string{
				"ADDR":                 ":8000",
				"ENV":                  "prod",
				"LOG_LEVEL":            "debug",
				"DB_PATH":              "data/somedb.db",
				"STRAVA_CLIENT_ID":     "strava-client-id",
				"STRAVA_CLIENT_SECRET": "strava-client-secret",
				"BASE_URL":             "http://leaderboard.com",
			},
			want: Config{
				Addr:               ":8000",
				Env:                "prod",
				LogLevel:           "debug",
				DBPath:             "data/somedb.db",
				StravaClientID:     "strava-client-id",
				StravaClientSecret: "strava-client-secret",
				BaseURL:            "http://leaderboard.com",
			},
		},
		{
			name: "StravaVerifyToken",
			env:  map[string]string{"STRAVA_VERIFY_TOKEN": "s3cret"},
			want: Config{
				Addr:              ":8080",
				Env:               "dev",
				LogLevel:          "info",
				DBPath:            "data/leaderboard.db",
				BaseURL:           "http://localhost:8080",
				StravaVerifyToken: "s3cret",
			},
		},
		{
			name: "StravaSubscriptionID",
			env:  map[string]string{"STRAVA_SUBSCRIPTION_ID": "120475"},
			want: Config{
				Addr:                 ":8080",
				Env:                  "dev",
				LogLevel:             "info",
				DBPath:               "data/leaderboard.db",
				BaseURL:              "http://localhost:8080",
				StravaSubscriptionID: 120475,
			},
		},
		{
			// Unset means "no subscription yet": 0, not an error.
			name: "StravaSubscriptionID unset",
			env:  map[string]string{"STRAVA_SUBSCRIPTION_ID": ""},
			want: Config{
				Addr:     ":8080",
				Env:      "dev",
				LogLevel: "info",
				DBPath:   "data/leaderboard.db",
				BaseURL:  "http://localhost:8080",
			},
		},
		{
			name:    "StravaSubscriptionID not a number",
			env:     map[string]string{"STRAVA_SUBSCRIPTION_ID": "abc"},
			want:    Config{},
			wantErr: true,
		},
		{
			name:    "StravaSubscriptionID negative",
			env:     map[string]string{"STRAVA_SUBSCRIPTION_ID": "-5"},
			want:    Config{},
			wantErr: true,
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
