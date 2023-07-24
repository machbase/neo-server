@if not exist .\packages md packages

@SET /p VERSION=<.\tmp\version.txt

@powershell Compress-Archive -Force -DestinationPath ".\packages\machbase-neo-v8.0.0-windows-amd64.zip" -Path ".\tmp\*neo*.exe"