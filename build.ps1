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

# 检查 winres.json 文件是否存在
$WinResFile = "winres/winres.json"
if (-not (Test-Path $WinResFile)) {
    Write-Host "错误：找不到 $WinResFile 文件" -ForegroundColor Red
    Read-Host "按任意键退出"
    exit 1
}

# 解析 winres.json 文件获取版本号
Write-Host "正在读取版本信息从 $WinResFile..." -ForegroundColor Yellow
try {
    $WinResContent = Get-Content $WinResFile -Raw | ConvertFrom-Json
    $Version = $WinResContent.RT_VERSION."#1"."0000".fixed.file_version
    Write-Host "从 winres.json 读取的版本号: $Version" -ForegroundColor Green
} catch {
    Write-Host "错误：解析 winres.json 文件失败: $_" -ForegroundColor Red
    Read-Host "按任意键退出"
    exit 1
}

# 验证版本号格式
if (-not $Version -or $Version -eq "") {
    Write-Host "错误：从 winres.json 中未找到有效的版本号" -ForegroundColor Red
    Read-Host "按任意键退出"
    exit 1
}

$BuildTime = Get-Date -Format "yyyy-MM-dd HH:mm:ss"

Write-Host "构建版本: $Version" -ForegroundColor Cyan
Write-Host "构建时间: $BuildTime" -ForegroundColor Cyan
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

# 通过 ldflags 传递版本信息
$LdFlags = @(
    "-X 'main.Version=$Version'",
    "-X 'main.BuildTime=$BuildTime'",
    "-s", # 省略符号表
    "-w"  # 省略DWARF调试信息
) -join " "

$BuildCommand = "go build -ldflags `"$LdFlags`" -o sc_mc.exe"
Write-Host "构建命令: $BuildCommand" -ForegroundColor Gray

Invoke-Expression $BuildCommand

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