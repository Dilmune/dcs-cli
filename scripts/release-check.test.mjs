import assert from 'node:assert/strict'
import { spawnSync } from 'node:child_process'
import { createHash } from 'node:crypto'
import {
  mkdirSync,
  mkdtempSync,
  readFileSync,
  rmSync,
  writeFileSync,
} from 'node:fs'
import { tmpdir } from 'node:os'
import path from 'node:path'
import test from 'node:test'
import {
  checkSource,
  parseChecksums,
  validateBuildInfo,
  validateMembers,
  validateTag,
} from './release-check.mjs'

const COMMIT = 'a'.repeat(40)
const ARCHIVES = ['darwin', 'linux', 'windows'].flatMap((os) =>
  ['amd64', 'arm64'].map(
    (arch) => `dcs_${os}_${arch}${os === 'windows' ? '.zip' : '.tar.gz'}`,
  ),
)

const CI_WORKFLOW = readFileSync(
  new URL('../.github/workflows/ci.yml', import.meta.url),
  'utf8',
)
const RELEASE_WORKFLOW = readFileSync(
  new URL('../.github/workflows/release.yml', import.meta.url),
  'utf8',
)

test('keeps source execution read-only and publishing explicitly gated', () => {
  assert.match(CI_WORKFLOW, /permissions:\n {2}contents: read/)
  assert.doesNotMatch(CI_WORKFLOW, /pull_request_target|self-hosted|secrets\./)
  const writer = RELEASE_WORKFLOW.split('\n  publish:\n')[1]
  assert.ok(writer)
  assert.match(RELEASE_WORKFLOW, /default: false/)
  assert.match(writer, /inputs\.publish/)
  assert.match(writer, /vars\.CLI_PUBLIC_RELEASE_ENABLED == 'true'/)
  assert.match(writer, /environment: cli-release/)
  assert.doesNotMatch(
    writer,
    /actions\/checkout|goreleaser release|go (build|test|run)|HOMEBREW_TAP_TOKEN/,
  )
  assert.match(writer, /--verify-tag/)
  assert.doesNotMatch(writer, /--clobber|release delete/)
})

test('passes the immutable artifact ID across reusable workflow boundaries', () => {
  assert.match(
    CI_WORKFLOW,
    /value: \$\{\{ jobs\.packages\.outputs\.artifact-id }}/,
  )
  assert.match(
    CI_WORKFLOW,
    /artifact-id: \$\{\{ steps\.upload\.outputs\.artifact-id }}/,
  )
  assert.match(
    CI_WORKFLOW,
    /name: dcs-packages-\$\{\{ github\.sha }}-\$\{\{ github\.run_attempt }}/,
  )
  assert.match(
    RELEASE_WORKFLOW,
    /artifact-ids: \$\{\{ needs\.verify\.outputs\.artifact-id }}/,
  )
  assert.match(RELEASE_WORKFLOW, /merge-multiple: true/)
  assert.doesNotMatch(CI_WORKFLOW, /overwrite: true/)
  assert.doesNotMatch(RELEASE_WORKFLOW, /github-token:|run-id:|repository:/)
})

test('artifact download guard refuses missing, multiple or malformed IDs', () => {
  const guard = RELEASE_WORKFLOW.match(
    /run: '(\[\[.*CLI_ARTIFACT_ID.*\]\])'/,
  )?.[1]
  assert.ok(guard, 'Artifact ID must be validated before download')
  for (const value of ['', '0', '01', '1,2', '12oops', '1\n2', '123']) {
    const result = spawnSync('bash', ['-c', guard], {
      env: { PATH: process.env.PATH, CLI_ARTIFACT_ID: value },
    })
    assert.ifError(result.error)
    assert.equal(result.status === 0, value === '123')
  }
})

function runPublisher(t, options = {}) {
  const script = RELEASE_WORKFLOW.match(
    /        run: \|\n((?: {10}[^\n]*(?:\n|$))+)/,
  )?.[1].replace(/^ {10}/gm, '')
  assert.ok(script, 'Run the actual publication step from the workflow')
  const directory = mkdtempSync(path.join(tmpdir(), 'dcs-publisher-test-'))
  t.after(() => rmSync(directory, { recursive: true }))
  const assets = path.join(directory, 'assets')
  mkdirSync(assets)
  const checksums = ARCHIVES.map((name) => {
    const content = `verified fixture: ${name}\n`
    writeFileSync(path.join(assets, name), content)
    return `${createHash('sha256').update(content).digest('hex')}  ${name}`
  })
  writeFileSync(path.join(assets, 'checksums.txt'), `${checksums.join('\n')}\n`)
  if (options.corruptAsset) {
    writeFileSync(path.join(assets, ARCHIVES[0]), 'changed after verification')
  }
  const callsFile = path.join(directory, 'calls.jsonl')
  writeFileSync(callsFile, '')
  const mock = path.join(directory, 'gh.cjs')
  writeFileSync(mock, `
    const { appendFileSync, readFileSync } = require('node:fs')
    const args = process.argv.slice(2)
    const options = JSON.parse(process.env.RELEASE_TEST_OPTIONS)
    const callsFile = process.env.RELEASE_TEST_CALLS
    appendFileSync(callsFile, JSON.stringify(args) + '\\n')
    const calls = readFileSync(callsFile, 'utf8').trim().split('\\n').map(JSON.parse)
    const tagEndpoint = 'repos/fixture/release/commits/v3.7.2'
    if (args.join(' ') === 'api --paginate --slurp repos/fixture/release/releases?per_page=100') {
      process.stdout.write(options.lookupBody ?? '[[]]')
      process.exitCode = options.lookupStatus ?? 0
    } else if (args.join(' ') === 'api ' + tagEndpoint + ' --jq .sha') {
      const created = calls.some((call) => call[0] === 'release' && call[1] === 'create')
      process.stdout.write(created ? (options.finalCommit ?? options.commit) : (options.initialCommit ?? options.commit))
      process.exitCode = created ? (options.finalTagStatus ?? 0) : (options.initialTagStatus ?? 0)
    } else if (args[0] === 'release' && args[1] === 'create') {
      process.exitCode = options.createStatus ?? 0
    } else if (args[0] === 'release' && args[1] === 'edit') {
      process.exitCode = options.publishStatus ?? 0
    } else {
      process.stderr.write('Unexpected gh command: ' + JSON.stringify(args))
      process.exitCode = 97
    }
  `)
  const result = spawnSync('bash', ['-c', `
    gh() { "$RELEASE_TEST_NODE" "$RELEASE_TEST_GH" "$@"; }
    ${script}
  `], {
    cwd: assets,
    env: {
      PATH: process.env.PATH,
      GH_REPO: 'fixture/release',
      CLI_TAG: 'v3.7.2',
      CLI_SHA: COMMIT,
      RELEASE_TEST_NODE: process.execPath,
      RELEASE_TEST_GH: mock,
      RELEASE_TEST_CALLS: callsFile,
      RELEASE_TEST_OPTIONS: JSON.stringify({ commit: COMMIT, ...options }),
    },
    encoding: 'utf8',
    timeout: 10_000,
  })
  assert.ifError(result.error)
  const calls = readFileSync(callsFile, 'utf8').trim().split('\n').map(JSON.parse)
  return { ...result, calls, mutations: calls.filter((call) => call[0] === 'release') }
}

test('stages a draft, rechecks the source tag and publishes separately', (t) => {
  const result = runPublisher(t, {
    lookupBody: JSON.stringify([
      [{ tag_name: 'v3.7.1', draft: false }],
      [{ tag_name: 'v3.8.0', draft: true }],
    ]),
  })
  assert.equal(result.status, 0, result.stderr)
  assert.deepEqual(result.mutations.map((call) => call[1]), ['create', 'edit'])
  const [create, publish] = result.mutations
  assert.ok(create.includes('--draft'), 'gh must not implicitly publish and clean up')
  assert.ok(create.includes('--verify-tag'))
  assert.deepEqual(create.slice(3, 10), ['checksums.txt', ...ARCHIVES])
  assert.ok(create[create.indexOf('--notes') + 1].includes(
    `https://github.com/Dilmune/dcs-cli/blob/${COMMIT}/CHANGELOG.md`,
  ))
  assert.deepEqual(publish, ['release', 'edit', 'v3.7.2', '--draft=false'])
  const createIndex = result.calls.indexOf(create)
  assert.deepEqual(result.calls.slice(createIndex + 1), [
    ['api', 'repos/fixture/release/commits/v3.7.2', '--jq', '.sha'],
    publish,
  ])
})

for (const draft of [false, true]) {
  for (const laterPage of [false, true]) {
    test(`refuses an existing ${draft ? 'draft' : 'release'} on ${laterPage ? 'a later' : 'the first'} page`, (t) => {
      const pages = [[{ tag_name: 'v3.7.2', draft }]]
      if (laterPage) pages.unshift([{ tag_name: 'v3.7.1', draft: false }])
      const result = runPublisher(t, { lookupBody: JSON.stringify(pages) })
      assert.notEqual(result.status, 0)
      assert.deepEqual(result.mutations, [])
    })
  }
}

for (const [name, options] of [
  ['a lost lookup response', { lookupStatus: 1, lookupBody: '' }],
  ['a failed lookup with apparently valid output', { lookupStatus: 1, lookupBody: '[[]]' }],
  ['truncated lookup JSON', { lookupBody: '[[' }],
  ['an empty lookup response', { lookupBody: '' }],
  ['missing lookup pages', { lookupBody: '[]' }],
  ['an API error body', { lookupBody: '{"message":"Not Found"}' }],
  ['a null lookup page', { lookupBody: '[null]' }],
  ['a release without a tag', { lookupBody: '[[{}]]' }],
  ['a failed initial tag lookup with apparently valid output', { initialTagStatus: 1 }],
  ['an initial source tag mismatch', { initialCommit: 'b'.repeat(40) }],
]) {
  test(`refuses publication after ${name}`, (t) => {
    const result = runPublisher(t, options)
    assert.notEqual(result.status, 0)
    assert.deepEqual(result.mutations, [])
  })
}

for (const [name, options] of [
  ['draft creation or upload failure', { createStatus: 1 }],
  ['a moved source tag after upload', { finalCommit: 'b'.repeat(40) }],
  ['a lost final tag lookup response', { finalTagStatus: 1, finalCommit: '' }],
  ['a failed final tag lookup with apparently valid output', { finalTagStatus: 1 }],
]) {
  test(`stops without cleanup or publication after ${name}`, (t) => {
    const result = runPublisher(t, options)
    assert.notEqual(result.status, 0)
    assert.deepEqual(result.mutations.map((call) => call[1]), ['create'])
    assert.ok(result.mutations[0].includes('--draft'))
  })
}

test('an ambiguous publish failure never triggers cleanup or another mutation', (t) => {
  const result = runPublisher(t, { publishStatus: 1 })
  assert.notEqual(result.status, 0)
  assert.deepEqual(result.mutations.map((call) => call[1]), ['create', 'edit'])
  assert.ok(result.mutations[0].includes('--draft'))
  assert.equal(result.calls.at(-1), result.mutations.at(-1))
})

test('refuses changed artifacts before any publication', (t) => {
  const result = runPublisher(t, { corruptAsset: true })
  assert.notEqual(result.status, 0)
  assert.deepEqual(result.mutations, [])
})

test('pins every external action to a full commit SHA', () => {
  const setup = readFileSync(
    new URL('../.github/actions/setup-go/action.yml', import.meta.url),
    'utf8',
  )
  for (const match of (CI_WORKFLOW + RELEASE_WORKFLOW + setup).matchAll(
    /uses: ([^\s]+)/g,
  )) {
    if (!match[1].startsWith('./')) {
      assert.match(match[1], /^[\w/-]+@[a-f0-9]{40}$/)
    }
  }
})

test('uses commit-derived binary timestamps as well as build metadata', () => {
  const config = readFileSync(
    new URL('../.goreleaser-public.yaml', import.meta.url),
    'utf8',
  )
  assert.match(config, /mod_timestamp: "\{\{ \.CommitTimestamp }}"/)
  assert.match(config, /BuildDate=\{\{ \.CommitDate }}/)
  assert.doesNotMatch(config, /\{\{ \.Date }}/)
  assert.match(config, /release:\n {2}disable: true/)
  assert.doesNotMatch(config, /^brews:|^homebrew_casks:/m)
})

function fixture() {
  const directory = mkdtempSync(path.join(tmpdir(), 'dcs-release-source-test-'))
  mkdirSync(path.join(directory, 'internal/client'), { recursive: true })
  const save = (file, content) =>
    writeFileSync(path.join(directory, file), content)
  save(
    'go.mod',
    'module github.com/dilmune/dcs-cli\n\ngo 1.25.0\n\ntoolchain go1.26.8\n',
  )
  save('internal/client/client.go', 'package client\n\nvar Version = "3.7.2"\n')
  for (const file of ['LICENSE', 'README.md', 'THIRD_PARTY_NOTICES.md']) {
    save(file, 'fixture\n')
  }
  const replies = {
    'rev-parse --show-toplevel': directory,
    'status --porcelain --untracked-files=all': '',
    'rev-parse HEAD': COMMIT,
    'rev-parse refs/tags/v3.7.2^{commit}': COMMIT,
  }
  const env = {
    GITHUB_ACTIONS: 'true',
    GITHUB_REF: 'refs/tags/v3.7.2',
    GITHUB_SHA: COMMIT,
  }
  const run = (tag = 'v3.7.2') =>
    checkSource({
      directory,
      tag,
      env,
      git: (args) => {
        assert.ok(
          Object.hasOwn(replies, args.join(' ')),
          'Unexpected Git command',
        )
        return replies[args.join(' ')]
      },
    })
  return { directory, replies, env, save, run }
}

test('accepts a stable matching source tag', () => {
  assert.doesNotThrow(() => validateTag('v3.7.2', '3.7.2'))
  const result = fixture().run()
  assert.equal(result.commit, COMMIT)
  assert.equal(result.toolchain, 'go1.26.8')
})

for (const tag of [
  'cli/v3.7.2',
  'v3.7.1',
  'v03.7.2',
  'v3.7.2-beta',
  'v3.7.2\ninjected',
  '--help',
]) {
  test(`rejects a mismatching or unsafe tag: ${JSON.stringify(tag)}`, () => {
    assert.throws(() => validateTag(tag, '3.7.2'), /exactly match/)
  })
}

test('rejects non-stable source versions', () => {
  for (const version of ['03.7.2', '3.7.2-beta', 'latest']) {
    assert.throws(() => validateTag(`v${version}`, version), /stable/)
  }
})

test('rejects a monorepo checkout', () => {
  const f = fixture()
  f.replies['rev-parse --show-toplevel'] = path.dirname(f.directory)
  assert.throws(() => f.run(), /standalone CLI repository/)
})

test('rejects dirty inputs and a tag pointing elsewhere', () => {
  const f = fixture()
  f.replies['status --porcelain --untracked-files=all'] = ' M go.mod'
  assert.throws(() => f.run(), /clean/)
  f.replies['status --porcelain --untracked-files=all'] = ''
  f.replies['rev-parse refs/tags/v3.7.2^{commit}'] = 'b'.repeat(40)
  assert.throws(() => f.run(), /point to HEAD/)
})

test('rejects a branch dispatch and a different event commit', () => {
  const f = fixture()
  f.env.GITHUB_REF = 'refs/heads/master'
  assert.throws(() => f.run(), /Select the existing tag/)
  f.env.GITHUB_REF = 'refs/tags/v3.7.2'
  f.env.GITHUB_SHA = 'b'.repeat(40)
  assert.throws(() => f.run(), /event SHA/)
})

test('snapshot preflight still requires clean standalone source and notices', () => {
  const f = fixture()
  f.env.GITHUB_REF = 'refs/heads/master'
  assert.equal(f.run('').version, '3.7.2')
  f.save('THIRD_PARTY_NOTICES.md', '')
  assert.throws(() => f.run(''), /must not be empty/)
})

test('rejects a private module or missing pinned toolchain', () => {
  const f = fixture()
  f.save('go.mod', 'module dcs/apps/dcs-cli\n')
  assert.throws(() => f.run(), /Wrong Go module/)
  f.save('go.mod', 'module github.com/dilmune/dcs-cli\n')
  assert.throws(() => f.run(), /toolchain pin/)
})

test('requires exactly six unique, safe checksum names', () => {
  const digest = 'a'.repeat(64)
  const entries = ARCHIVES.map((name) => `${digest}  ${name}`)
  assert.equal(parseChecksums(`${entries.join('\n')}\n`).size, 6)
  for (const lines of [
    entries.slice(1),
    [...entries, entries[0]],
    [entries[0], ...entries.slice(0, 5)],
    [...entries.slice(1), `${digest}  ../private.txt`],
    [...entries.slice(1), `bad  ${ARCHIVES[0]}`],
  ]) {
    assert.throws(() => parseChecksums(lines.join('\n')))
  }
})

test('requires each archive binary and exactly three documents', () => {
  const members = ['LICENSE', 'README.md', 'THIRD_PARTY_NOTICES.md', 'dcs']
  assert.doesNotThrow(() => validateMembers(members, 'dcs'))
  for (const files of [
    members.slice(1),
    [...members, '.env'],
    [...members, 'dcs'],
    members.map((name) => `../${name}`),
  ]) {
    assert.throws(
      () => validateMembers(files, 'dcs'),
      /Archive must contain only/,
    )
  }
  assert.throws(() => validateMembers(members, 'dcs.exe'))
})

test('checks every binary platform, toolchain and source revision', () => {
  const info = {
    GoVersion: 'go1.26.8',
    Path: 'github.com/dilmune/dcs-cli/cmd/dcs',
    Main: { Path: 'github.com/dilmune/dcs-cli' },
    Settings: Object.entries({
      GOOS: 'linux',
      GOARCH: 'arm64',
      CGO_ENABLED: '0',
      '-trimpath': 'true',
      'vcs.revision': COMMIT,
      'vcs.modified': 'false',
    }).map(([Key, Value]) => ({ Key, Value })),
  }
  const target = { os: 'linux', arch: 'arm64' }
  const source = { toolchain: 'go1.26.8', commit: COMMIT }
  assert.doesNotThrow(() => validateBuildInfo(info, target, source))
  for (const setting of info.Settings) {
    const invalid = structuredClone(info)
    invalid.Settings.find(({ Key }) => Key === setting.Key).Value = 'wrong'
    assert.throws(
      () => validateBuildInfo(invalid, target, source),
      /Unexpected binary setting/,
    )
  }
  assert.throws(
    () => validateBuildInfo({ ...info, GoVersion: 'go1.25.0' }, target, source),
    /toolchain/,
  )
})
