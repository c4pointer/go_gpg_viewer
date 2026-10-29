# GPG Password Store Viewer

A modern, cross-platform GUI application for browsing and managing password-store entries with GPG encryption support. Built with Go and Fyne framework.

![GPG Password Store Viewer](https://img.shields.io/badge/Go-1.26+-blue.svg)
![Fyne](https://img.shields.io/badge/Fyne-2.6.1+-green.svg)
![License](https://img.shields.io/badge/License-MIT-yellow.svg)

## Features

- 🔐 **GPG Integration**: Records are decrypted with gpg-agent (or a passphrase
  prompt) and re-encrypted for the recipients in the store's `.gpg-id`, like `pass`
- 📁 **Folder tree and search**: Browse nested folders or search the whole store
- ✏️ **Full editing**: Edit the password and notes, rename or move a record to
  another folder, delete it, generate a strong password, copy it to a clipboard
  that clears itself after 30 seconds
- 🔄 **Git Integration**: Every change can be committed automatically; **Sync**
  works out on its own whether to commit, pull or push
- 🎨 **Theme Support**: Follows the system light/dark preference, or pick one
- 🖥️ **Wayland and X11**: One binary, the right backend is chosen at start

## Quick Start

```bash
# Clone and install in one command
git clone  git@github.com:c4pointer/go_gpg_viewer.git && cd go_gpg_viewer && ./install.sh

# Run the application
gpg_viewer

# To uninstall later
make uninstall-user  # for user installation
# or
make uninstall       # for system-wide installation
```

## Screenshots

*[Screenshots will be added here]*

## Prerequisites

### System Requirements

- **Operating System**: Linux (tested on RHEL 9, Ubuntu, Debian)
- **Go Version**: 1.26 or higher
- **GPG**: GnuPG installed and configured
- **Git**: For repository synchronization (optional)

### Required Dependencies

The GUI is built with [Fyne](https://fyne.io), which needs the OpenGL, X11 and
Wayland development headers at build time.

#### Ubuntu/Debian
```bash
sudo apt update
sudo apt install golang-go gpg git build-essential \
  libgl1-mesa-dev xorg-dev libxkbcommon-dev libwayland-dev wayland-protocols
```

#### Fedora
```bash
sudo dnf install golang gpg git gcc mesa-libGL-devel libXcursor-devel \
  libXrandr-devel libXinerama-devel libXi-devel libxkbcommon-devel \
  wayland-devel wayland-protocols-devel
```

#### Arch Linux
```bash
sudo pacman -S go gnupg git base-devel mesa libxcursor libxrandr libxinerama \
  libxi libxkbcommon wayland wayland-protocols
```

Without the Wayland headers, build an X11-only binary with `make TAGS=x11`
(it also runs under XWayland).

## Installation

### Method 1: Using the Installation Script (Recommended)

The easiest way to install GPG Password Store Viewer:

```bash
# Clone the repository
git clone https://github.com/c4pointer/go_gpg_viewer.git
cd go_gpg_viewer

# Install for current user (recommended)
./install.sh

# Or install system-wide (requires sudo)
sudo ./install.sh --system

# Install dependencies only
./install.sh --deps-only

# Build only, don't install
./install.sh --build-only
```

The installation script will:
- ✅ Automatically detect your Linux distribution
- ✅ Install required dependencies (Go, GPG, Git)
- ✅ Build the application with proper version information
- ✅ Create desktop shortcuts
- ✅ Set up proper permissions

### Method 2: Using Make

If you prefer using Make:

```bash
# Clone the repository
git clone https://github.com/c4pointer/go_gpg_viewer.git
cd go_gpg_viewer

# Install for current user (recommended)
make install-user

# Install system-wide (requires sudo)
/

# Just build the application
make build 

# Show all available commands
make help
```

### Method 3: Manual Installation

For advanced users who want full control:

1. **Clone and build**
   ```bash
   git clone https://github.com/c4pointer/go_gpg_viewer.git
   cd go_gpg_viewer
   go build -o gpg_viewer
   chmod +x gpg_viewer
   ```

2. **Run directly**
   ```bash
   ./gpg_viewer
   ```

3. **Create desktop shortcut** (Optional)
   ```bash
   # For user installation
   mkdir -p ~/.local/share/applications
   cat > ~/.local/share/applications/gpg-viewer.desktop << EOF
   [Desktop Entry]
   Name=GPG Password Store Viewer
   Comment=Modern GUI for password-store with GPG support
   Exec=$(pwd)/gpg_viewer
   Icon=security-high
   Terminal=false
   Type=Application
   Categories=Utility;Security;
   Keywords=password;gpg;security;
   EOF
   ```

## Uninstallation

### Method 1: Using Make (Recommended)

If you installed using Make, use the corresponding uninstall command:

```bash
# For user installation
make uninstall-user

# For system-wide installation
make uninstall
```

### Method 2: Using Installation Script

The installation script doesn't have a built-in uninstall option, but you can manually remove the files:

```bash
# For user installation
rm -f ~/.local/bin/gpg_viewer
rm -f ~/.local/share/applications/gpg-viewer.desktop
update-desktop-database ~/.local/share/applications

# For system-wide installation
sudo rm -f /usr/local/bin/gpg_viewer
sudo rm -f /usr/share/applications/gpg-viewer.desktop
```

### Method 3: Manual Uninstallation

If you installed manually or need to clean up completely:

#### User Installation Cleanup
```bash
# Remove binary
rm -f ~/.local/bin/gpg_viewer

# Remove desktop shortcut
rm -f ~/.local/share/applications/gpg-viewer.desktop

# Update desktop database
update-desktop-database ~/.local/share/applications

# Remove configuration (optional)
rm -rf ~/.config/go_gpg_viewer
```

#### System-wide Installation Cleanup
```bash
# Remove binary
sudo rm -f /usr/local/bin/gpg_viewer

# Remove desktop shortcut
sudo rm -f /usr/share/applications/gpg-viewer.desktop

# Update desktop database
sudo update-desktop-database
```

#### Complete Cleanup (All Methods)
```bash
# Remove all possible installation locations
sudo rm -f /usr/local/bin/gpg_viewer
sudo rm -f /usr/bin/gpg_viewer
rm -f ~/.local/bin/gpg_viewer

# Remove desktop shortcuts
sudo rm -f /usr/share/applications/gpg-viewer.desktop
rm -f ~/.local/share/applications/gpg-viewer.desktop

# Update desktop databases
sudo update-desktop-database
update-desktop-database ~/.local/share/applications

# Remove configuration files (optional)
rm -rf ~/.config/go_gpg_viewer

# Remove build artifacts from source directory
cd /path/to/go_gpg_viewer/source
make clean
# or manually:
rm -f gpg_viewer
rm -rf build/
go clean
```

### Verification

After uninstallation, verify that the application has been removed:

```bash
# Check if binary exists
which gpg_viewer

# Check if desktop shortcut exists
ls ~/.local/share/applications/gpg-viewer.desktop
ls /usr/share/applications/gpg-viewer.desktop

# Try to run the application
gpg_viewer
```

If the commands above return "not found" or similar errors, the uninstallation was successful.

## Configuration

### Password Store Setup

1. **Initialize password store** (if not already done)
   ```bash
   # Create password store directory
   mkdir -p ~/.password-store
   
   # Initialize git repository (optional but recommended)
   cd ~/.password-store
   git init
   git remote add origin <your-remote-repo-url>
   ```

2. **Configure GPG** (if not already done)
   ```bash
   # Generate a new GPG key (if needed)
   gpg --full-generate-key
   
   # List your keys
   gpg --list-secret-keys --keyid-format LONG
   ```

3. **Create your first password**
   ```bash
   # Using pass command (if installed)
   pass insert example.com
   
   # Or manually create a GPG file
   echo "your-password-here" | gpg --encrypt --recipient your-email@example.com > ~/.password-store/example.com.gpg
   ```

### Application Settings

The application creates `~/.config/gpg_viewer/settings.json` (or the
`$XDG_CONFIG_HOME` equivalent) on first run. Everything in it can be changed
from the Settings dialog:

```json
{
  "password_store_path": "/home/username/.password-store",
  "default_recipient": "your-email@example.com",
  "auto_commit": true,
  "show_notifications": true,
  "theme": "system",
  "window_width": 800,
  "window_height": 600,
  "split_offset": 0.3
}
```

## Usage

### Starting the Application

```bash
# If built locally
./gpg_viewer

# If installed system-wide
gpg_viewer

# Or launch from desktop menu
```

### Basic Operations

1. **Browse the store**
   - The folder tree on the left mirrors your password store; a selected folder
     lists its records on the right
   - Type in the search box to find records anywhere in the store by name or path
   - The status bar shows what just happened and how many records the store holds

2. **Open and edit a record**
   - Click a record to decrypt it (gpg-agent is used when available, otherwise
     you are asked for the passphrase)
   - The first line is the password, everything else is notes, as in `pass`
   - Change the **Name** to rename the record or move it to another folder
     (`Work/github`); missing folders are created and emptied ones removed
   - **Copy** puts the password on the clipboard and clears it after 30 seconds;
     **Generate** fills in a 20-character random password; **Delete** removes
     the record after confirmation
   - Saved records are encrypted for the recipients in the nearest `.gpg-id`;
     without one, the original file's key or the default recipient is used

3. **New record**
   - **New record** (or `Ctrl+N`) opens a form; with a folder selected the name
     is prefilled with `Folder/`
   - Leave **Encrypt for** empty to use the store's `.gpg-id`

4. **Git**
   - With *Commit every change to git* enabled (default), each save, rename,
     create or delete becomes a commit, as with `pass`
   - **Commit** records any pending changes by hand
   - **Sync** commits pending changes, fetches, then pulls and/or pushes only
     what the branch's ahead/behind state requires. A branch without an
     upstream is published to `origin`. On a conflict the pull is undone, and
     sync refuses to run while a rebase or merge is unfinished

5. **Settings** (⚙️)
   - Password store folder (switched without a restart), default recipient,
     auto-commit, notifications and theme

### Keyboard Shortcuts

- `Ctrl+N`: New record
- `Ctrl+F`: Jump to the search box
- `Ctrl+Z` / `Ctrl+Y`: Undo / redo inside a text field

## Troubleshooting

### Common Issues

1. **"GPG not found" error**
   ```bash
   # Install GPG
   sudo apt install gnupg  # Ubuntu/Debian
   sudo dnf install gnupg  # RHEL/Fedora
   ```

2. **"Permission denied" when accessing password store**
   ```bash
   # Fix permissions
   chmod 700 ~/.password-store
   chmod 600 ~/.password-store/*.gpg
   ```

3. **"No GPG key found" error**
   ```bash
   # List available keys
   gpg --list-secret-keys
   
   # Generate a new key if needed
   gpg --full-generate-key
   ```

4. **Application won't start**
   ```bash
   # Check Go installation
   go version
   
   # Rebuild with verbose output
   go build -v -o gpg_viewer
   
   # Check for missing dependencies
   go mod tidy
   
   # Or use Make
   make clean
   make build
   ```

5. **Desktop shortcut not working**
   ```bash
   # Update desktop database
   update-desktop-database ~/.local/share/applications
   
   # Or for system-wide installation
   sudo update-desktop-database
   ```

6. **Installation script fails**
   ```bash
   # Make script executable
   chmod +x install.sh
   
   # Run with verbose output
   bash -x install.sh
   
   # Check script syntax
   bash -n install.sh
   ```

### Debug Mode

To run with debug output:

```bash
# Using Make (recommended)
make build-debug
./build/gpg_viewer

# Or manually
go build -tags debug -o gpg_viewer
./gpg_viewer
```

## Development

### Building for Different Platforms

```bash
# Using Make (recommended)
make release

# Or manually
# Linux
GOOS=linux GOARCH=amd64 go build -o gpg_viewer_linux_amd64

# Windows
GOOS=windows GOARCH=amd64 go build -o gpg_viewer_windows_amd64.exe

# macOS
GOOS=darwin GOARCH=amd64 go build -o gpg_viewer_darwin_amd64
```

### Development Commands

```bash
# Setup development environment
make setup-dev

# Build and run
make run

# Development mode with hot reload (requires air)
make dev

# Code quality checks
make check
make lint
make format

# Testing
make test

# Quick install for development
make quick-install
```

### Project Structure

```
go_gpg_viewer/
├── main.go                 # Main application entry point
├── go.mod                  # Go module definition
├── go.sum                  # Go module checksums
├── Makefile                # Build and installation automation
├── install.sh              # Smart installation script
├── LICENSE                 # MIT License
├── README.md               # This documentation
├── scanpassstore/          # Password store scanning logic
│   └── scan.go
├── settings/               # Application settings
│   ├── dialog.go          # Settings dialog UI
│   ├── settings.go        # Settings management
│   └── theme.go           # Theme handling
└── assets/                 # Application assets
    ├── assets.go          # Embedded resources
    └── icon.svg           # Application icon
```

### Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- [Fyne](https://fyne.io/) - Cross-platform GUI framework for Go
- [pass](https://www.passwordstore.org/) - Standard Unix password manager
- [GnuPG](https://gnupg.org/) - GNU Privacy Guard

## Support

- **Issues**: [GitHub Issues](https://github.com/c4pointer/go_gpg_viewer/issues)
- **Discussions**: [GitHub Discussions](https://github.com/c4pointer/go_gpg_viewer/discussions)
- **Email**: c4point@gmail.com

---

**Author**: Oleg Zubak <c4point@gmail.com>

*Built with ❤️ using Go and Fyne* 