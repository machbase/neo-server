@build.cmd

@if not exist .\packages md packages

@powershell Compress-Archive -Force -DestinationPath ".\packages\machbase-neo-fog-%VERSION%-windows-amd64.zip" -Path ".\tmp\*neo*.exe"