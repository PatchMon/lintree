package ui

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestCategoryFor(t *testing.T) {
	tests := []struct {
		name  string
		file  string
		isDir bool
		want  FileCategory
	}{
		// Directories
		{"directory", "src", true, CatDirectory},
		{"directory with ext", "images.bak", true, CatDirectory},

		// Code
		{"go file", "main.go", false, CatCode},
		{"python file", "script.py", false, CatCode},
		{"rust file", "lib.rs", false, CatCode},
		{"javascript", "app.js", false, CatCode},
		{"shell script", "run.sh", false, CatCode},

		// Web
		{"html file", "index.html", false, CatWeb},
		{"css file", "style.css", false, CatWeb},
		{"tsx file", "App.tsx", false, CatWeb},

		// Documents
		{"pdf", "doc.pdf", false, CatDocument},
		{"markdown", "README.md", false, CatDocument},
		{"text", "notes.txt", false, CatDocument},

		// Images
		{"png", "logo.png", false, CatImage},
		{"jpeg", "photo.jpeg", false, CatImage},
		{"svg", "icon.svg", false, CatImage},

		// Video
		{"mp4", "video.mp4", false, CatVideo},
		{"mkv", "movie.mkv", false, CatVideo},

		// Audio
		{"mp3", "song.mp3", false, CatAudio},
		{"flac", "track.flac", false, CatAudio},

		// Archive
		{"zip", "data.zip", false, CatArchive},
		{"tar.gz", "backup.gz", false, CatArchive},

		// Data
		{"json", "config.json", false, CatData},
		{"yaml", "values.yaml", false, CatData},
		{"sql", "dump.sql", false, CatData},

		// Binary
		{"exe", "app.exe", false, CatBinary},
		{"so", "lib.so", false, CatBinary},

		// Other / unknown
		{"unknown ext", "file.xyz", false, CatOther},
		{"no extension", "Makefile", false, CatOther},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CategoryFor(tt.file, tt.isDir)
			if got != tt.want {
				t.Errorf("CategoryFor(%q, %v) = %d, want %d", tt.file, tt.isDir, got, tt.want)
			}
		})
	}
}

func TestCategoryForCaseInsensitive(t *testing.T) {
	// Extension matching should be case-insensitive
	got := CategoryFor("IMAGE.PNG", false)
	if got != CatImage {
		t.Errorf("CategoryFor(\"IMAGE.PNG\", false) = %d, want %d (CatImage)", got, CatImage)
	}
}

func TestContrastFg(t *testing.T) {
	tests := []struct {
		name   string
		bg     tcell.Color
		wantR  int32 // expected R component of result
		bright bool  // true = expect light fg, false = expect dark fg
	}{
		{
			name:   "dark background returns light foreground",
			bg:     tcell.NewRGBColor(20, 20, 20),
			bright: true,
		},
		{
			name:   "light background returns dark foreground",
			bg:     tcell.NewRGBColor(230, 230, 230),
			bright: false,
		},
		{
			name:   "pure black returns light foreground",
			bg:     tcell.NewRGBColor(0, 0, 0),
			bright: true,
		},
		{
			name:   "pure white returns dark foreground",
			bg:     tcell.NewRGBColor(255, 255, 255),
			bright: false,
		},
		{
			name:   "mid-dark returns light foreground",
			bg:     tcell.NewRGBColor(50, 50, 50),
			bright: true,
		},
		{
			name:   "mid-light returns dark foreground",
			bg:     tcell.NewRGBColor(200, 200, 200),
			bright: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fg := ContrastFg(tt.bg)
			r, _, _ := fg.RGB()
			if tt.bright && r < 200 {
				t.Errorf("expected bright fg for dark bg, got R=%d", r)
			}
			if !tt.bright && r > 50 {
				t.Errorf("expected dark fg for light bg, got R=%d", r)
			}
		})
	}
}
