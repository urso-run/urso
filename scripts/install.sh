#!/bin/sh
set -e

# Urso Installation Script
# This script downloads and installs the latest version of Urso for macOS into the user's home directory.
#
# Usage:
#   curl -sSL https://raw.githubusercontent.com/urso-run/urso/main/scripts/install.sh | sh
#   curl -sSL https://raw.githubusercontent.com/urso-run/urso/main/scripts/install.sh | sh -s -- <token>

main() {
  token="$1"
  repo="urso-run/urso"

  # Define installation directory within the user's home to avoid sudo
  urso_home="$HOME/.urso"
  install_dir="$urso_home/bin"
  install_path="$install_dir/urso"

  # 1. Platform Check
  os=$(uname -s)
  if [ "$os" != "Darwin" ]; then
    echo "Error: Urso is only supported on macOS (Darwin)."
    exit 1
  fi

  # 2. Architecture Check
  arch=$(uname -m)
  case "$arch" in
    x86_64) arch="x86_64" ;;
    arm64)  arch="arm64" ;;
    *)
      echo "Error: Unsupported architecture $arch. Urso supports x86_64 and arm64."
      exit 1
      ;;
  esac

  # 3. Check if already installed
  if [ -f "$install_path" ]; then
    echo "Urso is already installed at $install_path"
  else
    # 4. Fetch Latest Release
    echo "Checking for the latest Urso release..."
    latest_version=$(curl -s "https://api.github.com/repos/$repo/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

    if [ -z "$latest_version" ]; then
      echo "Error: Could not retrieve the latest release version from GitHub."
      exit 1
    fi
    echo "Latest version found: $latest_version"

    # 5. Construct Download URL
    filename="urso_Darwin_${arch}.tar.gz"
    download_url="https://github.com/$repo/releases/download/$latest_version/$filename"

    # 6. Download and Extract
    tmp_dir=$(mktemp -d)
    trap 'rm -rf "$tmp_dir"' EXIT

    echo "Downloading Urso from $download_url..."
    if ! curl -sSL "$download_url" -o "$tmp_dir/urso.tar.gz"; then
      echo "Error: Failed to download Urso."
      exit 1
    fi

    echo "Extracting..."
    tar -xzf "$tmp_dir/urso.tar.gz" -C "$tmp_dir"

    # 7. Install Binary
    binary_source=$(find "$tmp_dir" -type f -name "urso" | head -n 1)

    if [ -z "$binary_source" ]; then
      echo "Error: Could not find 'urso' binary in the downloaded archive."
      exit 1
    fi

    echo "Installing Urso to $install_dir..."
    mkdir -p "$install_dir"
    mv "$binary_source" "$install_path"
    chmod +x "$install_path"
  fi

  # 8. Initialize
  echo "Initializing Urso..."
  # Run init from the new path.
  "$install_path" init < /dev/null || echo "Note: 'urso init' skipped or already initialized."

  # 9. Handle Managed Mode if Token Provided
  if [ -n "$token" ]; then
    echo "Registration token provided. Installing Urso as a service..."
    "$install_path" install --urso-registration-token "$token"
  else
    echo ""
    echo "Installation complete!"
  fi

  # 10. PATH check and instructions
  case :$PATH: in
    *:$install_dir:*) ;;
    *)
      echo ""
      echo "Note: $install_dir is not in your PATH."
      echo "You can add it by adding this to your shell profile (~/.zshrc or ~/.bash_profile):"
      echo "  export PATH=\"\$HOME/.urso/bin:\$PATH\""
      echo ""
      ;;
  esac

  if [ -z "$token" ]; then
    echo "To use managed mode, run:"
    echo "  $install_path install --urso-registration-token <YOUR_TOKEN>"
  fi
}

main "$@"
