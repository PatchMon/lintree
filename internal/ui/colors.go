package ui

import (
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
)

type FileCategory int

const (
	CatDirectory FileCategory = iota
	CatCode
	CatWeb
	CatDocument
	CatImage
	CatVideo
	CatAudio
	CatArchive
	CatData
	CatBinary
	CatOther
)

var categoryColors = map[FileCategory]tcell.Color{
	CatDirectory: tcell.NewRGBColor(86, 182, 194),   // Cyan
	CatCode:      tcell.NewRGBColor(91, 141, 239),    // Blue
	CatWeb:       tcell.NewRGBColor(86, 214, 194),    // Teal
	CatDocument:  tcell.NewRGBColor(152, 195, 121),   // Green
	CatImage:     tcell.NewRGBColor(229, 192, 123),   // Yellow
	CatVideo:     tcell.NewRGBColor(198, 120, 221),   // Magenta
	CatAudio:     tcell.NewRGBColor(224, 108, 159),   // Pink
	CatArchive:   tcell.NewRGBColor(224, 108, 117),   // Red
	CatData:      tcell.NewRGBColor(209, 154, 102),   // Orange
	CatBinary:    tcell.NewRGBColor(190, 80, 70),     // Dark Red
	CatOther:     tcell.NewRGBColor(92, 99, 112),     // Gray
}

var categoryLabels = map[FileCategory]string{
	CatDirectory: "Directory",
	CatCode:      "Source Code",
	CatWeb:       "Web",
	CatDocument:  "Document",
	CatImage:     "Image",
	CatVideo:     "Video",
	CatAudio:     "Audio",
	CatArchive:   "Archive",
	CatData:      "Data",
	CatBinary:    "Binary",
	CatOther:     "Other",
}

var extCategories = map[string]FileCategory{
	// Code
	".go": CatCode, ".rs": CatCode, ".py": CatCode, ".js": CatCode,
	".ts": CatCode, ".c": CatCode, ".cpp": CatCode, ".h": CatCode,
	".java": CatCode, ".rb": CatCode, ".php": CatCode, ".swift": CatCode,
	".kt": CatCode, ".scala": CatCode, ".lua": CatCode, ".sh": CatCode,
	".bash": CatCode, ".zsh": CatCode, ".fish": CatCode, ".vim": CatCode,
	".el": CatCode, ".clj": CatCode, ".ex": CatCode, ".erl": CatCode,
	".hs": CatCode, ".ml": CatCode, ".r": CatCode, ".m": CatCode,
	".cs": CatCode, ".dart": CatCode, ".zig": CatCode, ".nim": CatCode,
	// Web
	".html": CatWeb, ".css": CatWeb, ".scss": CatWeb, ".sass": CatWeb,
	".less": CatWeb, ".vue": CatWeb, ".svelte": CatWeb, ".jsx": CatWeb,
	".tsx": CatWeb, ".wasm": CatWeb,
	// Documents
	".pdf": CatDocument, ".doc": CatDocument, ".docx": CatDocument,
	".txt": CatDocument, ".md": CatDocument, ".rst": CatDocument,
	".tex": CatDocument, ".rtf": CatDocument, ".odt": CatDocument,
	".epub": CatDocument, ".pages": CatDocument,
	// Images
	".png": CatImage, ".jpg": CatImage, ".jpeg": CatImage, ".gif": CatImage,
	".webp": CatImage, ".svg": CatImage, ".bmp": CatImage, ".ico": CatImage,
	".tiff": CatImage, ".psd": CatImage, ".ai": CatImage, ".raw": CatImage,
	".heic": CatImage,
	// Video
	".mp4": CatVideo, ".mkv": CatVideo, ".avi": CatVideo, ".mov": CatVideo,
	".wmv": CatVideo, ".flv": CatVideo, ".webm": CatVideo, ".m4v": CatVideo,
	".3gp": CatVideo,
	// Audio
	".mp3": CatAudio, ".flac": CatAudio, ".wav": CatAudio, ".ogg": CatAudio,
	".aac": CatAudio, ".wma": CatAudio, ".m4a": CatAudio, ".opus": CatAudio,
	// Archives
	".zip": CatArchive, ".tar": CatArchive, ".gz": CatArchive, ".bz2": CatArchive,
	".xz": CatArchive, ".7z": CatArchive, ".rar": CatArchive, ".zst": CatArchive,
	".lz4": CatArchive, ".deb": CatArchive, ".rpm": CatArchive, ".snap": CatArchive,
	".AppImage": CatArchive, ".dmg": CatArchive, ".iso": CatArchive,
	// Data
	".json": CatData, ".xml": CatData, ".csv": CatData, ".sql": CatData,
	".db": CatData, ".sqlite": CatData, ".yaml": CatData, ".yml": CatData,
	".toml": CatData, ".ini": CatData, ".conf": CatData, ".cfg": CatData,
	".parquet": CatData, ".avro": CatData, ".proto": CatData,
	// Binaries
	".exe": CatBinary, ".dll": CatBinary, ".so": CatBinary, ".dylib": CatBinary,
	".o": CatBinary, ".a": CatBinary, ".lib": CatBinary, ".bin": CatBinary,
	".class": CatBinary, ".pyc": CatBinary,
}

// CategoryFor returns the file category for a filename.
func CategoryFor(name string, isDir bool) FileCategory {
	if isDir {
		return CatDirectory
	}
	ext := strings.ToLower(filepath.Ext(name))
	if cat, ok := extCategories[ext]; ok {
		return cat
	}
	return CatOther
}

// ColorFor returns the tcell color for a file.
func ColorFor(name string, isDir bool) tcell.Color {
	cat := CategoryFor(name, isDir)
	return categoryColors[cat]
}

// CategoryLabel returns the human-readable label for a category.
func CategoryLabel(name string, isDir bool) string {
	cat := CategoryFor(name, isDir)
	return categoryLabels[cat]
}

// DimColor reduces the brightness of a color for less prominent items.
func DimColor(c tcell.Color, factor float64) tcell.Color {
	r, g, b := c.RGB()
	return tcell.NewRGBColor(
		int32(float64(r)*factor),
		int32(float64(g)*factor),
		int32(float64(b)*factor),
	)
}

// ContrastFg returns white or black depending on background brightness.
func ContrastFg(bg tcell.Color) tcell.Color {
	r, g, b := bg.RGB()
	lum := 0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)
	if lum > 128 {
		return tcell.NewRGBColor(20, 20, 20)
	}
	return tcell.NewRGBColor(240, 240, 240)
}
