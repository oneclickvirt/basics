#!/bin/bash
#From https://github.com/oneclickvirt/basics
#2024.05.21

rm -rf /usr/bin/basics
rm -rf basics
os=$(uname -s)
arch=$(uname -m)

check_cdn() {
  local o_url=$1
  for cdn_url in "${cdn_urls[@]}"; do
    if curl -sL -k "$cdn_url$o_url" --max-time 6 | grep -q "success" >/dev/null 2>&1; then
      export cdn_success_url="$cdn_url"
      return
    fi
    sleep 0.5
  done
  export cdn_success_url=""
}

check_cdn_file() {
  check_cdn "https://raw.githubusercontent.com/spiritLHLS/ecs/main/back/test"
  if [ -n "$cdn_success_url" ]; then
    echo "CDN available, using CDN"
  else
    echo "No CDN available, no use CDN"
  fi
}

cdn_urls=("https://cdn0.spiritlhl.top/" "http://cdn3.spiritlhl.net/" "http://cdn1.spiritlhl.net/" "http://cdn2.spiritlhl.net/")
check_cdn_file

case $os in
Linux)
  case $arch in
  "x86_64" | "x86" | "amd64" | "x64")
    wget -O basics "${cdn_success_url}https://github.com/oneclickvirt/basics/releases/download/output/basics-linux-amd64"
    ;;
  "i386" | "i686")
    wget -O basics "${cdn_success_url}https://github.com/oneclickvirt/basics/releases/download/output/basics-linux-386"
    ;;
  "armv7l" | "armv8" | "armv8l" | "aarch64" | "arm64")
    wget -O basics "${cdn_success_url}https://github.com/oneclickvirt/basics/releases/download/output/basics-linux-arm64"
    ;;
  *)
    echo "Unsupported architecture: $arch"
    exit 1
    ;;
  esac
  ;;
Darwin)
  case $arch in
  "x86_64" | "x86" | "amd64" | "x64")
    wget -O basics "${cdn_success_url}https://github.com/oneclickvirt/basics/releases/download/output/basics-darwin-amd64"
    ;;
  "i386" | "i686")
    wget -O basics "${cdn_success_url}https://github.com/oneclickvirt/basics/releases/download/output/basics-darwin-386"
    ;;
  "armv7l" | "armv8" | "armv8l" | "aarch64" | "arm64")
    wget -O basics "${cdn_success_url}https://github.com/oneclickvirt/basics/releases/download/output/basics-darwin-arm64"
    ;;
  *)
    echo "Unsupported architecture: $arch"
    exit 1
    ;;
  esac
  ;;
FreeBSD)
  case $arch in
  amd64)
    wget -O basics "${cdn_success_url}https://github.com/oneclickvirt/basics/releases/download/output/basics-freebsd-amd64"
    ;;
  "i386" | "i686")
    wget -O basics "${cdn_success_url}https://github.com/oneclickvirt/basics/releases/download/output/basics-freebsd-386"
    ;;
  "armv7l" | "armv8" | "armv8l" | "aarch64" | "arm64")
    wget -O basics "${cdn_success_url}https://github.com/oneclickvirt/basics/releases/download/output/basics-freebsd-arm64"
    ;;
  *)
    echo "Unsupported architecture: $arch"
    exit 1
    ;;
  esac
  ;;
# OpenBSD)
#   case $arch in
#     amd64)
#       wget -O basics https://github.com/oneclickvirt/basics/releases/download/output/basics-openbsd-amd64
#       ;;
#     "i386" | "i686")
#       wget -O basics https://github.com/oneclickvirt/basics/releases/download/output/basics-openbsd-386
#       ;;
#     "armv7l" | "armv8" | "armv8l" | "aarch64" | "arm64")
#       wget -O basics https://github.com/oneclickvirt/basics/releases/download/output/basics-openbsd-arm64
#       ;;
#     *)
#       echo "Unsupported architecture: $arch"
#       exit 1
#       ;;
#   esac
#   ;;
*)
  echo "Unsupported operating system: $os"
  exit 1
  ;;
esac

chmod 777 basics
cp basics /usr/bin/basics
