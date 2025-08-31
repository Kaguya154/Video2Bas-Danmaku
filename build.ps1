# 设置程序名和输出目录
$AppName = "video2bas"
$SrcName = ""
$OutputDir = "build"
$ldflags = "-s -w"

# 创建输出目录（如果不存在）
if (-not (Test-Path $OutputDir)) {
    New-Item -ItemType Directory -Path $OutputDir | Out-Null
}

# 要编译的平台和架构组合
$targets = @(
    @{ GOOS = "windows"; GOARCH = "amd64" },
    @{ GOOS = "linux";   GOARCH = "amd64" },
    @{ GOOS = "darwin";  GOARCH = "amd64" }, # macOS Intel
    @{ GOOS = "darwin";  GOARCH = "arm64" }  # macOS Apple Silicon
)

foreach ($target in $targets) {
    $env:GOOS = $target.GOOS
    $env:GOARCH = $target.GOARCH
    $env:CGO_ENABLED = "0"

    # 构造文件名：myapp-windows-amd64.exe 或 myapp-linux-arm
    $ext = if ($env:GOOS -eq "windows") { ".exe" } else { "" }
    $outputFile = "$OutputDir/$AppName-$env:GOOS-$env:GOARCH$ext"

    Write-Host "🔧 Building $outputFile ..."
    go build -ldflags="$ldflags" -o $outputFile $SrcName

    if ($LASTEXITCODE -ne 0) {
        Write-Warning "❌ Failed to build $outputFile"
    }
}

# 清理环境变量
Remove-Item Env:GOOS, Env:GOARCH, Env:CGO_ENABLED

Write-Host "`n✅ All done. Output in ./$OutputDir/"
