<p align="center">
  <h1 align="center">lintree</h1>
  <p align="center">
    <strong>Terminal disk usage visualizer. Think WinDirStat, but in your terminal.</strong>
  </p>
  <p align="center">
    <a href="https://github.com/PatchMon/lintree/releases"><img src="https://img.shields.io/github/v/release/PatchMon/lintree?style=flat-square&color=00b4d8" alt="Release"></a>
    <a href="https://github.com/PatchMon/lintree/actions"><img src="https://img.shields.io/github/actions/workflow/status/PatchMon/lintree/ci.yml?style=flat-square" alt="CI"></a>
    <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue?style=flat-square" alt="License"></a>
    <a href="https://buymeacoffee.com/iby___"><img src="https://img.shields.io/badge/Buy%20Me%20a%20Coffee-ffdd00?style=flat-square&logo=buy-me-a-coffee&logoColor=black" alt="Buy Me A Coffee"></a>
  </p>
</p>

---

lintree scans your filesystem and renders an interactive treemap right in the terminal. See which files and folders are eating your disk space without leaving the command line.

## Features

- Squarified treemap that fills your terminal with proportional colored blocks
- Color coded file types: code (blue), video (magenta), archives (red), images (yellow), audio (pink), etc.
- Brightness scales with file size so the big stuff jumps out at you
- Drill into folders, go back, explore your whole disk interactively
- Sidebar shows file info, size, type, path, percentage, and top children
- Breadcrumb bar so you always know where you are
- Concurrent filesystem scanner that handles millions of files
- Works on Linux, macOS, and Windows

## Install

### Quick install (recommended)

```bash
curl -fsSL https://raw.githubusercontent.com/PatchMon/lintree/main/install.sh | sh
```

Detects your OS and architecture, downloads the latest release, and installs to `/usr/local/bin`.

### Go install

```bash
go install github.com/PatchMon/lintree@latest
```

### Download binary

Grab the latest binary from [GitHub Releases](https://github.com/PatchMon/lintree/releases).

### Build from source

```bash
git clone https://github.com/PatchMon/lintree.git
cd lintree
make build
```

## Usage

```bash
lintree              # Scan / (entire root filesystem)
lintree /home        # Scan a specific directory
lintree .            # Scan current directory
lintree ~/Downloads  # Scan your downloads
```

### Flags

```
lintree [path]       Scan and visualize disk usage (default: /)
lintree -v           Show version and check for updates
lintree -h           Show help
```

## Controls

| Key | Action |
|---|---|
| `↑` `↓` / `j` `k` | Navigate between cells |
| `←` `→` / `h` `l` | Spatial movement |
| `Enter` / `l` | Drill into directory |
| `Backspace` / `h` | Go back to parent |
| `Esc` | Go back or quit |
| `?` | Toggle help overlay |
| `q` / `Ctrl+C` | Quit |
| Mouse click | Select a cell |

## Color Legend

| Color | File Type |
|---|---|
| Cyan | Directories |
| Blue | Source code (.go, .py, .js, .rs, .c, ...) |
| Teal | Web files (.html, .css, .vue, .tsx, ...) |
| Green | Documents (.pdf, .doc, .md, .txt, ...) |
| Yellow | Images (.png, .jpg, .svg, .webp, ...) |
| Magenta | Video (.mp4, .mkv, .mov, ...) |
| Pink | Audio (.mp3, .flac, .wav, ...) |
| Red | Archives (.zip, .tar.gz, .7z, .rar, ...) |
| Orange | Data (.json, .csv, .sql, .yaml, ...) |
| Dark Red | Binaries (.exe, .so, .dll, ...) |
| Gray | Other |

## Updating

Check your current version and see if an update is available:

```bash
lintree -v
```

Update to the latest:

```bash
curl -fsSL https://raw.githubusercontent.com/PatchMon/lintree/main/install.sh | sh
```

## Development

```bash
# Build
make build

# Run
make run ARGS=/home

# Test
make test

# Lint
make lint
```

## How it works

1. **Concurrent filesystem scan.** A bounded goroutine pool walks the directory tree in parallel, collecting file sizes using actual disk block usage (`stat.Blocks * 512`) where available.

2. **Squarified treemap layout.** The [Bruls-Huizing-van Wijk algorithm](https://vanwijk.win.tue.nl/stm.pdf) arranges items into near-square rectangles proportional to their size, with a correction factor for terminal cell aspect ratio (characters are roughly 2x taller than wide).

3. **TUI rendering.** Built on [tcell](https://github.com/gdamore/tcell) for cell-by-cell terminal control with 24-bit truecolor support.

## Support

If lintree is useful to you, consider supporting the project:

<a href="https://buymeacoffee.com/iby___">
  <img src="https://img.shields.io/badge/Buy%20Me%20a%20Coffee-ffdd00?style=for-the-badge&logo=buy-me-a-coffee&logoColor=black" alt="Buy Me A Coffee">
</a>

**Website:** [lintree.sh](https://lintree.sh)

## License

MIT. See [LICENSE](LICENSE) for details.
