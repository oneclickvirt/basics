#!/bin/bash
#From https://github.com/oneclickvirt/basics
#2024.05.06

rm -rf basics
os=$(uname -s)
arch=$(uname -m)

case $os in
  Linux)
    case $arch in
      "x86_64" | "x86" | "amd64" | "x64")
        wget -O basics https://github.com/oneclickvirt/basics/releases/download/output/basics-linux-amd64
        ;;
      "i386" | "i686")
        wget -O basics https://github.com/oneclickvirt/basics/releases/download/output/basics-linux-386
        ;;
      "armv7l" | "armv8" | "armv8l" | "aarch64")
        wget -O basics https://github.com/oneclickvirt/basics/releases/download/output/basics-linux-arm64
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
        wget -O basics https://github.com/oneclickvirt/basics/releases/download/output/basics-darwin-amd64
        ;;
      "i386" | "i686")
        wget -O basics https://github.com/oneclickvirt/basics/releases/download/output/basics-darwin-386
        ;;
      "armv7l" | "armv8" | "armv8l" | "aarch64")
        wget -O basics https://github.com/oneclickvirt/basics/releases/download/output/basics-darwin-arm64
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
        wget -O basics https://github.com/oneclickvirt/basics/releases/download/output/basics-freebsd-amd64
        ;;
      "i386" | "i686")
        wget -O basics https://github.com/oneclickvirt/basics/releases/download/output/basics-freebsd-386
        ;;
      "armv7l" | "armv8" | "armv8l" | "aarch64")
        wget -O basics https://github.com/oneclickvirt/basics/releases/download/output/basics-freebsd-arm64
        ;;
      *)
        echo "Unsupported architecture: $arch"
        exit 1
        ;;
    esac
    ;;
  OpenBSD)
    case $arch in
      amd64)
        wget -O basics https://github.com/oneclickvirt/basics/releases/download/output/basics-openbsd-amd64
        ;;
      "i386" | "i686")
        wget -O basics https://github.com/oneclickvirt/basics/releases/download/output/basics-openbsd-386
        ;;
      "armv7l" | "armv8" | "armv8l" | "aarch64")
        wget -O basics https://github.com/oneclickvirt/basics/releases/download/output/basics-openbsd-arm64
        ;;
      *)
        echo "Unsupported architecture: $arch"
        exit 1
        ;;
    esac
    ;;
  *)
    echo "Unsupported operating system: $os"
    exit 1
    ;;
esac

chmod 777 basics
./basics