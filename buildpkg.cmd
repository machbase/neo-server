@if not exist .\packages md packages

@SET /p VERSION=<.\tmp\version.txt

@powershell Compress-Archive -Force -DestinationPath ".\packages\machbase-neo-%VERSION%-windows-amd64.zip" -Path ".\tmp\*neo*.exe"