# Installs the DCS CLI on Windows from a GitHub release of Dilmune/dcs-cli.
#
#   irm https://raw.githubusercontent.com/Dilmune/dcs-cli/main/install.ps1 | iex
#
# Knobs, set before running:
#   $env:DCS_VERSION      release tag to install, for example v3.8.1 (default: latest)
#   $env:DCS_INSTALL_DIR  directory for dcs.exe (default: $env:LOCALAPPDATA\Programs\dcs)
#
# Runs on Windows PowerShell 5.1 and PowerShell 7. It never calls exit, because
# under irm | iex that closes the user's window; every failure is a throw.
# The installer runs inside functions, so iex leaves no variables behind.
# With $env:DCS_INSTALL_TEST set nothing runs, so a test can dot-source this
# file and call the functions directly.

# Stable tags only: [0-9] and \z because .NET's \d matches any Unicode digit and
# $ also matches before a trailing newline, -cmatch because -match ignores case.
function Test-DcsReleaseTag {
    param([string]$Tag)
    return $Tag -cmatch '^v[0-9]+\.[0-9]+\.[0-9]+\z'
}

# RuntimeInformation needs .NET Framework 4.7.1 or later under Windows PowerShell.
function Get-DcsOSArchitecture {
    $runtime = 'System.Runtime.InteropServices.RuntimeInformation' -as [type]
    if ($runtime) {
        return [string]$runtime::OSArchitecture
    }
    return ''
}

# OSArchitecture ignores the bitness of the process, so it goes first. A 32-bit
# PowerShell on 64-bit Windows reports x86 in PROCESSOR_ARCHITECTURE and the
# real architecture in PROCESSOR_ARCHITEW6432.
function Resolve-DcsArchitecture {
    param([string]$OSArchitecture, [string]$Wow64Architecture, [string]$ProcessArchitecture)
    $name = $OSArchitecture
    if (-not $name) { $name = $Wow64Architecture }
    if (-not $name) { $name = $ProcessArchitecture }
    if ($name -in 'X64', 'AMD64') { return 'amd64' }
    if ($name -eq 'Arm64') { return 'arm64' }
    throw "dcs install: unsupported architecture: '$name' (amd64 and arm64 only)"
}

# The release page redirects to /releases/tag/vX.Y.Z and is not rate limited
# like the API, so it goes first. Returns '' when neither gives a stable tag.
function Get-DcsLatestTag {
    param([string]$LatestUrl, [string]$ApiUrl)
    $marker = '/releases/tag/'
    try {
        $request = [System.Net.WebRequest]::Create($LatestUrl)
        $request.Method = 'HEAD'
        $request.AllowAutoRedirect = $false
        $request.UserAgent = 'dcs-install'
        $response = $request.GetResponse()
        try {
            $location = [string]$response.Headers['Location']
        } finally {
            $response.Close()
        }
        $index = $location.LastIndexOf($marker, [System.StringComparison]::Ordinal)
        if ($index -ge 0) {
            $tag = $location.Substring($index + $marker.Length)
            if (Test-DcsReleaseTag -Tag $tag) { return $tag }
        }
    } catch {
        Write-Verbose "dcs install: no release redirect from ${LatestUrl}: $($_.Exception.Message)"
    }
    try {
        $tag = [string](Invoke-RestMethod -Uri $ApiUrl -UseBasicParsing).tag_name
        if (Test-DcsReleaseTag -Tag $tag) { return $tag }
    } catch {
        Write-Verbose "dcs install: no release from ${ApiUrl}: $($_.Exception.Message)"
    }
    return ''
}

function Save-DcsDownload {
    param([string]$Uri, [string]$OutFile)
    try {
        Invoke-WebRequest -Uri $Uri -OutFile $OutFile -UseBasicParsing
    } catch {
        throw "dcs install: download failed: $Uri ($($_.Exception.Message))"
    }
}

# A missing line fails like a mismatch. The hashes compare without case because
# Get-FileHash prints upper-case hex and checksums.txt holds lower-case.
function Assert-DcsChecksum {
    param([string]$Archive, [string]$Checksums, [string]$Name)
    $expected = @(foreach ($line in Get-Content -LiteralPath $Checksums) {
        $fields = -split $line
        if ($fields.Count -eq 2 -and $fields[1] -ceq $Name) { $fields[0] }
    })
    if ($expected.Count -eq 0) {
        throw "dcs install: checksums.txt has no entry for $Name"
    }
    $actual = (Get-FileHash -LiteralPath $Archive -Algorithm SHA256).Hash
    if ($expected.Count -ne 1 -or $actual -ine $expected[0]) {
        throw "dcs install: checksum mismatch for ${Name}: expected $($expected -join ' '), got $actual"
    }
}

# Windows keeps a running executable open, so opening it for writing fails with
# a sharing violation, an IOException. Access denied is a different failure.
function Test-DcsFileLocked {
    param([string]$Path)
    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) { return $false }
    try {
        $stream = [System.IO.File]::Open($Path, [System.IO.FileMode]::Open, [System.IO.FileAccess]::ReadWrite, [System.IO.FileShare]::None)
        $stream.Close()
        return $false
    } catch [System.IO.IOException] {
        return $true
    } catch {
        return $false
    }
}

# Staging next to the target keeps the final replace a rename on one volume.
function Install-DcsExecutable {
    param([string]$Source, [string]$Directory)
    try {
        New-Item -ItemType Directory -Path $Directory -Force | Out-Null
    } catch {
        throw "dcs install: could not create ${Directory}: $($_.Exception.Message)"
    }
    $target = Join-Path $Directory 'dcs.exe'
    $staged = "$target.tmp"
    try {
        Copy-Item -LiteralPath $Source -Destination $staged -Force
    } catch {
        throw "dcs install: could not write to ${Directory}: $($_.Exception.Message)"
    }
    try {
        Move-Item -LiteralPath $staged -Destination $target -Force
    } catch {
        Remove-Item -LiteralPath $staged -Force -ErrorAction SilentlyContinue
        if (Test-DcsFileLocked -Path $target) {
            throw "dcs install: $target is in use; close every running dcs and run the installer again"
        }
        throw "dcs install: could not replace ${target}: $($_.Exception.Message)"
    }
    return $target
}

# Windows PowerShell can turn a line the binary writes to stderr, such as the
# update notice, into an error record that Stop would treat as fatal. The exit
# code is the verdict.
function Invoke-DcsVersion {
    param([string]$Path)
    $ErrorActionPreference = 'Continue'
    try {
        & $Path version
    } catch {
        throw "dcs install: installed binary failed to run: $($_.Exception.Message)"
    }
    if ($LASTEXITCODE -ne 0) {
        throw "dcs install: installed binary failed to run (exit code $LASTEXITCODE)"
    }
}

# Machine and User together are the PATH a new terminal starts with.
function Test-DcsDirectoryOnPath {
    param([string]$Directory)
    $wanted = $Directory.TrimEnd('\')
    foreach ($scope in 'User', 'Machine') {
        foreach ($entry in ([string][Environment]::GetEnvironmentVariable('Path', $scope)) -split ';') {
            if ($entry.TrimEnd('\') -ieq $wanted) { return $true }
        }
    }
    return $false
}

function Get-DcsPathCommand {
    param([string]$Directory)
    $quoted = $Directory.Replace("'", "''")
    return "[Environment]::SetEnvironmentVariable('Path', [Environment]::GetEnvironmentVariable('Path', 'User') + ';$quoted', 'User')"
}

function Install-DcsCli {
    $ErrorActionPreference = 'Stop'
    # The progress bar makes Invoke-WebRequest many times slower on Windows PowerShell.
    $ProgressPreference = 'SilentlyContinue'

    if ($PSVersionTable.PSEdition -eq 'Core' -and -not $IsWindows) {
        throw 'dcs install: install.ps1 is for Windows; use install.sh on Linux and macOS'
    }

    $repo = 'Dilmune/dcs-cli'
    $latestUrl = "https://github.com/$repo/releases/latest"
    $apiUrl = "https://api.github.com/repos/$repo/releases/latest"

    $installDir = $env:DCS_INSTALL_DIR
    if (-not $installDir) {
        if (-not $env:LOCALAPPDATA) {
            throw 'dcs install: LOCALAPPDATA is not set; set $env:DCS_INSTALL_DIR to choose a directory'
        }
        $installDir = Join-Path $env:LOCALAPPDATA 'Programs\dcs'
    }
    $installDir = $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($installDir)

    $arch = Resolve-DcsArchitecture -OSArchitecture (Get-DcsOSArchitecture) -Wow64Architecture $env:PROCESSOR_ARCHITEW6432 -ProcessArchitecture $env:PROCESSOR_ARCHITECTURE

    $previousProtocols = [Net.ServicePointManager]::SecurityProtocol
    $tmp = Join-Path ([System.IO.Path]::GetTempPath()) ('dcs-install-' + [guid]::NewGuid().ToString('N'))
    try {
        # Windows PowerShell 5.1 can default to TLS 1.0, which GitHub refuses.
        # PowerShell 7 negotiates on its own, and pinning it would drop TLS 1.3.
        if ($PSVersionTable.PSEdition -ne 'Core') {
            [Net.ServicePointManager]::SecurityProtocol = $previousProtocols -bor [Net.SecurityProtocolType]::Tls12
        }

        if ($env:DCS_VERSION) {
            if (-not (Test-DcsReleaseTag -Tag $env:DCS_VERSION)) {
                throw "dcs install: DCS_VERSION must look like vX.Y.Z, got: $env:DCS_VERSION"
            }
            $version = $env:DCS_VERSION
        } else {
            $version = Get-DcsLatestTag -LatestUrl $latestUrl -ApiUrl $apiUrl
            if (-not $version) {
                throw "dcs install: could not resolve the latest release tag from $latestUrl or $apiUrl (set `$env:DCS_VERSION='vX.Y.Z' to pin one)"
            }
        }

        $archive = "dcs_windows_$arch.zip"
        $base = "https://github.com/$repo/releases/download/$version"
        Write-Output "Installing dcs $version (windows/$arch) to $installDir"

        New-Item -ItemType Directory -Path $tmp | Out-Null
        $zip = Join-Path $tmp $archive
        $checksums = Join-Path $tmp 'checksums.txt'
        Save-DcsDownload -Uri "$base/$archive" -OutFile $zip
        Save-DcsDownload -Uri "$base/checksums.txt" -OutFile $checksums
        Assert-DcsChecksum -Archive $zip -Checksums $checksums -Name $archive

        $extracted = Join-Path $tmp 'extracted'
        try {
            Expand-Archive -LiteralPath $zip -DestinationPath $extracted -Force
        } catch {
            throw "dcs install: could not extract ${archive}: $($_.Exception.Message)"
        }
        $exe = Join-Path $extracted 'dcs.exe'
        if (-not (Test-Path -LiteralPath $exe -PathType Leaf)) {
            throw "dcs install: $archive does not contain dcs.exe"
        }

        $target = Install-DcsExecutable -Source $exe -Directory $installDir
        Invoke-DcsVersion -Path $target

        if (-not (Test-DcsDirectoryOnPath -Directory $installDir)) {
            Write-Output "$installDir is not on your PATH. Add it with this command, then open a new terminal:"
            Write-Output "  $(Get-DcsPathCommand -Directory $installDir)"
        }
    } finally {
        Remove-Item -LiteralPath $tmp -Recurse -Force -ErrorAction SilentlyContinue
        [Net.ServicePointManager]::SecurityProtocol = $previousProtocols
    }
}

if (-not $env:DCS_INSTALL_TEST) {
    Install-DcsCli
}
