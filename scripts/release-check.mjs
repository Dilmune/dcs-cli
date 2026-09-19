import assert from 'node:assert/strict'
import { execFileSync } from 'node:child_process'
import { createHash } from 'node:crypto'
import {
  lstatSync,
  mkdirSync,
  mkdtempSync,
  readFileSync,
  realpathSync,
  rmSync,
  writeFileSync,
} from 'node:fs'
import { tmpdir } from 'node:os'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const MODULE = 'github.com/dilmune/dcs-cli'
const STABLE_VERSION = /^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$/
const DOCUMENTS = ['LICENSE', 'README.md', 'THIRD_PARTY_NOTICES.md']
const TARGETS = ['darwin', 'linux', 'windows'].flatMap((os) =>
  ['amd64', 'arm64'].map((arch) => ({
    os,
    arch,
    binary: os === 'windows' ? 'dcs.exe' : 'dcs',
    archive: `dcs_${os}_${arch}${os === 'windows' ? '.zip' : '.tar.gz'}`,
  })),
)

function run(command, args, options = {}) {
  return execFileSync(command, args, {
    stdio: ['ignore', 'pipe', 'pipe'],
    timeout: 60_000,
    maxBuffer: 32 * 1024 * 1024,
    ...options,
  })
}

export function validateTag(tag, version) {
  assert.match(version, STABLE_VERSION, 'Source version must be stable X.Y.Z')
  assert.equal(tag, `v${version}`, 'Tag must exactly match the source version')
}

export function checkSource({
  directory = process.cwd(),
  tag = '',
  env = process.env,
  git = (args) => run('git', args, { cwd: directory }).toString().trim(),
} = {}) {
  assert.equal(
    realpathSync(git(['rev-parse', '--show-toplevel'])),
    realpathSync(directory),
    'Build from a standalone CLI repository, not the private monorepo',
  )
  assert.equal(
    git(['status', '--porcelain', '--untracked-files=all']),
    '',
    'Source tree must be clean',
  )
  const moduleFile = readFileSync(path.join(directory, 'go.mod'), 'utf8')
  assert.match(
    moduleFile,
    /^module github\.com\/dilmune\/dcs-cli$/m,
    'Wrong Go module',
  )
  const toolchain = moduleFile.match(/^toolchain (go\d+\.\d+\.\d+)$/m)?.[1]
  assert.ok(toolchain, 'Missing stable Go toolchain pin')
  const source = readFileSync(
    path.join(directory, 'internal/client/client.go'),
    'utf8',
  )
  const declarations = [...source.matchAll(/^var Version = "([^"]+)"$/gm)]
  assert.equal(declarations.length, 1, 'Expected one source version')
  const version = declarations[0][1]
  assert.match(version, STABLE_VERSION, 'Source version must be stable X.Y.Z')
  for (const file of DOCUMENTS) {
    const filename = path.join(directory, file)
    assert.ok(lstatSync(filename).isFile(), `${file} must be a regular file`)
    assert.ok(
      readFileSync(filename, 'utf8').trim(),
      `${file} must not be empty`,
    )
  }
  const commit = git(['rev-parse', 'HEAD'])
  assert.match(commit, /^[0-9a-f]{40}$/, 'Expected full source commit')
  if (tag) {
    validateTag(tag, version)
    assert.equal(
      git(['rev-parse', `refs/tags/${tag}^{commit}`]),
      commit,
      'Tag does not point to HEAD',
    )
    if (env.GITHUB_ACTIONS === 'true') {
      assert.equal(
        env.GITHUB_REF,
        `refs/tags/${tag}`,
        'Select the existing tag when running the release workflow',
      )
      assert.equal(
        env.GITHUB_SHA,
        commit,
        'GitHub event SHA does not match checkout',
      )
    }
  }
  return { version, toolchain, commit }
}

export function parseChecksums(text) {
  const entries = text
    .trim()
    .split('\n')
    .map((line) => {
      const match = line.match(/^([0-9a-f]{64}) {2}([a-z0-9_.]+)$/)
      assert.ok(match, 'Malformed checksum entry')
      return [match[2], match[1]]
    })
  assert.deepEqual(
    entries.map(([name]) => name).sort(),
    TARGETS.map((target) => target.archive).sort(),
    'Checksums must cover exactly the six release archives, once each',
  )
  return new Map(entries)
}

export function validateMembers(members, binary) {
  assert.deepEqual(
    [...members].sort(),
    [...DOCUMENTS, binary].sort(),
    'Archive must contain only its binary and the three required documents',
  )
}

export function validateBuildInfo(info, target, source) {
  assert.equal(
    info.GoVersion,
    source.toolchain,
    'Binary toolchain differs from go.mod',
  )
  assert.equal(info.Path, `${MODULE}/cmd/dcs`, 'Unexpected binary package')
  assert.equal(info.Main.Path, MODULE, 'Unexpected binary module')
  const settings = Object.fromEntries(
    info.Settings.map(({ Key, Value }) => [Key, Value]),
  )
  for (const [key, value] of Object.entries({
    GOOS: target.os,
    GOARCH: target.arch,
    CGO_ENABLED: '0',
    '-trimpath': 'true',
    'vcs.revision': source.commit,
    'vcs.modified': 'false',
  })) {
    assert.equal(settings[key], value, `Unexpected binary setting: ${key}`)
  }
}

function archiveCommand(archive, member) {
  return archive.endsWith('.zip')
    ? ['unzip', member ? ['-p', archive, member] : ['-Z', '-1', archive]]
    : ['tar', member ? ['-xOf', archive, member] : ['-tf', archive]]
}

export function checkArchives({
  directory = process.cwd(),
  dist = 'dist',
  tag = '',
} = {}) {
  const source = checkSource({ directory, tag })
  const output = path.resolve(directory, dist)
  const metadata = JSON.parse(
    readFileSync(path.join(output, 'metadata.json'), 'utf8'),
  )
  assert.equal(
    metadata.commit,
    source.commit,
    'Package metadata must match source HEAD',
  )
  if (tag) {
    assert.equal(
      metadata.version,
      source.version,
      'Package version must match source',
    )
  } else {
    assert.match(
      metadata.version,
      /-snapshot$/,
      'Untagged builds must be marked as snapshots',
    )
  }
  const checksums = parseChecksums(
    readFileSync(path.join(output, 'checksums.txt'), 'utf8'),
  )
  const scratch = mkdtempSync(path.join(tmpdir(), 'dcs-archive-check-'))
  try {
    const configDirectory = path.join(scratch, 'config')
    mkdirSync(configDirectory)
    writeFileSync(
      path.join(configDirectory, 'version-check.json'),
      JSON.stringify({
        latest_version: source.version,
        checked_at: new Date().toISOString(),
      }),
    )
    for (const target of TARGETS) {
      const archive = path.join(output, target.archive)
      assert.ok(lstatSync(archive).isFile(), 'Archive must be a regular file')
      assert.equal(
        createHash('sha256').update(readFileSync(archive)).digest('hex'),
        checksums.get(target.archive),
        `Checksum mismatch: ${target.archive}`,
      )
      const [listCommand, listArgs] = archiveCommand(archive)
      validateMembers(
        run(listCommand, listArgs).toString().trim().split('\n'),
        target.binary,
      )
      for (const file of DOCUMENTS) {
        const [command, args] = archiveCommand(archive, file)
        assert.deepEqual(
          run(command, args),
          readFileSync(path.join(directory, file)),
          `Archive document differs from source: ${file}`,
        )
      }
      const binary = path.join(
        scratch,
        `${target.os}-${target.arch}-${target.binary}`,
      )
      const [command, args] = archiveCommand(archive, target.binary)
      writeFileSync(binary, run(command, args), { flag: 'wx', mode: 0o755 })
      const info = JSON.parse(
        run('go', ['version', '-m', '-json', binary]).toString(),
      )
      validateBuildInfo(info, target, source)
      const nativeArch = process.arch === 'x64' ? 'amd64' : process.arch
      const nativeOS =
        process.platform === 'win32' ? 'windows' : process.platform
      if (target.os === nativeOS && target.arch === nativeArch) {
        const version = JSON.parse(
          run(binary, ['version', '--json'], {
            env: {
              PATH: process.env.PATH,
              DCS_CONFIG_DIR: configDirectory,
              DCS_API_KEY: '',
              DCS_API_URL: 'http://127.0.0.1:1',
              NO_COLOR: '1',
            },
          }).toString(),
        )
        assert.equal(
          version.version,
          metadata.version,
          'Native CLI reports wrong version',
        )
        assert.equal(
          version.commit,
          source.commit,
          'Native CLI reports wrong commit',
        )
        assert.equal(
          version.go,
          source.toolchain,
          'Native CLI reports wrong toolchain',
        )
      }
    }
  } finally {
    rmSync(scratch, { recursive: true })
  }
  return {
    ...source,
    packagedVersion: metadata.version,
    archives: TARGETS.length,
    documentsPerArchive: DOCUMENTS.length,
  }
}

if (
  process.argv[1] &&
  path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)
) {
  try {
    const [command, first = '', second = '', ...extra] = process.argv.slice(2)
    assert.equal(extra.length, 0, 'Too many arguments')
    let result
    if (command === 'source' && !second) {
      result = checkSource({ tag: first })
    } else if (command === 'archives' && first) {
      result = checkArchives({ dist: first, tag: second })
    } else {
      throw new Error(
        'Usage: node scripts/release-check.mjs source [tag] | archives <dist> [tag]',
      )
    }
    console.log(JSON.stringify(result, null, 2))
  } catch (error) {
    console.error(`CLI release check: ${error.message}`)
    process.exitCode = 1
  }
}
