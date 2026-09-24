import assert from 'node:assert/strict'
import { mkdtempSync, writeFileSync } from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import test from 'node:test'
import {
  block,
  compareInventory,
  compareNames,
  header,
  parseModuleList,
  parseNotices,
  parseReport,
  TARGETS,
  toolchainVersion,
  versionToken,
} from './third-party-notices.mjs'

const row = (fields) => `${fields.join('\t')}\n`

test('the six release targets are covered', () => {
  assert.equal(TARGETS.length, 6)
  for (const goos of ['linux', 'darwin', 'windows']) {
    const arches = TARGETS.filter(([o]) => o === goos).map(([, a]) => a)
    assert.deepEqual(arches.sort(), ['amd64', 'arm64'])
  }
})

test('the pinned toolchain is read from go.mod', () => {
  assert.equal(toolchainVersion('go 1.25.8\n\ntoolchain go1.26.8\n'), '1.26.8')
  assert.throws(() => toolchainVersion('go 1.25.8\n'), /must pin a stable/)
  assert.throws(() => toolchainVersion('toolchain go1.26\n'), /must pin/)
})

test('a pseudo-version is identified by its commit hash', () => {
  assert.equal(versionToken('v0.4.3'), 'v0.4.3')
  assert.equal(
    versionToken('v0.0.0-20260811164956-006e29f97886'),
    '006e29f97886',
  )
})

test('names sort case-insensitively', () => {
  const sorted = [
    'github.com/zalando/go-keyring/LICENSE',
    'github.com/zalando/go-keyring/internal/shellescape/LICENSE',
  ].sort(compareNames)
  assert.equal(
    sorted[0],
    'github.com/zalando/go-keyring/internal/shellescape/LICENSE',
  )
})

test('the main module is not its own dependency', () => {
  const modules = parseModuleList(
    row(['github.com/dilmune/dcs-cli', '']) +
      row(['github.com/spf13/cobra', 'v1.10.2']) +
      row(['golang.org/x/term', 'v0.40.0']),
  )
  assert.deepEqual(
    [...modules],
    [
      ['github.com/spf13/cobra', 'v1.10.2'],
      ['golang.org/x/term', 'v0.40.0'],
    ],
  )
})

test('an unclassified license stops the run rather than shipping unattributed', () => {
  const unknown = row([
    'x/y',
    'v1.0.0',
    '/cache/x/y/LICENSE',
    'https://x/y',
    'Unknown',
  ])
  assert.throws(
    () => parseReport(unknown),
    /could not determine the license of x\/y/,
  )
  const missing = row(['x/y', 'v1.0.0', '', '', ''])
  assert.throws(() => parseReport(missing), /no license file for x\/y/)
  const good = row([
    'x/y',
    'v1.0.0',
    '/cache/x/y/LICENSE',
    'https://x/y',
    'MIT',
  ])
  assert.equal(parseReport(good).length, 1)
})

test('license text is fenced verbatim with its trailing blank lines trimmed', () => {
  const dir = mkdtempSync(path.join(os.tmpdir(), 'dcs-notices-test-'))
  const file = path.join(dir, 'LICENSE')
  writeFileSync(file, '\nMIT License\r\n\r\nBe nice.\n\n\n')
  assert.equal(
    block('x/y/LICENSE', file),
    '### x/y/LICENSE\n\n````text\n\nMIT License\n\nBe nice.\n````\n',
  )
})

test('the header names the tool and toolchain it was generated with', () => {
  const text = header('1.26.8')
  assert.match(text, /go-licenses\nv2\.0\.1 and Go 1\.26\.8/)
})

const NOTICES = [
  '# Third-party notices',
  '',
  '## Dependency sources',
  '',
  '- [github.com/spf13/cobra](https://github.com/spf13/cobra/blob/v1.10.2/LICENSE.txt)',
  '- [golang.org/x/sys/unix](https://cs.opensource.google/go/x/sys/+/v0.47.0:LICENSE)',
  '',
  '## Dependency license and notice texts',
  '',
  '### github.com/spf13/cobra/LICENSE.txt',
  '',
  '### golang.org/x/sys/unix/LICENSE',
  '',
  '## Go runtime and vendored runtime dependencies',
  '',
  '### Go 1.26.8: LICENSE',
  '',
].join('\n')

test('only the sources list and the text headings are parsed', () => {
  const { sources, headings } = parseNotices(NOTICES)
  assert.deepEqual(
    sources.map((s) => s.name),
    ['github.com/spf13/cobra', 'golang.org/x/sys/unix'],
  )
  assert.deepEqual(headings, [
    'github.com/spf13/cobra/LICENSE.txt',
    'golang.org/x/sys/unix/LICENSE',
  ])
})

test('a matching inventory reports no problems', () => {
  const { sources, headings } = parseNotices(NOTICES)
  const modules = new Map([
    ['github.com/spf13/cobra', 'v1.10.2'],
    ['golang.org/x/sys', 'v0.47.0'],
  ])
  assert.deepEqual(compareInventory(sources, headings, modules), [])
})

test('drift is reported in both directions', () => {
  const { sources, headings } = parseNotices(NOTICES)
  const modules = new Map([
    ['golang.org/x/sys', 'v0.47.0'],
    ['charm.land/huh/v2', 'v2.0.3'],
  ])
  assert.deepEqual(compareInventory(sources, headings, modules), [
    'listed but not linked by any target: github.com/spf13/cobra',
    'linked but unattributed: charm.land/huh/v2',
  ])
})

test('a version that moved underneath the URL is caught', () => {
  const { sources, headings } = parseNotices(NOTICES)
  const modules = new Map([
    ['github.com/spf13/cobra', 'v1.10.2'],
    ['golang.org/x/sys', 'v0.44.0'],
  ])
  assert.deepEqual(compareInventory(sources, headings, modules), [
    'golang.org/x/sys/unix: linked at v0.44.0, license URL points elsewhere ' +
      '(https://cs.opensource.google/go/x/sys/+/v0.47.0:LICENSE)',
  ])
})

test('a listed dependency with no license text is caught', () => {
  const { sources } = parseNotices(NOTICES)
  const modules = new Map([
    ['github.com/spf13/cobra', 'v1.10.2'],
    ['golang.org/x/sys', 'v0.47.0'],
  ])
  const headings = ['github.com/spf13/cobra/LICENSE.txt']
  assert.deepEqual(compareInventory(sources, headings, modules), [
    'golang.org/x/sys/unix: listed with no license text section',
  ])
})
