#!/bin/bash

# GPG Password Store Viewer Installation Script
# Author: Oleg Zubak <c4point@gmail.com>

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Variables
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BINARY_NAME="gpg_viewer"
VERSION="1.0.0"

# Function to print colored output
print_status() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_header() {
    echo -e "${BLUE}================================${NC}"
    echo -e "${BLUE}  GPG Password Store Viewer${NC}"
    echo -e "${BLUE}  Installation Script v${VERSION}${NC}"
    echo -e "${BLUE}================================${NC}"
    echo
}

# Function to check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Function to detect OS
detect_os() {
    if [[ -f /etc/os-release ]]; then
        . /etc/os-release
        OS=$NAME
        VER=$VERSION_ID
    elif type lsb_release >/dev/null 2>&1; then
        OS=$(lsb_release -si)
        VER=$(lsb_release -sr)
    elif [[ -f /etc/lsb-release ]]; then
        . /etc/lsb-release
        OS=$DISTRIB_ID
        VER=$DISTRIB_RELEASE
    elif [[ -f /etc/debian_version ]]; then
        OS=Debian
        VER=$(cat /etc/debian_version)
    elif [[ -f /etc/SuSe-release ]]; then
        OS=SuSE
    elif [[ -f /etc/redhat-release ]]; then
        OS=RedHat
    else
        OS=$(uname -s)
        VER=$(uname -r)
    fi
    echo "$OS"
}

# Function to install dependencies
install_dependencies() {
    local os=$(detect_os)
    print_status "Detected OS: $os"
    
    if ! command_exists go; then
        print_warning "Go is not installed. Installing dependencies..."
        
        case "$os" in
            *"Ubuntu"*|*"Debian"*)
                sudo apt update
                sudo apt install -y golang-go gpg git build-essential
                ;;
            *"Red Hat"*|*"CentOS"*|*"Fedora"*|*"RHEL"*)
                if command_exists dnf; then
                    sudo dnf install -y golang gpg git gcc
                elif command_exists yum; then
                    sudo yum install -y golang gpg git gcc
                fi
                ;;
            *"Arch"*)
                sudo pacman -S --noconfirm go gnupg git base-devel
                ;;
            *)
                print_error "Unsupported OS: $os"
                print_error "Please install Go manually: https://golang.org/doc/install"
                exit 1
                ;;
        esac
    else
        print_status "Go is already installed: $(go version)"
    fi
    
    if ! command_exists gpg; then
        print_error "GPG is not installed. Please install it manually."
        exit 1
    fi
    
    print_status "Dependencies check completed"
}

# Function to build the application
build_application() {
    print_status "Building application..."
    
    if [[ ! -f "$SCRIPT_DIR/go.mod" ]]; then
        print_error "go.mod not found. Are you in the correct directory?"
        exit 1
    fi
    
    cd "$SCRIPT_DIR"
    
    # Download dependencies
    go mod download
    go mod tidy
    
    # Build the application
    go build -ldflags "-X main.Version=$VERSION" -o "$BINARY_NAME"
    
    if [[ ! -f "$BINARY_NAME" ]]; then
        print_error "Build failed!"
        exit 1
    fi
    
    chmod +x "$BINARY_NAME"
    print_status "Build completed successfully"
}

# Function to install system-wide
install_system_wide() {
    print_status "Installing system-wide..."
    
    # Copy binary
    sudo cp "$BINARY_NAME" /usr/local/bin/
    
    # Create desktop entry
    sudo tee /usr/share/applications/gpg-viewer.desktop > /dev/null << EOF
[Desktop Entry]
Name=GPG Password Store Viewer
Comment=Modern GUI for password-store with GPG support
Exec=/usr/local/bin/gpg_viewer
Icon=security-high
Terminal=false
Type=Application
Categories=Utility;Security;
Keywords=password;gpg;security;
EOF
    
    print_status "System-wide installation completed"
    print_status "You can now run 'gpg_viewer' from anywhere"
}

# Function to install for current user only
install_user_local() {
    print_status "Installing for current user..."
    
    # Create user bin directory
    mkdir -p ~/.local/bin
    
    # Copy binary
    cp "$BINARY_NAME" ~/.local/bin/
    
    # Create desktop entry
    mkdir -p ~/.local/share/applications
    tee ~/.local/share/applications/gpg-viewer.desktop > /dev/null << EOF
[Desktop Entry]
Name=GPG Password Store Viewer
Comment=Modern GUI for password-store with GPG support
Exec=$HOME/.local/bin/gpg_viewer
Icon=security-high
Terminal=false
Type=Application
Categories=Utility;Security;
Keywords=password;gpg;security;
EOF
    
    # Update desktop database
    if command_exists update-desktop-database; then
        update-desktop-database ~/.local/share/applications
    fi
    
    print_status "User installation completed"
    print_warning "Make sure ~/.local/bin is in your PATH:"
    echo "export PATH=\"\$HOME/.local/bin:\$PATH\""
}

# Function to show usage
show_usage() {
    echo "Usage: $0 [OPTIONS]"
    echo
    echo "Options:"
    echo "  -h, --help          Show this help message"
    echo "  -u, --user          Install for current user only (default)"
    echo "  -s, --system        Install system-wide (requires sudo)"
    echo "  -d, --deps-only     Install dependencies only"
    echo "  -b, --build-only    Build only, don't install"
    echo "  -c, --clean         Clean build artifacts before building"
    echo
    echo "Examples:"
    echo "  $0                  # Install for current user"
    echo "  $0 --system         # Install system-wide"
    echo "  $0 --deps-only      # Install dependencies only"
    echo "  $0 --build-only     # Build only"
}

# Function to clean build artifacts
clean_build() {
    print_status "Cleaning build artifacts..."
    rm -f "$BINARY_NAME"
    go clean
    print_status "Clean completed"
}

# Main installation function
main() {
    local install_type="user"
    local deps_only=false
    local build_only=false
    local clean_build_flag=false
    
    # Parse command line arguments
    while [[ $# -gt 0 ]]; do
        case $1 in
            -h|--help)
                show_usage
                exit 0
                ;;
            -u|--user)
                install_type="user"
                shift
                ;;
            -s|--system)
                install_type="system"
                shift
                ;;
            -d|--deps-only)
                deps_only=true
                shift
                ;;
            -b|--build-only)
                build_only=true
                shift
                ;;
            -c|--clean)
                clean_build_flag=true
                shift
                ;;
            *)
                print_error "Unknown option: $1"
                show_usage
                exit 1
                ;;
        esac
    done
    
    print_header
    
    # Check if running as root for system installation
    if [[ "$install_type" == "system" && "$EUID" -ne 0 ]]; then
        print_error "System installation requires root privileges"
        print_error "Run with sudo or use --user for user installation"
        exit 1
    fi
    
    # Install dependencies
    install_dependencies
    
    if [[ "$deps_only" == true ]]; then
        print_status "Dependencies installation completed"
        exit 0
    fi
    
    # Clean if requested
    if [[ "$clean_build_flag" == true ]]; then
        clean_build
    fi
    
    # Build application
    build_application
    
    if [[ "$build_only" == true ]]; then
        print_status "Build completed"
        exit 0
    fi
    
    # Install based on type
    if [[ "$install_type" == "system" ]]; then
        install_system_wide
    else
        install_user_local
    fi
    
    print_status "Installation completed successfully!"
    echo
    print_status "You can now run the application:"
    if [[ "$install_type" == "system" ]]; then
        echo "  gpg_viewer"
    else
        echo "  ~/.local/bin/gpg_viewer"
        echo "  or add ~/.local/bin to your PATH"
    fi
    echo
    print_status "For more information, see README.md"
}

# Run main function with all arguments
main "$@" 