@REM Install cygwin with gcc toolchain
@REM      
@REM    - Prefer using TDM-GCC-64
@REM

@SET GOOS=windows
@SET GOARCH=amd64
@SET CGO_ENABLED=1
@REM SET CC=C:\TDM-GCC-64\bin\gcc.exe
@REM SET CXX=C:\TDM-GCC-64\bin\g++.exe
@SET CC=gcc.exe
@SET CXX=g++.exe
@SET CGO_LDFLAGS=
@SET CGO_CFLAGS=
@SET GO11MODULE=on

@if not exist .\tmp md tmp

@git describe --tags --abbrev=0 > .\tmp\version.txt
@git rev-parse --short main > .\tmp\gitsha.txt
@date /T > .\tmp\buildtime.txt
@go version > .\tmp\goverstr.txt

@SET /p VERSION=<.\tmp\version.txt
@SET /p GITSHA=<.\tmp\gitsha.txt
@SET /p BUILDTIME=<.\tmp\buildtime.txt
@SET /p GOVERSTR=<.\tmp\goverstr.txt

@for /f "tokens=3*" %%a in ("%GOVERSTR%") do (
    @SET GOVERSTR=%%a
)

@SET GOVERSTR=%GOVERSTR:~2%
@SET BUILDTIME=%BUILDTIME:/=-%
@SET BUILDTIME=%BUILDTIME: =%

@SET MODNAME=github.com/machbase/neo-server
@SET LDFLAGS=-X %MODNAME%/mods.versionString="%VERSION%"
@SET LDFLAGS=%LDFLAGS% -X %MODNAME%/mods.versionGitSHA="%GITSHA%"
@SET LDFLAGS=%LDFLAGS% -X %MODNAME%/mods.buildTimestamp="%BUILDTIME%"
@SET LDFLAGS=%LDFLAGS% -X %MODNAME%/mods.goVersionString="%GOVERSTR%"
@SET LDFLAGS=%LDFLAGS% -X %MODNAME%/mods.editionString="fog"

@REM -tags=timetzdata is required for windows users
go build -ldflags "%LDFLAGS%" -tags=fog_edition,timetzdata -o .\tmp\machbase-neo.exe .\main\machbase-neo

@SET CGO_ENABLED=0 
go build -ldflags "-H=windowsgui" -o .\tmp\neowin.exe .\main\neowin

@if not exist .\packages md packages

@powershell Compress-Archive -Force -DestinationPath ".\packages\machbase-neo-fog-%VERSION%-windows-amd64.zip" -Path ".\tmp\*neo*.exe"