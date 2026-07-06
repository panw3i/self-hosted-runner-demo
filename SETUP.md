# Self-Hosted Runner 配置记录

**日期**: 2026-07-06
**主机**: WHYDN4C8G-lAN9P (Windows 11, 10.0.26200 x64)
**仓库**: https://github.com/panw3i/self-hosted-runner-demo
**账户**: panw3i

---

## 1. 环境准备

### 网络验证

```powershell
Test-NetConnection -ComputerName github.com -Port 443
# TcpTestSucceeded: True ✅ 无需公网入口，Windows 主动连 GitHub 即可
```

### 工具安装

| 工具 | 版本 | 安装方式 |
|------|------|----------|
| Git | 2.55.0 | `winget install Git.Git` |
| Go | 1.26.4 | 已预装 `C:\Program Files\Go\bin` |
| Node.js | 24.18.0 | 已预装 `C:\Program Files\nodejs` |
| GitHub CLI (gh) | 2.96.0 | 已预装 `C:\Program Files\GitHub CLI` |

### PATH 配置

工具分散在不同目录，shell 默认 PATH 没包含，手动加入系统 PATH：

```powershell
[System.Environment]::SetEnvironmentVariable('Path',
  'C:\Program Files\Git\cmd;C:\Program Files\GitHub CLI;C:\Program Files\Go\bin;C:\Program Files\nodejs;'
  + [System.Environment]::GetEnvironmentVariable('Path', 'Machine'),
  'Machine')
```

### Git 配置

```powershell
git config --global user.email "panw3i@users.noreply.github.com"
git config --global user.name "panw3i"
gh auth setup-git   # 用 gh token 作为 git credential
```

---

## 2. 安装 Runner

### 获取 Token

通过 gh API 获取短期 token（约 1 小时有效，不要提前保存）：

```powershell
$token = (gh api --method POST repos/panw3i/self-hosted-runner-demo/actions/runners/registration-token | ConvertFrom-Json).token
```

### 下载 & 解压

```powershell
mkdir C:\actions-runner
cd C:\actions-runner

# 查最新版本号
$latest = (Invoke-RestMethod 'https://api.github.com/repos/actions/runner/releases/latest').tag_name
# v2.335.1

$ProgressPreference = 'SilentlyContinue'
Invoke-WebRequest -Uri "https://github.com/actions/runner/releases/download/$latest/actions-runner-win-x64-$(($latest -replace 'v','')).zip" -OutFile actions-runner-win-x64.zip -UseBasicParsing
Expand-Archive -Path actions-runner-win-x64.zip -DestinationPath .
```

### 配置（无人值守 + 服务模式）

```powershell
.\config.cmd `
  --url https://github.com/panw3i/self-hosted-runner-demo `
  --token <TOKEN> `
  --name win-build-01 `
  --labels win-build,wails,windows `
  --unattended `
  --runasservice
```

### 配置参数说明

| 参数 | 值 | 说明 |
|------|-----|------|
| `--name` | `win-build-01` | Runner 标识名 |
| `--labels` | `win-build,wails,windows` | 用于 workflow 精准投递 |
| `--unattended` | | 无交互模式 |
| `--runasservice` | | 注册为 Windows 服务，开机自启 |

### 服务管理

```powershell
# 查看状态
Get-Service 'actions.runner.*'

# 启动/停止
Start-Service 'actions.runner.panw3i-self-hosted-runner-demo.win-build-01'
Stop-Service 'actions.runner.panw3i-self-hosted-runner-demo.win-build-01'
```

服务名格式: `actions.runner.<repo名>.<runner名>`

### 验证在线

```powershell
# 本机
Get-Service 'actions.runner.*' | Select Name, Status

# GitHub API 确认
gh api repos/panw3i/self-hosted-runner-demo/actions/runners |
  ConvertFrom-Json | Select -Expand runners |
  Where name -eq 'win-build-01' | Select name, status, os
# status: online ✅
```

---

## 3. 仓库与 Workflow

### 创建仓库

```powershell
gh repo create panw3i/self-hosted-runner-demo --public --description 'Demo repo for GitHub Actions self-hosted runner on Windows'
```

### 目录结构

```
self-hosted-runner-demo/
├── .github/
│   └── workflows/
│       ├── test.yml          # 基础连接测试
│       └── release.yml       # Go 构建 + Release
├── main.go                   # 简单 Go 程序
├── go.mod
└── SETUP.md                  # 本文档
```

### test.yml - 基础连通性测试

```yaml
name: Windows Build Test
on:
  workflow_dispatch:
jobs:
  test:
    runs-on: [self-hosted, win-build]
    steps:
      - uses: actions/checkout@v5
      - shell: powershell
        run: |
          whoami
          hostname
          go version
          node -v
          git --version
```

> ⚠️ 注意：使用 `actions/checkout@v5`，v4 已因 Node.js 20 弃用产生警告。

### release.yml - Go 构建 + GitHub Release

```yaml
name: Build and Release
on:
  push:
    tags: ["v*"]
  workflow_dispatch:

jobs:
  build:
    runs-on: [self-hosted, win-build]
    steps:
      - uses: actions/checkout@v5
      - shell: powershell
        run: |
          $version = if ($env:GITHUB_REF_NAME -match '^v') { $env:GITHUB_REF_NAME } else { "dev" }
          go build -ldflags "-X main.version=$version" -o build/demo-${version}-windows-amd64.exe .
      - uses: actions/upload-artifact@v4
        with:
          name: windows-build
          path: build/*.exe
          retention-days: 7

  release:
    needs: build
    runs-on: [self-hosted, win-build]
    if: startsWith(github.ref, 'refs/tags/v')
    permissions:
      contents: write
    steps:
      - uses: actions/download-artifact@v4
        with:
          name: windows-build
          path: build
      - env:
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        shell: powershell
        run: |
          $tag = $env:GITHUB_REF_NAME
          gh release create $tag build/*.exe --title "Release $tag" --notes "Build from self-hosted runner win-build-01"
```

---

## 4. 权限配置

**关键踩坑**: 默认 `GITHUB_TOKEN` 的 workflow 权限是 read-only，创建 Release 会 403。

需要通过 API 设置：

```powershell
'{ "default_workflow_permissions": "write" }' | gh api -X PUT repos/panw3i/self-hosted-runner-demo/actions/permissions/workflow --input -
```

同时 workflow 中需要声明：

```yaml
permissions:
  contents: write
```

---

## 5. 发版流程

```bash
# 1. 开发完成后打 tag
git tag v1.0.0

# 2. 推送 tag，自动触发构建 + Release
git push origin v1.0.0
```

GitHub 自动完成：
1. Runner 接收 job → 在 Windows 上 `go build` 编译 exe
2. 上传 artifact（保留 7 天）
3. 创建 GitHub Release 并附带 exe 下载

---

## 6. 注意事项

- **Windows runner 不适合 Docker**: 如果 workflow 依赖 Docker container actions，需要 Linux runner
- **Token 有效期**: Runner 注册 token 约 1 小时过期，现场获取现场用
- **Runner 服务**: 配置了 `--runasservice` 后开机自启，以 `NT AUTHORITY\NETWORK SERVICE` 运行
- **标签匹配**: workflow 用 `runs-on: [self-hosted, win-build]` 精准投递到这台机器
- **工作目录**: Runner 工作区在 `C:\actions-runner\_work\`，每次 job 会 checkout 到对应子目录
