@echo off
cd /d "%~dp0\.."

set CONF=config.yml
set ID=

if "%~1"=="" goto :run
if "%~x1"==".yml" (set CONF=%~1) & (set ID=%~2) & goto :run
if "%~x1"==".yaml" (set CONF=%~1) & (set ID=%~2) & goto :run
set ID=%~1

:run
if not exist "%CONF%" (
  echo ERROR: config file not found: %CONF%
  exit /b 1
)
echo ^>^>^> Starting ChatSvr (conf=%CONF%, id=%ID%)
if "%ID%"=="" (
  go run .\apps\chatsvr\cmd\ -conf %CONF%
) else (
  go run .\apps\chatsvr\cmd\ -conf %CONF% -id %ID%
)
