# Package Overview

## Package Type

MACHBASE provides manual installation and package installation files.

|Installation type|Description|Note|
|--|--|--|
|manual installation|Has a compressed file format and the extension tgz for Unix.<br>The user decompresses using tar and GNU gzip to proceed with the installation.|Can be installed only in console environment|
|package installation|Provides an installation package for each operating system environment.<br> - Windows: msi <br> -  Linux: tgz|Can be installed only in console environment|

## Package File Name Structure

The package file name is configured as follows.

```
machbase-EDITION_VERSION-OS-CPU-BIT-MODE-OPTIONAL.EXT
```

|item|Description|
|--|--|
|EDITION|Indicates the edition of the package.<br> - standard: Standard Edition<br> - cluster: Cluster Edition|
|VERSION|Indicates the version of the package.<br>In detail, it is classified as _MajorVersion.MinorVersion.FixVersion.AUX_ by numbers and characters.<br>- Major Version: Product main version - number<br>- Minior Version: A version with relatively large features added in the same main version. DB file / protocol compatibility is not guaranteed. -number<br>- Fix Version: A bug / minor feature added in the same main version. DB file / protocol compatibility  is guaranteed. - number<br>- AUX: Indicates the package classification -number<br> -- official: general package<br> -- community: community edition package|
|OS|Indicates the operating system name. (Example) LINUX, WINDOWS|
|CPU|Indicates the type of CPU installed in the operating system. (Example) X86, IA64|
|BIT|Indicates whether  the compiled binary  is 32-bit or 64-bit. (Example) 32, 64|
|MODE|Indicates the release mode of the binary once compiled. (Example) release, debug, prerelease|
|OPTIONAL|Only displayed in Enterprise Edition.<br>lightweight: Indicates a lightweight package to be added to the Coordinator.|
|EXT|The package file extension. Depending on the package, it is available as tgz, rpm, deb, and msi.|
