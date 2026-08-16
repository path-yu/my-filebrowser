@echo off
REM ============================================================================
REM  FileBrowser Windows build launcher (thin, no logic).
REM  All build logic lives in build-core.ps1 (PowerShell) so we escape cmd's
REM  fragile parenthesis / escape / encoding / ERRORLEVEL hell.
REM
REM  Usage:
REM    build-windows                    default amd64, full FE + BE
REM    build-windows skipFrontend       skip pnpm build, reuse existing dist
REM    build-windows skipBackend        only frontend
REM    build-windows clean              rm output/, rm frontend/dist/*, go clean -cache
REM    build-windows arm64              full build for windows/arm64
REM    build-windows arm64 skipFrontend combine freely
REM ============================================================================
setlocal EnableExtensions EnableDelayedExpansion
set "PS1=%~dp0build-core.ps1"

REM  ---- Argument normalization -----------------------------------------------
REM  PS1 switches are "-SkipFrontend -SkipBackend -Clean" (with dash, PascalCase).
REM  This launcher also accepts the dash-less lowercase forms the user
REM  already typed in previous sessions (cmd style). Convert them here so
REM  the PS ValidateSet($Arch) never sees "skipFrontend" as a positional value.
REM  Arch tokens (amd64/arm64/386) remain positional.
set "ARGS="
:argv_loop
if "%~1"=="" goto :argv_done
set "T=%~1"
set "U=%T%"
call :toUPPER U
if "!U!"=="SKIPFRONTEND" ( set "ARGS=!ARGS! -SkipFrontend" ) else ^
if "!U!"=="SKIPBACKEND"  ( set "ARGS=!ARGS! -SkipBackend"  ) else ^
if "!U!"=="CLEAN"        ( set "ARGS=!ARGS! -Clean"        ) else ^
if "!U!"=="AMD64"        ( set "ARGS=!ARGS! amd64"         ) else ^
if "!U!"=="ARM64"        ( set "ARGS=!ARGS! arm64"         ) else ^
if "!U!"=="386"          ( set "ARGS=!ARGS! 386"           ) else (
    REM Anything else (e.g. already "-SkipFrontend") is passed through verbatim
    set "ARGS=!ARGS! !T!"
)
shift
goto :argv_loop
:argv_done

REM  ---- Execute via pwsh when present (PS7+), else fall back to Windows PS 5.1
where pwsh >nul 2>nul
if "%ERRORLEVEL%"=="0" (
    pwsh -NoProfile -ExecutionPolicy Bypass -File "%PS1%" %ARGS%
    set "E=%ERRORLEVEL%"
    goto :end
)
powershell -NoProfile -ExecutionPolicy Bypass -File "%PS1%" %ARGS%
set "E=%ERRORLEVEL%"
:end
exit /b %E%

REM  ---- Helper: uppercase a variable that holds the NAME of another variable
:toUPPER
set "s=!%~1!"
for %%i in (A B C D E F G H I J K L M N O P Q R S T U V W X Y Z) do call set "s=%%s:%%i=%%i%%"
set "%~1=%s%"
exit /b 0
