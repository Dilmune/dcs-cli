import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { parseChecksums } from './release-check.mjs'

const STABLE_VERSION = /^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$/
const RELEASES = 'https://github.com/Dilmune/dcs-cli/releases/download'
const PLATFORMS = [
  {
    block: 'on_macos',
    targets: [
      ['Hardware::CPU.intel?', 'dcs_darwin_amd64.tar.gz'],
      ['Hardware::CPU.arm?', 'dcs_darwin_arm64.tar.gz'],
    ],
  },
  {
    block: 'on_linux',
    targets: [
      [
        'Hardware::CPU.intel? && Hardware::CPU.is_64_bit?',
        'dcs_linux_amd64.tar.gz',
      ],
      [
        'Hardware::CPU.arm? && Hardware::CPU.is_64_bit?',
        'dcs_linux_arm64.tar.gz',
      ],
    ],
  },
]

function target(condition, archive, version, checksums) {
  return [
    `    if ${condition}`,
    `      url "${RELEASES}/v${version}/${archive}"`,
    `      sha256 "${checksums.get(archive)}"`,
    '      define_method(:install) do',
    '        bin.install "dcs"',
    '      end',
    '    end',
  ].join('\n')
}

export function renderFormula(version, checksumsText) {
  assert.match(version, STABLE_VERSION, 'Version must be stable X.Y.Z')
  const checksums = parseChecksums(checksumsText)
  const platforms = PLATFORMS.map(({ block, targets }) =>
    [
      `  ${block} do`,
      ...targets.map(([condition, archive]) =>
        target(condition, archive, version, checksums),
      ),
      '  end',
    ].join('\n'),
  )
  return [
    '# typed: false',
    '# frozen_string_literal: true',
    '',
    `# Generated from the published v${version} checksums. DO NOT EDIT.`,
    'class Dcs < Formula',
    '  desc "CLI for Dilmune Cloud Services"',
    '  homepage "https://dilmune.com"',
    `  version "${version}"`,
    '  license "MIT"',
    '',
    platforms.join('\n\n'),
    '',
    '  test do',
    '    system "#{bin}/dcs", "version", "--json"',
    '  end',
    'end',
    '',
  ].join('\n')
}

if (
  process.argv[1] &&
  path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)
) {
  try {
    const [version, checksumsFile, ...extra] = process.argv.slice(2)
    assert.ok(
      version && checksumsFile && extra.length === 0,
      'Usage: node scripts/homebrew-formula.mjs <version> <checksums.txt>',
    )
    process.stdout.write(
      renderFormula(version, readFileSync(checksumsFile, 'utf8')),
    )
  } catch (error) {
    console.error(`Homebrew formula: ${error.message}`)
    process.exitCode = 1
  }
}
