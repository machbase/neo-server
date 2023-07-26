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

@git rev-parse --short work/gsc > .\tmp\gitsha.txt
@date /T > .\tmp\buildtime.txt
@go version > .\tmp\goverstr.txt
@cd > .\tmp\cwd.txt

@SET VERSION=v8.0.0
@SET /p GITSHA=<.\tmp\gitsha.txt
@SET /p BUILDTIME=<.\tmp\buildtime.txt
@SET /p GOVERSTR=<.\tmp\goverstr.txt
@SET /p PRJABSPATH=<.\tmp\cwd.txt

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
@SET LDFLAGS=%LDFLAGS% -X %MODNAME%/mods.editionString="standard"

@REM -tags=timetzdata is required for windows users
go build -ldflags "%LDFLAGS%" -tags=timetzdata -o .\tmp\machbase-neo.exe .\main\machbase-neo

@SET CGO_ENABLED=0 
@REM go build -ldflags "-H=windowsgui" -o .\tmp\neow.exe .\main\neow
fyne package --os windows --src main\neow --icon %PRJABSPATH%\main\neow\res\appicon.png --id com.machbase.neow
move /Y %PRJABSPATH%\main\neow\neow.exe %PRJABSPATH%\tmp\neow.exe