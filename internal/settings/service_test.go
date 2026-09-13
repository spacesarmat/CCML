package settings

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/spacesarmat/CCML/internal/model"
)

func TestSaveLoadMetadataSettings(t *testing.T) {
	t.Setenv("THEAUDIODB_API_KEY", "")
	t.Setenv("ITUNES_COUNTRY", "")
	dir := t.TempDir()
	service := New(dir)

	cfg := model.MetadataSettings{
		MusicBrainzEnabled:  true,
		DeezerEnabled:       true,
		DiscogsEnabled:      true,
		DiscogsToken:        " secret ",
		ITunesCountry:       " gb ",
		SpotifyMarket:       " de ",
		YandexMusicEnabled:  true,
		YandexMusicToken:    " yandex-secret ",
		YandexMusicLanguage: " RU ",
		TraxsourceEnabled:   true,
	}
	if err := service.Save(cfg); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := service.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.DiscogsToken != "secret" {
		t.Fatalf("discogs token = %q", got.DiscogsToken)
	}
	if got.ITunesCountry != "GB" {
		t.Fatalf("itunes country = %q", got.ITunesCountry)
	}
	if got.SpotifyMarket != "DE" {
		t.Fatalf("spotify market = %q", got.SpotifyMarket)
	}
	if got.TheAudioDBAPIKey != "123" {
		t.Fatalf("theaudiodb default key = %q", got.TheAudioDBAPIKey)
	}
	if !got.YandexMusicEnabled || got.YandexMusicToken != "yandex-secret" || got.YandexMusicLanguage != "ru" {
		t.Fatalf("yandex settings = %+v", got)
	}
	if !got.TraxsourceEnabled {
		t.Fatal("traxsource setting was not preserved")
	}

	info, err := os.Stat(filepath.Join(dir, fileName))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	// POSIX permission bits are meaningful on macOS/Linux. Windows security is
	// enforced by NTFS ACLs and Go reports synthesized mode bits (commonly 0666),
	// so asserting 0600 there is incorrect even when the file is not broadly shared.
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("settings file permissions are too broad: %o", info.Mode().Perm())
	}
}

func TestMalformedSettingsFallsBackToDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, fileName)
	if err := os.WriteFile(path, []byte("{broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := New(dir).Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !cfg.MusicBrainzEnabled || !cfg.DeezerEnabled || cfg.ITunesCountry == "" {
		t.Fatalf("unexpected fallback config: %+v", cfg)
	}
	if _, err := os.Stat(path + ".invalid"); err != nil {
		t.Fatalf("invalid file was not preserved: %v", err)
	}
}
