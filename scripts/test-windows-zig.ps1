param(
    [string]$Go = "go",
    [string]$Zig = "zig"
)

$ErrorActionPreference = "Stop"

function Resolve-Executable {
    param(
        [string]$Value,
        [string]$Name
    )

    if (Test-Path $Value) {
        return (Resolve-Path $Value).Path
    }

    $cmd = Get-Command $Value -ErrorAction SilentlyContinue
    if ($null -ne $cmd) {
        return $cmd.Source
    }

    throw "$Name not found: $Value"
}

$goExe = Resolve-Executable $Go "Go toolchain"
$zigExe = Resolve-Executable $Zig "Zig compiler"

$env:CC = "$zigExe cc -target x86_64-windows-gnu"
$env:CXX = "$zigExe c++ -target x86_64-windows-gnu"

& $goExe test ./...
