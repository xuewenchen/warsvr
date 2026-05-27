@echo off
cd /d "%~dp0\.."

set CONF=%1
if "%CONF%"=="" set CONF=config.yml

if not exist "%CONF%" (
  echo ERROR: config file not found: %CONF%
  exit /b 1
)

echo === Starting all services ===
echo Config: %CONF%

echo.
echo ^>^>^> Starting ChatSvr...
start "ChatSvr" cmd /c "go run .\apps\chatsvr\cmd\ -conf %CONF% & pause"

timeout /t 2 /nobreak >nul

echo ^>^>^> Starting Gateway...
start "Gateway" cmd /c "go run .\apps\gateway\cmd\ -conf %CONF% & pause"

echo.
echo === Both services started in separate windows ===
echo Close each window or press Ctrl+C in them to stop.
