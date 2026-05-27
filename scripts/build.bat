@echo off
pushd "%~dp0\.."
set ROOT=%CD%

if "%1"=="" goto :all
if "%1"=="all" goto :all
if "%1"=="chatsvr" goto :chatsvr
if "%1"=="gateway" goto :gateway
echo Usage: build.bat [all^|chatsvr^|gateway]
popd & exit /b 1

:all
  call :chatsvr
  call :gateway
  goto :done

:chatsvr
  echo Building chatsvr...
  if not exist "%ROOT%\bin" mkdir "%ROOT%\bin"
  go build -o "%ROOT%\bin\chatsvr.exe" .\apps\chatsvr\cmd\
  if %errorlevel% neq 0 (popd & exit /b %errorlevel%)
  echo   ^> %ROOT%\bin\chatsvr.exe
  goto :eof

:gateway
  echo Building gateway...
  if not exist "%ROOT%\bin" mkdir "%ROOT%\bin"
  go build -o "%ROOT%\bin\gateway.exe" .\apps\gateway\cmd\
  if %errorlevel% neq 0 (popd & exit /b %errorlevel%)
  echo   ^> %ROOT%\bin\gateway.exe
  goto :eof

:done
  echo Done. Binaries at %ROOT%\bin\
popd
