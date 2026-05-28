@echo off
setlocal enabledelayedexpansion
cd /d "%~dp0\.."

set BIN=%CD%\bin
set PID_DIR=%BIN%\.pids
if not exist "%BIN%" mkdir "%BIN%"
if not exist "%PID_DIR%" mkdir "%PID_DIR%"

set CMD=%1
set SVC=%2
set CONF=%3
set ID=%4
if "%CMD%"=="" goto :usage
if "%SVC%"=="" set SVC=all
if "%CONF%"=="" set CONF=config.yml

if "%CMD%"=="build"    goto :build
if "%CMD%"=="start"    goto :start
if "%CMD%"=="stop"     goto :stop
if "%CMD%"=="restart"  goto :restart
if "%CMD%"=="reboot"   goto :reboot
goto :usage

:build
  if "%SVC%"=="chatsvr" (call :build_one chatsvr & goto :eof)
  if "%SVC%"=="gateway" (call :build_one gateway & goto :eof)
  call :build_one chatsvr
  call :build_one gateway
  echo Done.
  goto :eof

:start
  if "%SVC%"=="chatsvr" (call :start_one chatsvr & goto :eof)
  if "%SVC%"=="gateway" (call :start_one gateway & goto :eof)
  call :start_one chatsvr
  timeout /t 1 /nobreak >nul
  call :start_one gateway
  goto :eof

:stop
  if "%SVC%"=="chatsvr" (call :stop_one chatsvr & goto :eof)
  if "%SVC%"=="gateway" (call :stop_one gateway & goto :eof)
  call :stop_one chatsvr
  call :stop_one gateway
  goto :eof

:restart
  call :stop %SVC%
  timeout /t 1 /nobreak >nul
  call :start %SVC%
  goto :eof

:reboot
  call :stop %SVC%
  call :build %SVC%
  call :start %SVC%
  goto :eof

:usage
  echo Usage: svc.bat ^<cmd^> ^<target^> [config] [id]
  echo.
  echo Commands:
  echo   build xxx    - compile binary
  echo   start xxx    - run binary (auto-builds if missing)
  echo   stop xxx     - kill process
  echo   restart xxx  - stop + start
  echo   reboot xxx   - stop + build + start
  echo.
  echo Targets: chatsvr ^| gateway ^| all
  echo Config:  path to config yml (default: config.yml)
  echo ID:      instance ID in config services array (default: first entry)
  echo.
  echo Examples:
  echo   svc.bat build all
  echo   svc.bat start chatsvr
  echo   svc.bat start gateway prod.yml
  echo   svc.bat start gateway prod.yml gw-1
  echo   svc.bat restart chatsvr prod.yml cs-2
  echo   svc.bat reboot all prod.yml
  goto :eof

:: ---- helpers ----

:build_one
  echo ^>^>^> Building %~1...
  go build -o "%BIN%\%~1.exe" ./apps/%~1/cmd
  if %errorlevel% neq 0 exit /b %errorlevel%
  echo   -^> %BIN%\%~1.exe
  goto :eof

:start_one
  if exist "%PID_DIR%\%~1.pid" (
    call :is_running %~1 && (echo %~1 is already running & goto :eof)
  )
  if not exist "%BIN%\%~1.exe" call :build_one %~1
  if not exist "%CD%\log" mkdir "%CD%\log"
  echo ^>^>^> Starting %~1 (conf=%CONF%, id=%ID%)...
  :: Temp launcher batch: clean redirection without escaping
  set LAUNCH=%TEMP%\cardwar_%~1.bat
  if "%ID%"=="" (
    echo @"%BIN%\%~1.exe" -conf %CONF% ^> "%CD%\log\%~1.log" 2^>^&1 > "%LAUNCH%"
  ) else (
    echo @"%BIN%\%~1.exe" -conf %CONF% -id %ID% ^> "%CD%\log\%~1.log" 2^>^&1 > "%LAUNCH%"
  )
  :: PowerShell: hidden, detached, survives terminal close
  powershell -Command "Start-Process -WindowStyle Hidden cmd -ArgumentList '/c ""%LAUNCH%""'"
  echo   Started (log: log\%~1.log)
  goto :eof

:stop_one
  echo ^>^>^> Stopping %~1...
  taskkill /fi "IMAGENAME eq %~1.exe" /f 2>nul
  del "%PID_DIR%\%~1.pid" 2>nul
  echo   Stopped
  goto :eof

:is_running
  tasklist /fi "IMAGENAME eq %~1.exe" 2>nul | findstr "%~1.exe" >nul
  exit /b %errorlevel%
