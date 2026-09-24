// Regenerates and verifies apps/dcs-cli/THIRD_PARTY_NOTICES.md, which ships
// inside every release archive. The archive checker requires the file to be
// present; nothing checked that its contents were true, and the Charm v2
// migration left it naming four modules the binary no longer contains.
//
//   node scripts/third-party-notices.mjs            regenerate the file
//   node scripts/third-party-notices.mjs --check    verify it, no writes
//
// Regeneration needs go-licenses (pinned below) and the module cache warm for
// all six release targets. The check needs neither: it reads the committed
// file and compares it against `go list -deps`, which is why it is cheap
// enough to run on every pull request.

import { execFileSync } from 'node:child_process'
import {
  existsSync,
  mkdtempSync,
  readFileSync,
  rmSync,
  writeFileSync,
} from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

export const GO_LICENSES = 'github.com/google/go-licenses/v2@v2.0.1'

// Every target .goreleaser-public.yaml builds. The dependency set differs per
// platform (wincred and x/sys/windows are Windows-only, godbus and termios are
// not), so the notices are the union and nothing platform-specific is dropped.
export const TARGETS = [
  ['linux', 'amd64'],
  ['linux', 'arm64'],
  ['darwin', 'amd64'],
  ['darwin', 'arm64'],
  ['windows', 'amd64'],
  ['windows', 'arm64'],
]

const MAIN_MODULE = 'github.com/dilmune/dcs-cli'
const NOTICE_NAMES = ['NOTICE', 'NOTICE.txt', 'NOTICE.md']
const RUNTIME_VENDORED = ['crypto', 'net', 'sys', 'text']
const FENCE = '````'
const SOURCES_HEADING = '## Dependency sources'
const TEXTS_HEADING = '## Dependency license and notice texts'
const RUNTIME_HEADING = '## Go runtime and vendored runtime dependencies'

const cliDir = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const noticesPath = path.join(cliDir, 'THIRD_PARTY_NOTICES.md')

function go(args, env = {}) {
  return execFileSync('go', args, {
    cwd: cliDir,
    encoding: 'utf8',
    maxBuffer: 64 * 1024 * 1024,
    env: { ...process.env, GOWORK: 'off', GOFLAGS: '-mod=readonly', ...env },
  })
}

function targetEnv(goos, goarch) {
  return { GOOS: goos, GOARCH: goarch, CGO_ENABLED: '0' }
}

export function toolchainVersion(goModText) {
  const pinned = /^toolchain go(\d+\.\d+\.\d+)$/m.exec(goModText)
  if (!pinned) {
    throw new Error('apps/dcs-cli/go.mod must pin a stable Go toolchain')
  }
  return pinned[1]
}

// A pseudo-version's identity in a license URL is its commit hash, not the
// v0.0.0-timestamp- prefix the module graph reports.
export function versionToken(version) {
  const pseudo = /-([0-9a-f]{12})$/.exec(version)
  return pseudo ? pseudo[1] : version
}

// The module graph for the union of all six targets, test dependencies
// included: the notices cover "the CLI and its tests", and testify, go-spew
// and go-difflib are linked into the test binaries CI runs.
export function parseModuleList(stdout) {
  const modules = new Map()
  for (const line of stdout.split('\n')) {
    const [modulePath, version] = line.split('\t')
    if (!modulePath || !version || modulePath === MAIN_MODULE) {
      continue
    }
    modules.set(modulePath, version)
  }
  return modules
}

export function linkedModules() {
  const modules = new Map()
  for (const [goos, goarch] of TARGETS) {
    const stdout = go(
      [
        'list',
        '-deps',
        '-test',
        '-f',
        '{{if .Module}}{{.Module.Path}}\t{{.Module.Version}}{{end}}',
        './...',
      ],
      targetEnv(goos, goarch),
    )
    for (const [modulePath, version] of parseModuleList(stdout)) {
      const seen = modules.get(modulePath)
      if (seen && seen !== version) {
        throw new Error(
          `${modulePath} resolves to ${seen} and ${version} across targets`,
        )
      }
      modules.set(modulePath, version)
    }
  }
  return new Map([...modules].sort(byName))
}

function byName(a, b) {
  return compareNames(a[0], b[0])
}

// go-licenses emits its inventory in case-insensitive path order, which puts
// go-keyring/internal/shellescape ahead of go-keyring/LICENSE.
export function compareNames(a, b) {
  const left = a.toLowerCase()
  const right = b.toLowerCase()
  if (left === right) {
    return a < b ? -1 : a > b ? 1 : 0
  }
  return left < right ? -1 : 1
}

function goLicensesBin() {
  const preset = process.env.GO_LICENSES_BIN
  if (preset) {
    return preset
  }
  const gobin = path.join(os.tmpdir(), 'dcs-cli-notices-tools')
  const bin = path.join(gobin, 'go-licenses')
  if (!existsSync(bin)) {
    execFileSync('go', ['install', GO_LICENSES], {
      encoding: 'utf8',
      env: { ...process.env, GOWORK: 'off', GOFLAGS: '', GOBIN: gobin },
      stdio: 'inherit',
    })
  }
  return bin
}

export function parseReport(stdout) {
  const libraries = []
  for (const line of stdout.split('\n')) {
    if (!line.trim()) {
      continue
    }
    const [name, version, licensePath, licenseURL, licenseName] =
      line.split('\t')
    if (name === MAIN_MODULE) {
      continue
    }
    if (!licensePath || !licenseURL) {
      throw new Error(`go-licenses found no license file for ${name}`)
    }
    // An unattributed dependency in a shipped archive is exactly what this
    // file exists to prevent, so an unclassified license stops the run.
    if (!licenseName || licenseName === 'Unknown') {
      throw new Error(
        `go-licenses could not determine the license of ${name} ${version} ` +
          `(${licensePath}); resolve it by hand before regenerating`,
      )
    }
    libraries.push({ name, version, licensePath, licenseURL })
  }
  return libraries
}

function collectLibraries(bin, goroot) {
  const scratch = mkdtempSync(path.join(os.tmpdir(), 'dcs-cli-notices-'))
  const template = path.join(scratch, 'report.tmpl')
  writeFileSync(
    template,
    '{{range .}}{{.Name}}\t{{.Version}}\t{{.LicensePath}}\t' +
      '{{.LicenseURL}}\t{{.LicenseName}}\n{{end}}',
  )
  const libraries = new Map()
  try {
    for (const [goos, goarch] of TARGETS) {
      const stdout = execFileSync(
        bin,
        ['report', './...', '--include_tests', '--template', template],
        {
          cwd: cliDir,
          encoding: 'utf8',
          maxBuffer: 64 * 1024 * 1024,
          env: {
            ...process.env,
            GOWORK: 'off',
            GOFLAGS: '-mod=readonly',
            GOROOT: goroot,
            ...targetEnv(goos, goarch),
          },
        },
      )
      for (const library of parseReport(stdout)) {
        const seen = libraries.get(library.name)
        if (seen && seen.version !== library.version) {
          throw new Error(
            `${library.name} reports ${seen.version} and ${library.version}`,
          )
        }
        libraries.set(library.name, library)
      }
    }
  } finally {
    rmSync(scratch, { recursive: true, force: true })
  }
  return [...libraries.values()].sort((a, b) => compareNames(a.name, b.name))
}

export function block(heading, filePath) {
  const body = readFileSync(filePath, 'utf8')
    .replace(/\r\n/g, '\n')
    .replace(/\s+$/, '')
  return [`### ${heading}`, '', `${FENCE}text`, body, FENCE, ''].join('\n')
}

function licenseFiles(library) {
  const files = [library.licensePath]
  const dir = path.dirname(library.licensePath)
  for (const notice of NOTICE_NAMES) {
    const candidate = path.join(dir, notice)
    if (existsSync(candidate)) {
      files.push(candidate)
    }
  }
  return files
}

export function header(goVersion) {
  return [
    '# Third-party notices',
    '',
    'This file preserves upstream license and notice texts collected for the CLI and',
    'its tests on macOS, Linux and Windows (AMD64 and ARM64). Dependency versions are',
    'pinned in `go.mod` and `go.sum`; this inventory was prepared with go-licenses',
    `${GO_LICENSES.split('@')[1]} and Go ${goVersion}. Not every dependency is linked into every platform's binary.`,
    '',
    'The upstream texts below govern their respective components. They do not grant a',
    "license to Dilmune's own code. The CLI's first-party license belongs in `LICENSE`.",
    '',
    "The inventory also includes the pinned Go runtime's LICENSE and PATENTS and the",
    'notices supplied with its vendored runtime dependencies. Compiler-tool-only',
    'dependencies and the optional BoringCrypto build are not included here.',
    '',
  ].join('\n')
}

function render(libraries, goVersion, goroot) {
  const sections = []
  for (const library of libraries) {
    for (const file of licenseFiles(library)) {
      sections.push({
        heading: `${library.name}/${path.basename(file)}`,
        file,
      })
    }
  }
  sections.sort((a, b) => compareNames(a.heading, b.heading))

  const runtime = [
    { heading: `Go ${goVersion}: LICENSE`, file: path.join(goroot, 'LICENSE') },
    { heading: `Go ${goVersion}: PATENTS`, file: path.join(goroot, 'PATENTS') },
  ]
  for (const vendored of RUNTIME_VENDORED) {
    for (const name of ['LICENSE', 'PATENTS']) {
      const relative = `src/vendor/golang.org/x/${vendored}/${name}`
      const file = path.join(goroot, relative)
      if (!existsSync(file)) {
        continue
      }
      runtime.push({ heading: `Go ${goVersion}: ${relative}`, file })
    }
  }

  return [
    header(goVersion),
    SOURCES_HEADING,
    '',
    ...libraries.map((l) => `- [${l.name}](${l.licenseURL})`),
    '',
    TEXTS_HEADING,
    '',
    ...sections.map((s) => block(s.heading, s.file)),
    RUNTIME_HEADING,
    '',
    ...runtime.map((s) => block(s.heading, s.file)),
  ]
    .join('\n')
    .replace(/\n+$/, '\n')
}

function generate() {
  const goVersion = toolchainVersion(
    readFileSync(path.join(cliDir, 'go.mod'), 'utf8'),
  )
  const goroot = go(['env', 'GOROOT']).trim()
  const running = go(['env', 'GOVERSION']).trim()
  if (running !== `go${goVersion}`) {
    throw new Error(
      `go.mod pins go${goVersion} but ${running} is selected; ` +
        'the runtime notices below would name the wrong release',
    )
  }
  const libraries = collectLibraries(goLicensesBin(), goroot)
  writeFileSync(noticesPath, render(libraries, goVersion, goroot))
  process.stdout.write(
    `THIRD_PARTY_NOTICES.md: ${libraries.length} dependency entries\n`,
  )
}

export function parseNotices(text) {
  const sources = []
  const headings = []
  let section = ''
  for (const line of text.split('\n')) {
    if (line.startsWith('## ')) {
      section = line
      continue
    }
    if (section === SOURCES_HEADING) {
      const entry = /^- \[([^\]]+)\]\(([^)]+)\)$/.exec(line)
      if (entry) {
        sources.push({ name: entry[1], url: entry[2] })
      }
      continue
    }
    if (section === TEXTS_HEADING && line.startsWith('### ')) {
      headings.push(line.slice(4))
    }
  }
  return { sources, headings }
}

// Two-way: no listed dependency that nothing links, no linked module that the
// file fails to attribute, and no version that has moved underneath the URL.
export function compareInventory(sources, headings, modules) {
  const problems = []
  const covered = new Set()
  for (const { name, url } of sources) {
    const owner = [...modules.keys()]
      .filter((m) => name === m || name.startsWith(`${m}/`))
      .sort((a, b) => b.length - a.length)[0]
    if (!owner) {
      problems.push(`listed but not linked by any target: ${name}`)
      continue
    }
    covered.add(owner)
    const token = versionToken(modules.get(owner))
    if (!url.includes(token)) {
      problems.push(
        `${name}: linked at ${modules.get(owner)}, license URL points elsewhere (${url})`,
      )
    }
    const prefix = `${name}/`
    const attributed = headings.some(
      (h) => h.startsWith(prefix) && !h.slice(prefix.length).includes('/'),
    )
    if (!attributed) {
      problems.push(`${name}: listed with no license text section`)
    }
  }
  for (const modulePath of modules.keys()) {
    if (!covered.has(modulePath)) {
      problems.push(`linked but unattributed: ${modulePath}`)
    }
  }
  return problems
}

function check() {
  const text = readFileSync(noticesPath, 'utf8')
  const goVersion = toolchainVersion(
    readFileSync(path.join(cliDir, 'go.mod'), 'utf8'),
  )
  const { sources, headings } = parseNotices(text)
  const problems = compareInventory(sources, headings, linkedModules())
  if (!text.includes(`### Go ${goVersion}: LICENSE`)) {
    problems.push(
      `runtime notices do not name go${goVersion}, the toolchain go.mod pins`,
    )
  }
  const scope = `${sources.length} entries against go list -deps for ${TARGETS.length} targets`
  if (problems.length === 0) {
    process.stdout.write(
      `THIRD_PARTY_NOTICES.md: ${scope}, module set matches.\n` +
        'Module identity and versions only; the license texts themselves are not re-read.\n',
    )
    return
  }
  for (const problem of problems) {
    process.stdout.write(`  ${problem}\n`)
  }
  process.stdout.write(
    `::error file=apps/dcs-cli/THIRD_PARTY_NOTICES.md::THIRD_PARTY_NOTICES.md ` +
      `does not match the modules the CLI links. Run: node apps/dcs-cli/scripts/third-party-notices.mjs\n`,
  )
  process.exitCode = 1
}

const invoked = process.argv[1] === fileURLToPath(import.meta.url)
if (invoked) {
  if (process.argv.includes('--check')) {
    check()
  } else {
    generate()
  }
}
