# justfile for go-htpdate-win (windows/powershell)_

set shell := ["powershell", "-NoProfile", "-Command"]

default:
    just build

build:
    Write-Host "Building go-htpdate-win.exe..."
    go build -ldflags="-s -w" -buildvcs=false -o go-htpdate-win.exe go-htpdate-win.go

clean:
    Write-Host "Cleaning build artifacts..."
    if (Test-Path go-htpdate-win.exe) { Remove-Item go-htpdate-win.exe }

#requires upx
pack:
    &.\upx go-htpdate-win.exe
