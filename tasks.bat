@echo off
setlocal enabledelayedexpansion

REM go-mall tasks wrapper for PowerShell / CMD.
REM Avoids the ".sh associated with WSL bash -> hangs" problem.
REM Usage: .\tasks.bat start-all | stop-all | health | log user ...

set "BASH_EXE="
for %%P in (
    "C:\Program Files\Git\bin\bash.exe"
    "C:\Users\WZC\AppData\Local\Programs\Git\bin\bash.exe"
    "C:\Users\WZC\.workbuddy-ai\binaries\PortableGit\versions\1.3.0\bin\bash.exe"
) do (
    if exist %%P if not defined BASH_EXE set "BASH_EXE=%%~P"
)

if not defined BASH_EXE (
    echo [ERROR] Git Bash not found. Install Git for Windows or run tasks.sh from Git Bash.
    exit /b 1
)

REM Convert project root to forward-slash path for bash.
set "ROOT=%~dp0"
set "ROOT=%ROOT:\=/%"
"%BASH_EXE%" -c "cd '%ROOT%' && ./scripts/tasks.sh %*"
exit /b %ERRORLEVEL%
