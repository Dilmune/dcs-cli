import assert from 'node:assert/strict'
import { spawnSync } from 'node:child_process'
import { readFileSync } from 'node:fs'
import test from 'node:test'
import { fileURLToPath } from 'node:url'
import { renderFormula } from './homebrew-formula.mjs'

const SCRIPT = fileURLToPath(new URL('./homebrew-formula.mjs', import.meta.url))
const CHECKSUMS_FILE = fileURLToPath(
  new URL('./testdata/checksums-3.8.2.txt', import.meta.url),
)
const CHECKSUMS = readFileSync(CHECKSUMS_FILE, 'utf8')
const PUBLISHED = readFileSync(
  new URL('./testdata/dcs-3.8.2.rb', import.meta.url),
  'utf8',
)

function runScript(args) {
  const result = spawnSync(process.execPath, [SCRIPT, ...args], {
    encoding: 'utf8',
    timeout: 10_000,
  })
  assert.ifError(result.error)
  return result
}

test('renders the published 3.8.2 formula byte for byte', () => {
  assert.equal(renderFormula('3.8.2', CHECKSUMS), PUBLISHED)
})

test('the command prints the formula to stdout and nothing to stderr', () => {
  const result = runScript(['3.8.2', CHECKSUMS_FILE])
  assert.equal(result.status, 0, result.stderr)
  assert.equal(result.stdout, PUBLISHED)
  assert.equal(result.stderr, '')
})

test('every archive hash lands in the matching sha256 line', () => {
  const formula = renderFormula('3.8.2', CHECKSUMS)
  for (const line of CHECKSUMS.trim().split('\n')) {
    const [hash, archive] = line.split(/ {2}/)
    if (archive.endsWith('.zip')) {
      assert.doesNotMatch(formula, new RegExp(hash))
      continue
    }
    assert.match(
      formula,
      new RegExp(
        `url "https://github\\.com/Dilmune/dcs-cli/releases/download/v3\\.8\\.2/${archive.replaceAll('.', '\\.')}"\\n {6}sha256 "${hash}"`,
      ),
    )
  }
})

for (const version of ['03.8.2', '3.8.2-beta', 'v3.8.2', 'latest', '']) {
  test(`rejects a non-stable version: ${JSON.stringify(version)}`, () => {
    assert.throws(() => renderFormula(version, CHECKSUMS), /stable X\.Y\.Z/)
  })
}

test('rejects checksums that do not cover exactly the six archives', () => {
  const lines = CHECKSUMS.trim().split('\n')
  for (const variant of [
    lines.slice(1),
    [...lines, lines[0]],
    [...lines.slice(1), `${'a'.repeat(64)}  dcs_freebsd_amd64.tar.gz`],
    [...lines.slice(1), lines[0].replace(/^[0-9a-f]{4}/, 'zzzz')],
    [...lines.slice(1), lines[0].slice(1)],
    [...lines.slice(1), lines[0].replace(/ {2}/, ' ')],
    [],
  ]) {
    assert.throws(() => renderFormula('3.8.2', variant.join('\n')))
  }
})

test('the command exits non-zero with a message on bad input', () => {
  for (const [args, message] of [
    [[], /Usage/],
    [['3.8.2'], /Usage/],
    [['3.8.2', CHECKSUMS_FILE, 'extra'], /Usage/],
    [['3.8', CHECKSUMS_FILE], /stable X\.Y\.Z/],
    [['3.8.2', `${CHECKSUMS_FILE}.missing`], /ENOENT/],
  ]) {
    const result = runScript(args)
    assert.equal(result.status, 1, args.join(' '))
    assert.equal(result.stdout, '')
    assert.match(result.stderr, /^Homebrew formula: /)
    assert.match(result.stderr, message)
  }
})
