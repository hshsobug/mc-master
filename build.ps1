# build.ps1 - UTF-8 编码
Write-Host "========================================" -ForegroundColor Green
Write-Host "           项目构建脚本" -ForegroundColor Green
Write-Host "========================================" -ForegroundColor Green
Write-Host ""

# 检查是否在项目目录
if (-not (Test-Path "go.mod")) {
    Write-Host "错误：请在包含 go.mod 文件的目录中运行此脚本" -ForegroundColor Red
    Read-Host "按任意键退出"
    exit 1
}

Write-Host "当前目录：$PWD"
Write-Host ""

# 切换 Go 版本
Write-Host "切换到 Go 1.20.14..." -ForegroundColor Yellow
g use 1.20.14
if ($LASTEXITCODE -ne 0) {
    Write-Host "错误：切换 Go 版本失败" -ForegroundColor Red
    Read-Host "按任意键退出"
    exit 1
}

Write-Host "当前 Go 版本："
go version
Write-Host ""

# 构建
Write-Host "开始构建..." -ForegroundColor Yellow
go build -o sc_mc.exe
if ($LASTEXITCODE -ne 0) {
    Write-Host "构建失败！" -ForegroundColor Red
    Read-Host "按任意键退出"
    exit 1
}

Write-Host "构建成功！输出文件：sc_mc.exe" -ForegroundColor Green
if (Test-Path "sc_mc.exe") {
    $file = Get-Item "sc_mc.exe"
    Write-Host "文件大小：$([math]::Round($file.Length/1KB, 2)) KB" -ForegroundColor Cyan
}

Write-Host ""
Write-Host "按 Enter 键退出..." -ForegroundColor Gray
Read-Host