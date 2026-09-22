# typed: false
# frozen_string_literal: true

# Generated from the published v3.8.2 checksums. DO NOT EDIT.
class Dcs < Formula
  desc "CLI for Dilmune Cloud Services"
  homepage "https://dilmune.com"
  version "3.8.2"
  license "MIT"

  on_macos do
    if Hardware::CPU.intel?
      url "https://github.com/Dilmune/dcs-cli/releases/download/v3.8.2/dcs_darwin_amd64.tar.gz"
      sha256 "e0c90d4ab5f891353a6de9e37789b080617e6be48f9ee1337269ce3c9c5883f9"
      define_method(:install) do
        bin.install "dcs"
      end
    end
    if Hardware::CPU.arm?
      url "https://github.com/Dilmune/dcs-cli/releases/download/v3.8.2/dcs_darwin_arm64.tar.gz"
      sha256 "e99fe789631d0aa6a18208b3e3fd25a4a04fdffaaa0ab5fdc853192c5d9119ed"
      define_method(:install) do
        bin.install "dcs"
      end
    end
  end

  on_linux do
    if Hardware::CPU.intel? && Hardware::CPU.is_64_bit?
      url "https://github.com/Dilmune/dcs-cli/releases/download/v3.8.2/dcs_linux_amd64.tar.gz"
      sha256 "ca7f6e706a9fbf7fe5677ad460a89f88c2e73358d82e14b4120f9bd2f2dd3a43"
      define_method(:install) do
        bin.install "dcs"
      end
    end
    if Hardware::CPU.arm? && Hardware::CPU.is_64_bit?
      url "https://github.com/Dilmune/dcs-cli/releases/download/v3.8.2/dcs_linux_arm64.tar.gz"
      sha256 "183c1d61db98116237dfb652b1546bcf4849d21e07b22d8cfa036c3bb1d1529e"
      define_method(:install) do
        bin.install "dcs"
      end
    end
  end

  test do
    system "#{bin}/dcs", "version", "--json"
  end
end
