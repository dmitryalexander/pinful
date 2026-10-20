param([switch]$NoBrowser)
$ErrorActionPreference = 'Stop'
Set-Location -LiteralPath $PSScriptRoot
docker compose up -d --build
if ($LASTEXITCODE -ne 0) { throw 'Startup failed. See the Docker output above.' }
if (!$NoBrowser) {
    $config = docker compose config --format json | ConvertFrom-Json
    $port = $config.services.pinful.ports[0].published
    Start-Process ('http://localhost:' + $port + '/?pt_page=lists')
}
