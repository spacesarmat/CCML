package settings

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spacesarmat/CCML/internal/model"
)

const fileName = "settings.json"

type Service struct {
	path string
}

func New(appDir string) *Service {
	return &Service{path: filepath.Join(appDir, fileName)}
}

func (s *Service) Load() (model.MetadataSettings, error) {
	defaults := DefaultsFromEnvironment()
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return defaults, nil
		}
		return model.MetadataSettings{}, fmt.Errorf("read settings: %w", err)
	}

	current := defaults
	if err := json.Unmarshal(data, &current); err != nil {
		_ = os.Rename(s.path, s.path+".invalid")
		return defaults, nil
	}
	current.Normalize()
	return current, nil
}

func (s *Service) Save(settings model.MetadataSettings) error {
	settings.Normalize()
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("encode settings: %w", err)
	}
	data = append(data, '\n')

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create settings directory: %w", err)
	}

	temp, err := os.CreateTemp(dir, "settings-*.tmp")
	if err != nil {
		return fmt.Errorf("create settings temp file: %w", err)
	}
	tempPath := temp.Name()
	cleanup := func() { _ = os.Remove(tempPath) }
	defer cleanup()

	if err := temp.Chmod(0o600); err != nil {
		_ = temp.Close()
		return fmt.Errorf("protect settings temp file: %w", err)
	}
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return fmt.Errorf("write settings temp file: %w", err)
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return fmt.Errorf("sync settings temp file: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close settings temp file: %w", err)
	}
	if err := os.Rename(tempPath, s.path); err != nil {
		return fmt.Errorf("replace settings file: %w", err)
	}
	_ = os.Chmod(s.path, 0o600)
	return nil
}

func DefaultsFromEnvironment() model.MetadataSettings {
	cfg := model.MetadataSettings{
		MusicBrainzEnabled:  true,
		TheAudioDBEnabled:   true,
		DeezerEnabled:       true,
		ITunesEnabled:       true,
		DiscogsEnabled:      strings.TrimSpace(os.Getenv("DISCOGS_TOKEN")) != "",
		SpotifyEnabled:      strings.TrimSpace(os.Getenv("SPOTIFY_ACCESS_TOKEN")) != "" || (strings.TrimSpace(os.Getenv("SPOTIFY_CLIENT_ID")) != "" && strings.TrimSpace(os.Getenv("SPOTIFY_CLIENT_SECRET")) != ""),
		AppleMusicEnabled:   strings.TrimSpace(os.Getenv("APPLE_MUSIC_DEVELOPER_TOKEN")) != "",
		YouTubeEnabled:      strings.TrimSpace(os.Getenv("YOUTUBE_API_KEY")) != "",
		SoundCloudEnabled:   strings.TrimSpace(os.Getenv("SOUNDCLOUD_ACCESS_TOKEN")) != "",
		YandexMusicEnabled:  strings.TrimSpace(os.Getenv("YANDEX_MUSIC_TOKEN")) != "",
		TraxsourceEnabled:   false,
		MuzvizorEnabled:     false,
		RemixPoolEnabled:    false,
		BananaStreetEnabled: false,

		TraxsourceAPIKey: strings.TrimSpace(os.Getenv("TRAXSOURCE_API_KEY")),

		TheAudioDBAPIKey:         strings.TrimSpace(os.Getenv("THEAUDIODB_API_KEY")),
		ITunesCountry:            strings.TrimSpace(os.Getenv("ITUNES_COUNTRY")),
		DiscogsToken:             strings.TrimSpace(os.Getenv("DISCOGS_TOKEN")),
		SpotifyAccessToken:       strings.TrimSpace(os.Getenv("SPOTIFY_ACCESS_TOKEN")),
		SpotifyClientID:          strings.TrimSpace(os.Getenv("SPOTIFY_CLIENT_ID")),
		SpotifyClientSecret:      strings.TrimSpace(os.Getenv("SPOTIFY_CLIENT_SECRET")),
		SpotifyMarket:            strings.TrimSpace(os.Getenv("SPOTIFY_MARKET")),
		AppleMusicDeveloperToken: strings.TrimSpace(os.Getenv("APPLE_MUSIC_DEVELOPER_TOKEN")),
		AppleMusicStorefront:     strings.TrimSpace(os.Getenv("APPLE_MUSIC_STOREFRONT")),
		YouTubeAPIKey:            strings.TrimSpace(os.Getenv("YOUTUBE_API_KEY")),
		SoundCloudAccessToken:    strings.TrimSpace(os.Getenv("SOUNDCLOUD_ACCESS_TOKEN")),
		YandexMusicToken:         strings.TrimSpace(os.Getenv("YANDEX_MUSIC_TOKEN")),
		YandexMusicLanguage:      strings.TrimSpace(os.Getenv("YANDEX_MUSIC_LANGUAGE")),
	}
	cfg.Normalize()
	return cfg
}
