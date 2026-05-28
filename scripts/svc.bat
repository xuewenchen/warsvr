@echo off
cd /d "%~dp0\.."
if not exist bin\svchelper.exe (
  echo Building svchelper...
  go build -o bin\svchelper.exe .\tools\svchelper\ >nul 2>&1
)
bin\svchelper.exe %*
