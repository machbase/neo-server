(() => {
    'use strict';

    const process = require('process');
    const fs = require('fs');
    const path = require('path');
    const parseArgs = require('util/parseArgs');
    const rawHttp = require('@jsh/http');
    const zip = require('archive/zip');
    const tar = require('archive/tar');
    const zlib = require('zlib');
    const semver = require('semver');

    const LOCK_FILE_NAME = 'package-lock.json';
    const optionHelp = { type: 'boolean', short: 'h', description: 'Show help', default: false };
    const optionProjectDir = { type: 'string', short: 'C', description: 'Use this project directory instead of the current working directory' };

    const defaultConfig = {
        usage: 'Usage: pkg <command> [options]',
        options: {
            help: optionHelp,
        },
    };

    const initConfig = {
        command: 'init',
        usage: 'pkg init [options] <name>',
        description: 'Create a package.json in the selected project directory',
        options: {
            help: optionHelp,
            dir: optionProjectDir,
        },
        positionals: [
            { name: 'name', description: 'Package name for the current project' },
        ],
    };

    const installConfig = {
        command: 'install',
        usage: 'pkg install [options] [name]',
        description: 'Install dependencies into the selected project directory and maintain package-lock.json',
        options: {
            help: optionHelp,
            dir: optionProjectDir,
        },
        positionals: [
            { name: 'name', description: 'Optional package name to add or update', optional: true },
        ],
    };

    const MACHBASE_DEFAULT_BASE_URL = 'https://github.com/machbase/neo-pkg/raw/refs/heads/main/projects';
    const MACHBASE_DEFAULT_GITHUB_API_URL = 'https://api.github.com';
    const NPM_DEFAULT_REGISTRY_URL = 'https://registry.npmjs.org';

    let parsed;
    try {
        parsed = parseArgs(process.argv.slice(2), defaultConfig, initConfig, installConfig);
    } catch (err) {
        console.println(err.message);
        printHelp();
        process.exit(1);
    }

    if (parsed.values.help) {
        printHelp(parsed.command);
        process.exit(0);
    }

    if (!parsed.command) {
        printHelp();
        process.exit(1);
    }

    if (parsed.command === 'init') {
        doInit(parsed.namedPositionals.name, parsed.values.dir);
        return;
    }

    if (parsed.command === 'install') {
        doInstall(parsed.namedPositionals.name, parsed.values.dir);
        return;
    }

    console.println(`Unknown command: ${parsed.command}`);
    printHelp();
    process.exit(1);

    function printHelp(command) {
        if (command === 'init') {
            console.println(parseArgs.formatHelp(initConfig));
            return;
        }
        if (command === 'install') {
            console.println(parseArgs.formatHelp(installConfig));
            return;
        }
        console.println(parseArgs.formatHelp(defaultConfig, initConfig, installConfig));
    }

    function doInit(name, initDir) {
        validatePackageName(name);

        const cwd = prepareProjectDirectory(process.cwd(), initDir);
        const pkgPath = path.resolve(cwd, 'package.json');
        if (fs.existsSync(pkgPath)) {
            throw new Error(`package.json already exists: ${pkgPath}`);
        }

        const manifest = {
            name: name,
            version: '1.0.0',
            dependencies: {},
        };

        writeJsonFile(pkgPath, manifest);
        console.println(`Created ${pkgPath}`);
    }

    function doInstall(request, installDir) {
        const state = createInstallState(installDir);
        const rootPlan = buildRootInstallPlan(request, state);
        const rootDependencyNames = Object.keys(rootPlan.installDependencies).sort();

        if (rootDependencyNames.length === 0) {
            throw new Error('No dependencies to install. Add dependencies to package.json or provide a package name.');
        }

        fs.mkdirSync(state.installRoot, { recursive: true });
        for (const depName of rootDependencyNames) {
            installPackageRequest({ name: depName, spec: rootPlan.installDependencies[depName] }, state);
        }

        persistProjectState(rootPlan, state);
    }

    function createInstallState(installDir) {
        const invocationCwd = process.cwd();
        const cwd = prepareProjectDirectory(invocationCwd, installDir);
        const manifestPath = path.resolve(cwd, 'package.json');
        const lockPath = path.resolve(cwd, LOCK_FILE_NAME);
        return {
            cwd,
            invocationCwd,
            installRoot: path.resolve(cwd, 'node_modules'),
            manifestPath,
            lockPath,
            projectManifest: readOptionalJsonFile(manifestPath),
            lockFile: readOptionalJsonFile(lockPath),
            registryUrl: normalizeBaseUrl(process.env.get('PKG_NPM_REGISTRY_URL') || NPM_DEFAULT_REGISTRY_URL),
            machbaseBaseUrl: normalizeBaseUrl(process.env.get('PKG_MACHBASE_BASE_URL') || MACHBASE_DEFAULT_BASE_URL),
            machbaseGithubApiUrl: normalizeBaseUrl(process.env.get('PKG_MACHBASE_GITHUB_API_URL') || MACHBASE_DEFAULT_GITHUB_API_URL),
            requested: new Set(),
            resolvedPackages: {},
        };
    }

    function buildRootInstallPlan(request, state) {
        const installDependencies = cloneDependencies(getRootDependencies(state));
        let requestedTask = null;
        if (request) {
            requestedTask = parseRequestedPackage(request);
            validatePackageName(requestedTask.name);
            installDependencies[requestedTask.name] = requestedSpecifier(requestedTask);
        }
        return { installDependencies, requestedTask };
    }

    function getRootDependencies(state) {
        if (state.projectManifest && isRecord(state.projectManifest.dependencies)) {
            return state.projectManifest.dependencies;
        }
        const rootPackage = getLockRootPackage(state.lockFile);
        if (rootPackage && isRecord(rootPackage.dependencies)) {
            return rootPackage.dependencies;
        }
        return {};
    }

    function installPackageRequest(task, state) {
        const requestKey = `${task.name}@${task.spec || ''}`;
        if (state.requested.has(requestKey)) {
            return;
        }
        state.requested.add(requestKey);

        validatePackageName(task.name);

        const locked = findLockedDependency(state.lockFile, task.name, task.spec);
        let staged = null;
        try {
            staged = isMachbaseScoped(task.name)
                ? stageMachbasePackage(task, state, locked)
                : stageNpmPackage(task, state, locked);

            const installResult = installStagedPackage(staged, state);
            rememberResolvedPackage(task, installResult.manifest, staged.source, state);

            const dependencies = normalizeDependencies(installResult.manifest.dependencies);
            const dependencyNames = Object.keys(dependencies).sort();
            for (const depName of dependencyNames) {
                installPackageRequest({ name: depName, spec: dependencies[depName] }, state);
            }
        } finally {
            if (staged && staged.tempRoot) {
                cleanupPath(staged.tempRoot);
            }
        }
    }

    function stageMachbasePackage(task, state, locked) {
        const moduleName = task.name.slice('@machbase/'.length);
        const resolvedSource = locked && typeof locked.resolved === 'string' ? locked.resolved : '';
        const artifactUrl = resolvedSource && resolvedSource.endsWith('.zip')
            ? resolvedSource
            : `${state.machbaseBaseUrl}/${moduleName}/${moduleName}.zip`;
        const directoryUrl = machbaseDirectorySourceUrl(moduleName, resolvedSource, state);

        if (!resolvedSource || resolvedSource.endsWith('.zip')) {
            const stagedArchive = tryStageMachbaseArchive(task, state, locked, moduleName, artifactUrl);
            if (stagedArchive) {
                return stagedArchive;
            }
        }

        return stageMachbaseDirectory(task, state, locked, moduleName, directoryUrl);
    }

    function tryStageMachbaseArchive(task, state, locked, moduleName, artifactUrl) {
        const tempRoot = allocateTempRoot(state.cwd, moduleName);
        const zipPath = path.join(tempRoot, `${moduleName}.zip`);
        const stageDir = path.join(tempRoot, 'stage');
        try {
            const archiveBytes = httpGetBytesOrNull(artifactUrl, [404]);
            if (archiveBytes === null) {
                cleanupPath(tempRoot);
                return null;
            }

            writeBinaryFile(zipPath, archiveBytes);

            const archive = new zip.Zip(zipPath);
            const entries = archive.getEntries();
            validateArchiveEntries(entries, artifactUrl);
            archive.extractAllTo(stageDir, true);

            return finalizeMachbaseStage(task, locked, artifactUrl, tempRoot, stageDir);
        } catch (err) {
            cleanupPath(tempRoot);
            throw err;
        }
    }

    function stageMachbaseDirectory(task, state, locked, moduleName, directoryUrl) {
        const source = parseGitHubProjectSource(directoryUrl);
        if (!source) {
            throw new Error(`Machbase package archive missing and directory fallback is unsupported for ${directoryUrl}`);
        }

        const tempRoot = allocateTempRoot(state.cwd, moduleName);
        const stageDir = path.join(tempRoot, 'stage');
        try {
            downloadGitHubDirectory(source, state, stageDir);
            return finalizeMachbaseStage(task, locked, directoryUrl, tempRoot, stageDir);
        } catch (err) {
            cleanupPath(tempRoot);
            throw err;
        }
    }

    function finalizeMachbaseStage(task, locked, sourceUrl, tempRoot, stageDir) {
        const packageRoot = findPackageRoot(stageDir);
        const manifest = readJsonFile(path.join(packageRoot, 'package.json'));

        if (manifest.name !== task.name) {
            throw new Error(`Machbase package name mismatch: expected ${task.name}, got ${manifest.name}`);
        }
        if (locked && locked.version && manifest.version !== locked.version) {
            throw new Error(`Locked Machbase package version mismatch for ${task.name}: expected ${locked.version}, got ${manifest.version}`);
        }
        if (task.spec && !satisfiesVersion(manifest.version, task.spec)) {
            throw new Error(`Requested version ${task.spec} does not match Machbase package version ${manifest.version}`);
        }

        return {
            manifest,
            packageRoot,
            tempRoot,
            source: sourceUrl,
        };
    }

    function stageNpmPackage(task, state, locked) {
        let resolvedVersion = '';
        let tarballUrl = '';

        if (locked && locked.version && locked.resolved) {
            resolvedVersion = locked.version;
            tarballUrl = locked.resolved;
        } else {
            const metadataUrl = `${state.registryUrl}/${encodeURIComponent(task.name)}`;
            const metadata = httpGetJson(metadataUrl);
            resolvedVersion = resolveNpmVersion(metadata, task.spec);
            const versionMeta = metadata.versions && metadata.versions[resolvedVersion];
            if (!versionMeta || !versionMeta.dist || !versionMeta.dist.tarball) {
                throw new Error(`npm metadata missing tarball for ${task.name}@${resolvedVersion}`);
            }
            tarballUrl = versionMeta.dist.tarball;
        }

        const tempRoot = allocateTempRoot(state.cwd, task.name.replace(/[\/]/g, '-'));
        const tgzPath = path.join(tempRoot, 'package.tgz');
        const tarPath = path.join(tempRoot, 'package.tar');
        const stageDir = path.join(tempRoot, 'stage');

        writeBinaryFile(tgzPath, httpGetBytes(tarballUrl));
        const tarBytes = zlib.gunzipSync(fs.readFileSync(tgzPath, 'buffer'));
        writeBinaryFile(tarPath, tarBytes);

        const archive = new tar.Tar(tarPath);
        const entries = archive.getEntries();
        validateArchiveEntries(entries, tarballUrl);
        archive.extractAllTo(stageDir, true);

        const packageRoot = findPackageRoot(stageDir);
        const manifest = readJsonFile(path.join(packageRoot, 'package.json'));

        if (manifest.name !== task.name) {
            throw new Error(`npm package name mismatch: expected ${task.name}, got ${manifest.name}`);
        }
        if (resolvedVersion && manifest.version !== resolvedVersion) {
            throw new Error(`Resolved npm package version mismatch for ${task.name}: expected ${resolvedVersion}, got ${manifest.version}`);
        }
        if (task.spec && !satisfiesVersion(manifest.version, task.spec)) {
            throw new Error(`Resolved npm package version ${manifest.version} does not satisfy ${task.spec}`);
        }

        return {
            manifest,
            packageRoot,
            tempRoot,
            source: tarballUrl,
        };
    }

    function installStagedPackage(staged, state) {
        const targetDir = packageInstallPath(state.installRoot, staged.manifest.name);
        const installedManifest = readInstalledManifest(targetDir);

        if (installedManifest && installedManifest.version === staged.manifest.version) {
            console.println(`Up to date: ${staged.manifest.name}@${staged.manifest.version}`);
            return {
                manifest: installedManifest,
                targetDir,
                changed: false,
            };
        }

        const tempTarget = `${targetDir}.pkg-tmp-${Date.now()}`;
        cleanupPath(tempTarget);
        fs.mkdirSync(path.dirname(targetDir), { recursive: true });
        fs.cpSync(staged.packageRoot, tempTarget, { recursive: true, force: true });
        cleanupPath(targetDir);
        fs.renameSync(tempTarget, targetDir);

        console.println(`Installed ${staged.manifest.name}@${staged.manifest.version}`);

        return {
            manifest: staged.manifest,
            targetDir,
            changed: true,
        };
    }

    function persistProjectState(rootPlan, state) {
        const manifestDependencies = buildManifestDependencies(rootPlan, state);
        const lockRootDependencies = sortRecord(manifestDependencies || rootPlan.installDependencies);
        if (manifestDependencies) {
            const nextManifest = {
                ...ensureProjectManifest(state),
                dependencies: manifestDependencies,
            };
            writeJsonFile(state.manifestPath, nextManifest);
            state.projectManifest = nextManifest;
        }

        const lockFile = buildLockFile(lockRootDependencies, state);
        writeJsonFile(state.lockPath, lockFile);
        state.lockFile = lockFile;
    }

    function buildManifestDependencies(rootPlan, state) {
        if (!state.projectManifest && !rootPlan.requestedTask) {
            return null;
        }

        const dependencies = cloneDependencies(state.projectManifest ? state.projectManifest.dependencies : {});
        if (rootPlan.requestedTask) {
            const resolved = state.resolvedPackages[rootPlan.requestedTask.name];
            if (!resolved) {
                throw new Error(`Missing resolved package metadata for ${rootPlan.requestedTask.name}`);
            }
            dependencies[rootPlan.requestedTask.name] = manifestSpecifier(rootPlan.requestedTask, resolved.version);
        }
        return sortRecord(dependencies);
    }

    function ensureProjectManifest(state) {
        if (state.projectManifest) {
            return state.projectManifest;
        }
        state.projectManifest = {
            name: path.basename(state.cwd),
            version: '1.0.0',
            dependencies: {},
        };
        return state.projectManifest;
    }

    function buildLockFile(rootDependencies, state) {
        const rootName = state.projectManifest && typeof state.projectManifest.name === 'string'
            ? state.projectManifest.name
            : path.basename(state.cwd);
        const rootVersion = state.projectManifest && typeof state.projectManifest.version === 'string'
            ? state.projectManifest.version
            : '0.0.0';

        const packages = {
            '': {
                name: rootName,
                version: rootVersion,
                dependencies: rootDependencies,
            },
        };
        const dependencies = {};

        const names = Object.keys(state.resolvedPackages).sort();
        for (const name of names) {
            const resolved = state.resolvedPackages[name];
            const packageEntry = {
                name,
                version: resolved.version,
                resolved: resolved.resolved,
            };
            if (Object.keys(resolved.requires).length > 0) {
                packageEntry.dependencies = resolved.requires;
            }
            packages[lockPackagePath(name)] = packageEntry;
        }

        for (const name of Object.keys(rootDependencies).sort()) {
            const dependencyEntry = buildDependencyNode(name, state, new Set());
            if (dependencyEntry) {
                dependencies[name] = dependencyEntry;
            }
        }

        return {
            name: rootName,
            version: rootVersion,
            lockfileVersion: 2,
            requires: true,
            packages,
            dependencies,
        };
    }

    function buildDependencyNode(packageName, state, trail) {
        const resolved = state.resolvedPackages[packageName];
        if (!resolved) {
            return null;
        }
        const node = {
            version: resolved.version,
            resolved: resolved.resolved,
        };
        const requires = resolved.requires;
        if (Object.keys(requires).length > 0) {
            node.requires = requires;
        }
        if (trail.has(packageName)) {
            return node;
        }
        const nextTrail = new Set(trail);
        nextTrail.add(packageName);
        const children = {};
        for (const depName of Object.keys(requires).sort()) {
            const childNode = buildDependencyNode(depName, state, nextTrail);
            if (childNode) {
                children[depName] = childNode;
            }
        }
        if (Object.keys(children).length > 0) {
            node.dependencies = children;
        }
        return node;
    }

    function rememberResolvedPackage(task, manifest, resolvedUrl, state) {
        state.resolvedPackages[manifest.name] = {
            version: manifest.version,
            resolved: resolvedUrl,
            specifier: task.spec || '',
            requires: sortRecord(normalizeDependencies(manifest.dependencies)),
        };
    }

    function packageInstallPath(installRoot, packageName) {
        if (packageName.startsWith('@')) {
            const parts = packageName.split('/');
            return path.join(installRoot, parts[0], parts[1]);
        }
        return path.join(installRoot, packageName);
    }

    function lockPackagePath(packageName) {
        if (packageName.startsWith('@')) {
            const parts = packageName.split('/');
            return `node_modules/${parts[0]}/${parts[1]}`;
        }
        return `node_modules/${packageName}`;
    }

    function readInstalledManifest(targetDir) {
        const manifestPath = path.join(targetDir, 'package.json');
        if (!fs.existsSync(manifestPath)) {
            return null;
        }
        try {
            return readJsonFile(manifestPath);
        } catch (err) {
            return null;
        }
    }

    function findLockedDependency(lockFile, packageName, spec) {
        if (!lockFile) {
            return null;
        }
        let entry = null;
        if (isRecord(lockFile.dependencies) && isRecord(lockFile.dependencies[packageName])) {
            entry = lockFile.dependencies[packageName];
        } else if (isRecord(lockFile.packages)) {
            const packageEntry = lockFile.packages[lockPackagePath(packageName)];
            if (isRecord(packageEntry)) {
                entry = packageEntry;
            }
        }
        if (!entry || typeof entry.version !== 'string' || typeof entry.resolved !== 'string') {
            return null;
        }
        if (spec && !satisfiesVersion(entry.version, spec)) {
            return null;
        }
        return {
            version: entry.version,
            resolved: entry.resolved,
            dependencies: normalizeDependencies(entry.dependencies),
        };
    }

    function manifestSpecifier(task, resolvedVersion) {
        if (task.spec) {
            return task.spec;
        }
        if (isMachbaseScoped(task.name)) {
            return resolvedVersion;
        }
        return `^${resolvedVersion}`;
    }

    function getLockRootPackage(lockFile) {
        if (!lockFile || !isRecord(lockFile.packages) || !isRecord(lockFile.packages[''])) {
            return null;
        }
        return lockFile.packages[''];
    }

    function allocateTempRoot(cwd, label) {
        const safeLabel = label.replace(/[^a-zA-Z0-9._-]+/g, '_');
        const tempRoot = path.join(cwd, '.pkg-tmp', `${safeLabel}-${Date.now()}-${Math.floor(Math.random() * 100000)}`);
        fs.mkdirSync(tempRoot, { recursive: true });
        return tempRoot;
    }

    function cleanupPath(targetPath) {
        if (fs.existsSync(targetPath)) {
            fs.rmSync(targetPath, { recursive: true, force: true });
        }
    }

    function readJsonFile(filePath) {
        return JSON.parse(fs.readFileSync(filePath, 'utf8'));
    }

    function readOptionalJsonFile(filePath) {
        if (!fs.existsSync(filePath)) {
            return null;
        }
        return readJsonFile(filePath);
    }

    function writeJsonFile(filePath, value) {
        fs.writeFileSync(filePath, `${JSON.stringify(value, null, 2)}\n`, 'utf8');
    }

    function validatePackageName(name) {
        if (typeof name !== 'string' || name.length === 0) {
            throw new Error('Package name is required');
        }
        if (!/^(?:@[A-Za-z0-9._-]+\/)?[A-Za-z0-9._-]+$/.test(name)) {
            throw new Error(`Invalid package name: ${name}`);
        }
    }

    function parseRequestedPackage(value) {
        if (typeof value !== 'string' || value.length === 0) {
            throw new Error('Package name is required');
        }
        if (value.startsWith('@')) {
            const slashIndex = value.indexOf('/');
            if (slashIndex === -1) {
                throw new Error(`Invalid scoped package name: ${value}`);
            }
            const versionMarker = value.lastIndexOf('@');
            if (versionMarker > slashIndex) {
                return {
                    name: value.slice(0, versionMarker),
                    spec: value.slice(versionMarker + 1),
                };
            }
            return { name: value, spec: '' };
        }

        const versionMarker = value.lastIndexOf('@');
        if (versionMarker > 0) {
            return {
                name: value.slice(0, versionMarker),
                spec: value.slice(versionMarker + 1),
            };
        }
        return { name: value, spec: '' };
    }

    function requestedSpecifier(task) {
        return task.spec || 'latest';
    }

    function isMachbaseScoped(name) {
        return /^@machbase\/[A-Za-z0-9._-]+$/.test(name);
    }

    function machbaseDirectorySourceUrl(moduleName, resolvedSource, state) {
        if (resolvedSource) {
            const archiveSuffix = `/${moduleName}.zip`;
            if (resolvedSource.endsWith(archiveSuffix)) {
                return normalizeBaseUrl(resolvedSource.slice(0, -archiveSuffix.length));
            }
            return normalizeBaseUrl(resolvedSource);
        }
        return `${state.machbaseBaseUrl}/${moduleName}`;
    }

    function resolveInstallDirectory(invocationCwd, installDir) {
        if (typeof installDir !== 'string' || installDir.trim().length === 0) {
            return invocationCwd;
        }
        const resolved = path.resolve(invocationCwd, installDir.trim());
        if (fs.existsSync(resolved) && !fs.statSync(resolved).isDirectory()) {
            throw new Error(`Install target is not a directory: ${resolved}`);
        }
        return resolved;
    }

    function prepareProjectDirectory(invocationCwd, installDir) {
        const cwd = resolveInstallDirectory(invocationCwd, installDir);
        fs.mkdirSync(cwd, { recursive: true });
        return cwd;
    }

    function normalizeBaseUrl(url) {
        return String(url).replace(/\/+$/, '');
    }

    function httpGetJson(url, headers) {
        const response = httpRequest('GET', url, { headers });
        try {
            return response.json();
        } finally {
            response.close();
        }
    }

    function httpGetBytes(url) {
        const response = httpRequest('GET', url);
        try {
            return response.readAll();
        } finally {
            response.close();
        }
    }

    function httpGetBytesOrNull(url, allowedStatusCodes) {
        const response = httpRequest('GET', url, { allowedStatusCodes });
        try {
            if (!response.ok) {
                return null;
            }
            return response.readAll();
        } finally {
            response.close();
        }
    }

    function httpRequest(method, url, options) {
        const opts = options || {};
        const headers = opts.headers || {};
        const allowedStatusCodes = new Set(opts.allowedStatusCodes || []);
        const client = rawHttp.NewClient();
        const request = rawHttp.NewRequest(method, url);
        request.header.set('Accept', headers.Accept || '*/*');
        for (const name of Object.keys(headers)) {
            if (name !== 'Accept') {
                request.header.set(name, String(headers[name]));
            }
        }
        const response = client.do(request);
        if (!response.ok && !allowedStatusCodes.has(response.statusCode)) {
            let body = '';
            try {
                body = response.string();
            } catch (err) {
                body = '';
            } finally {
                response.close();
            }
            const error = new Error(`HTTP ${response.statusCode} ${response.statusMessage} for ${url}${body ? `: ${body}` : ''}`);
            error.statusCode = response.statusCode;
            error.statusMessage = response.statusMessage;
            throw error;
        }
        return response;
    }

    function writeBinaryFile(filePath, data) {
        const bytes = toByteArray(data);
        fs.writeFileSync(filePath, bytes, 'buffer');
    }

    function toByteArray(data) {
        if (data instanceof Uint8Array) {
            return Array.from(data);
        }
        if (data instanceof ArrayBuffer) {
            return Array.from(new Uint8Array(data));
        }
        if (Array.isArray(data)) {
            return data;
        }
        throw new Error('Unsupported binary payload type');
    }

    function findPackageRoot(baseDir) {
        const relativePaths = fs.readdirSync(baseDir, { recursive: true });
        const packageJsons = relativePaths
            .filter((entry) => entry === 'package.json' || entry.endsWith('/package.json'))
            .filter((entry) => !entry.startsWith('node_modules/'));

        if (packageJsons.length === 0) {
            throw new Error(`No package.json found under ${baseDir}`);
        }

        const uniqueRoots = [];
        const seen = new Set();
        for (const entry of packageJsons) {
            const root = entry === 'package.json' ? '.' : path.dirname(entry);
            if (!seen.has(root)) {
                seen.add(root);
                uniqueRoots.push(root);
            }
        }

        if (seen.has('package')) {
            return path.join(baseDir, 'package');
        }
        if (seen.has('.')) {
            return baseDir;
        }
        if (uniqueRoots.length === 1) {
            return path.join(baseDir, uniqueRoots[0]);
        }

        uniqueRoots.sort((left, right) => left.length - right.length);
        return path.join(baseDir, uniqueRoots[0]);
    }

    function downloadGitHubDirectory(source, state, targetDir) {
        fs.mkdirSync(targetDir, { recursive: true });
        const entries = listGitHubDirectoryEntries(source, state, source.projectPath);
        for (const entry of entries) {
            downloadGitHubDirectoryEntry(entry, source, state, targetDir);
        }
    }

    function listGitHubDirectoryEntries(source, state, repoPath) {
        const apiUrl = `${state.machbaseGithubApiUrl}/repos/${encodeURIComponent(source.owner)}/${encodeURIComponent(source.repo)}/contents/${encodePathSegments(repoPath)}?ref=${encodeURIComponent(source.ref)}`;
        const payload = httpGetJson(apiUrl, {
            Accept: 'application/vnd.github+json',
            'User-Agent': 'neo-pkg',
        });
        if (!Array.isArray(payload)) {
            throw new Error(`Invalid GitHub directory listing for ${repoPath}`);
        }
        return payload;
    }

    function downloadGitHubDirectoryEntry(entry, source, state, targetDir) {
        if (!isRecord(entry) || typeof entry.type !== 'string' || typeof entry.path !== 'string') {
            throw new Error('Invalid GitHub directory entry');
        }

        const relativePath = relativeProjectPath(entry.path, source.projectPath);
        if (entry.type === 'dir') {
            const childDir = resolveSafeTargetPath(targetDir, relativePath, entry.path);
            fs.mkdirSync(childDir, { recursive: true });
            const childEntries = listGitHubDirectoryEntries(source, state, entry.path);
            for (const childEntry of childEntries) {
                downloadGitHubDirectoryEntry(childEntry, source, state, targetDir);
            }
            return;
        }

        if (entry.type !== 'file') {
            return;
        }
        if (typeof entry.download_url !== 'string' || entry.download_url.length === 0) {
            throw new Error(`GitHub file entry is missing download URL: ${entry.path}`);
        }

        const filePath = resolveSafeTargetPath(targetDir, relativePath, entry.path);
        fs.mkdirSync(path.dirname(filePath), { recursive: true });
        writeBinaryFile(filePath, httpGetBytes(entry.download_url));
    }

    function parseGitHubProjectSource(sourceUrl) {
        let parsedUrl;
        try {
            parsedUrl = new URL(sourceUrl);
        } catch (err) {
            return null;
        }

        const segments = parsedUrl.pathname.split('/').filter((segment) => segment.length > 0);
        if (segments.length < 6) {
            return null;
        }

        let refStartIndex = -1;
        if (segments[2] === 'raw' && segments[3] === 'refs' && segments[4] === 'heads') {
            refStartIndex = 5;
        } else if (segments[2] === 'refs' && segments[3] === 'heads') {
            refStartIndex = 4;
        } else {
            return null;
        }

        const projectsIndex = segments.lastIndexOf('projects');
        if (projectsIndex === -1 || projectsIndex <= refStartIndex || projectsIndex >= segments.length - 1) {
            return null;
        }

        const owner = segments[0];
        const repo = segments[1];
        const ref = segments.slice(refStartIndex, projectsIndex).join('/');
        const projectPath = segments.slice(projectsIndex).join('/');
        if (!owner || !repo || !ref) {
            return null;
        }

        return {
            owner,
            repo,
            ref,
            projectPath,
        };
    }

    function relativeProjectPath(entryPath, projectPath) {
        if (entryPath === projectPath) {
            return '';
        }
        if (!entryPath.startsWith(`${projectPath}/`)) {
            throw new Error(`GitHub entry path is outside the project directory: ${entryPath}`);
        }
        return entryPath.slice(projectPath.length + 1);
    }

    function resolveSafeTargetPath(baseDir, relativePath, sourceLabel) {
        const normalized = String(relativePath || '').replace(/\\/g, '/');
        if (!normalized) {
            return baseDir;
        }
        if (!isSafeRelativePath(normalized)) {
            throw new Error(`Unsafe GitHub path in ${sourceLabel}: ${relativePath}`);
        }
        return path.join(baseDir, normalized);
    }

    function encodePathSegments(value) {
        return String(value)
            .split('/')
            .filter((segment) => segment.length > 0)
            .map((segment) => encodeURIComponent(segment))
            .join('/');
    }

    function validateArchiveEntries(entries, sourceLabel) {
        for (const entry of entries) {
            const name = entry.name || '';
            const normalized = String(name).replace(/\\/g, '/');
            if (!normalized || normalized === '.' || normalized.includes('\0')) {
                throw new Error(`Unsafe archive entry in ${sourceLabel}: ${name}`);
            }
            if (!isSafeRelativePath(normalized)) {
                throw new Error(`Unsafe archive entry in ${sourceLabel}: ${name}`);
            }
        }
    }

    function isSafeRelativePath(value) {
        return !value.startsWith('/')
            && value !== '..'
            && !value.startsWith('../')
            && !value.includes('/../')
            && !/^[A-Za-z]:/.test(value)
            && !value.includes('\0');
    }

    function resolveNpmVersion(metadata, spec) {
        const versions = Object.keys(metadata.versions || {});
        if (versions.length === 0) {
            throw new Error(`npm metadata does not contain any versions for ${metadata.name || 'package'}`);
        }

        const rawSpec = String(spec || '').trim();
        if (!rawSpec || rawSpec === '*' || rawSpec === 'latest') {
            const latest = metadata['dist-tags'] && metadata['dist-tags'].latest;
            if (latest && metadata.versions[latest]) {
                return latest;
            }
            const fallback = semver.maxSatisfying(versions, '*');
            if (fallback) {
                return fallback;
            }
            throw new Error(`Unable to resolve latest version for ${metadata.name || 'package'}`);
        }

        if (metadata['dist-tags'] && metadata['dist-tags'][rawSpec]) {
            return metadata['dist-tags'][rawSpec];
        }

        if (metadata.versions[rawSpec]) {
            return rawSpec;
        }

        const matched = semver.maxSatisfying(versions, rawSpec);
        if (!matched) {
            throw new Error(`No npm version found for ${metadata.name || 'package'} matching ${rawSpec}`);
        }
        return matched;
    }

    function satisfiesVersion(version, spec) {
        const rawSpec = String(spec || '').trim();
        if (!rawSpec || rawSpec === '*' || rawSpec === 'latest') {
            return true;
        }
        return semver.satisfies(version, rawSpec);
    }

    function normalizeDependencies(value) {
        if (!isRecord(value)) {
            return {};
        }
        const normalized = {};
        for (const name of Object.keys(value)) {
            if (typeof value[name] === 'string' && value[name].length > 0) {
                normalized[name] = value[name];
            }
        }
        return normalized;
    }

    function cloneDependencies(value) {
        return { ...normalizeDependencies(value) };
    }

    function sortRecord(value) {
        const sorted = {};
        for (const key of Object.keys(value).sort()) {
            sorted[key] = value[key];
        }
        return sorted;
    }

    function isRecord(value) {
        return value && typeof value === 'object' && !Array.isArray(value);
    }
})();