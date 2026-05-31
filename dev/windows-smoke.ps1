param(
    [Parameter(Mandatory = $true)]
    [string]$BinDir,

    [string]$Endpoint = "127.0.0.1:7422",
    [string]$HttpAddress = "127.0.0.1:7423",
    [string]$RunDir = "C:\DevLogBusVerify\smoke"
)

$ErrorActionPreference = "Stop"

function Stop-DevLogBusProcesses {
    Get-Process devlogbusd -ErrorAction SilentlyContinue | Stop-Process -Force
    Get-Process devlogbus -ErrorAction SilentlyContinue | Stop-Process -Force
}

function Wait-HttpHealth {
    param([string]$BaseUrl)

    for ($i = 0; $i -lt 50; $i++) {
        try {
            $health = Invoke-RestMethod -Uri "$BaseUrl/api/health" -TimeoutSec 1
            if ($health.ok -eq $true) {
                return
            }
        } catch {
        }
        Start-Sleep -Milliseconds 200
    }
    throw "devlogbusd HTTP health did not become ready at $BaseUrl"
}

function Invoke-DevLogBusTail {
    param(
        [string]$Source,
        [string]$OutFile,
        [string]$ErrFile
    )

    $tail = Start-Process -FilePath "$BinDir\devlogbus.exe" `
        -ArgumentList @("tail", "--endpoint", $Endpoint, "--replay", "10", "--source", $Source) `
        -RedirectStandardOutput $OutFile `
        -RedirectStandardError $ErrFile `
        -PassThru `
        -WindowStyle Hidden
    Start-Sleep -Seconds 2
    if (-not $tail.HasExited) {
        $tail.Kill()
        $tail.WaitForExit()
    }

    $output = ""
    if (Test-Path $OutFile) {
        $output += Get-Content $OutFile -Raw
    }
    if (Test-Path $ErrFile) {
        $output += Get-Content $ErrFile -Raw
    }
    return $output
}

Stop-DevLogBusProcesses
Remove-Item -Recurse -Force $RunDir -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force $RunDir | Out-Null

Write-Output "versions"
& "$BinDir\devlogbusd.exe" version
& "$BinDir\devlogbus.exe" version

Write-Output "endpoint"
$defaultEndpoint = & "$BinDir\devlogbus.exe" endpoint
if ($LASTEXITCODE -ne 0) {
	exit $LASTEXITCODE
}
Write-Output $defaultEndpoint
if ($defaultEndpoint.Trim() -ne $Endpoint) {
	throw "default endpoint = $defaultEndpoint; want $Endpoint"
}

& "$BinDir\devlogbus.exe" tui --help | Out-Null
if ($LASTEXITCODE -ne 0) {
	exit $LASTEXITCODE
}

$daemon = Start-Process -FilePath "$BinDir\devlogbusd.exe" `
    -ArgumentList @("run", "--endpoint", $Endpoint, "--http", $HttpAddress, "--max-records", "100", "--echo=false") `
    -RedirectStandardOutput "$RunDir\devlogbusd.out" `
    -RedirectStandardError "$RunDir\devlogbusd.err" `
    -PassThru `
    -WindowStyle Hidden

try {
	Wait-HttpHealth "http://$HttpAddress"

	$about = Invoke-RestMethod -Uri "http://$HttpAddress/api/about"
	if ($about.broker.endpoint -ne $Endpoint) {
		throw "about endpoint = $($about.broker.endpoint); want $Endpoint"
	}
	$index = Invoke-WebRequest -Uri "http://$HttpAddress/" -UseBasicParsing
	if ($index.StatusCode -ne 200) {
		throw "UI index returned status $($index.StatusCode)"
	}
	$asset = [regex]::Match($index.Content, '"/assets/[^"]+')
	if (-not $asset.Success) {
		throw "UI index did not reference a bundled asset"
	}
	$assetPath = $asset.Value.TrimStart('"')
	$assetResponse = Invoke-WebRequest -Uri "http://$HttpAddress$assetPath" -UseBasicParsing
	if ($assetResponse.StatusCode -ne 200) {
		throw "UI asset $assetPath returned status $($assetResponse.StatusCode)"
	}

	& "$BinDir\devlogbus.exe" emit --endpoint $Endpoint --source win-cli-smoke --level warn --message "windows cli smoke" --attr host=parallels
    if ($LASTEXITCODE -ne 0) {
        exit $LASTEXITCODE
    }

    Start-Sleep -Milliseconds 300
    $tailOutput = Invoke-DevLogBusTail "win-cli-smoke" "$RunDir\tail.out" "$RunDir\tail.err"
    Write-Output $tailOutput
    if ($tailOutput -notmatch "windows cli smoke") {
        throw "CLI tail did not replay win-cli-smoke"
    }

    $record = @{
        level = "info"
        source = "chrome:windows-smoke"
        message = "windows browser tap shaped smoke"
        attrs = @{
            sourceGroup = "chrome:windows"
            tabTitle = "Windows Smoke"
            url = "http://windows.local/test"
        }
    } | ConvertTo-Json -Depth 5
    Invoke-RestMethod -Method Post -Uri "http://$HttpAddress/api/records" -ContentType "application/json" -Body $record | Out-Null

    $records = Invoke-RestMethod -Uri "http://$HttpAddress/api/records?source=chrome:windows-smoke&replay=10"
    if ($records[0].message -ne "windows browser tap shaped smoke") {
        throw "HTTP replay did not return Browser Tap-shaped record"
    }

    $stream = Start-Process -FilePath "curl.exe" `
        -ArgumentList @("-sS", "-N", "http://$HttpAddress/api/stream?source=chrome:windows-smoke&replay=1") `
        -RedirectStandardOutput "$RunDir\stream.out" `
        -RedirectStandardError "$RunDir\stream.err" `
        -PassThru `
        -WindowStyle Hidden
    Start-Sleep -Seconds 2
    if (-not $stream.HasExited) {
        $stream.Kill()
        $stream.WaitForExit()
    }
    $streamOutput = Get-Content "$RunDir\stream.out" -Raw
    Write-Output $streamOutput
    if ($streamOutput -notmatch "windows browser tap shaped smoke") {
        throw "SSE stream did not replay Browser Tap-shaped record"
    }

    Invoke-RestMethod -Method Delete -Uri "http://$HttpAddress/api/records/expunge?source=chrome:windows-smoke" | Out-Null
    & "$BinDir\devlogbus.exe" expunge --endpoint $Endpoint --source win-cli-smoke
    if ($LASTEXITCODE -ne 0) {
        exit $LASTEXITCODE
    }

    Write-Output "journal bridge unsupported check"
    $bridge = Start-Process -FilePath "$BinDir\devlogbus-journal-bridge.exe" `
        -ArgumentList @("run", "--endpoint", $Endpoint, "--once") `
        -RedirectStandardOutput "$RunDir\journal-bridge.out" `
        -RedirectStandardError "$RunDir\journal-bridge.err" `
        -PassThru `
        -WindowStyle Hidden
    $bridge.WaitForExit()
    $bridgeOutput = ""
    if (Test-Path "$RunDir\journal-bridge.out") {
        $bridgeOutput += Get-Content "$RunDir\journal-bridge.out" -Raw
    }
    if (Test-Path "$RunDir\journal-bridge.err") {
        $bridgeOutput += Get-Content "$RunDir\journal-bridge.err" -Raw
    }
    Write-Output $bridgeOutput
    if ($bridge.ExitCode -eq 0 -or $bridgeOutput -notmatch "journald bridge is only supported on linux") {
        throw "journal bridge did not report Linux-only support"
    }

    Write-Output "windows smoke passed"
} finally {
    if ($daemon -and -not $daemon.HasExited) {
        $daemon.Kill()
        $daemon.WaitForExit()
    }
    Stop-DevLogBusProcesses
}
