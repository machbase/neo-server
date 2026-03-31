(() => {
    'use strict';

    const process = require('process');
    const fs = require('fs');
    const path = require('path');
    const parseArgs = require('util/parseArgs');
    const splitFields = require('util/splitFields');
    const rawHttp = require('@jsh/http');
    const zip = require('archive/zip');
    const tar = require('archive/tar');
    const zlib = require('zlib');
    const semver = require('semver');

    const LOCK_FILE_NAME = 'package-lock.json';
    const GLOBAL_PROJECT_DIR = '/work';
    const optionHelp = { type: 'boolean', short: 'h', description: 'Show help', default: false };
    const optionProjectDir = { type: 'string', short: 'C', description: 'Use this project directory instead of the current working directory' };
    const optionGlobal = { type: 'boolean', short: 'g', description: 'Install into the global package directory /work and ignore --dir', default: false };

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
            global: optionGlobal,
        },
        positionals: [
            { name: 'name', description: 'Optional package name to add or update', optional: true },
        ],
    };

    const uninstallConfig = {
        command: 'uninstall',
        usage: 'pkg uninstall [options] <name>',
        description: 'Remove a dependency and its generated package command wrapper from the selected project directory',
        options: {
            help: optionHelp,
            dir: optionProjectDir,
            global: optionGlobal,
        },
        positionals: [
            { name: 'name', description: 'Package name to remove' },
        ],
    };

    const runConfig = {
        command: 'run',
        usage: 'pkg run [options] <key> [...args]',
        description: 'Run a package.json script from the selected project directory',
        options: {
            help: optionHelp,
            dir: optionProjectDir,
        },
        positionals: [
            { name: 'key', description: 'Script name in package.json' },
            { name: 'args', description: 'Additional arguments to append to the script', optional: true, variadic: true },
        ],
    };

    const GITHUB_DEFAULT_API_URL = 'https://api.github.com';
    const NPM_DEFAULT_REGISTRY_URL = 'https://registry.npmjs.org';

    let parsed;
    try {
        parsed = parseArgs(process.argv.slice(2), defaultConfig, initConfig, installConfig, uninstallConfig, runConfig);
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
        doInstall(parsed.namedPositionals.name, parsed.values.dir, parsed.values.global);
        return;
    }

    if (parsed.command === 'run') {
        doRun(parsed.namedPositionals.key, parsed.namedPositionals.args || [], parsed.values.dir);
        return;
    }

    if (parsed.command === 'uninstall') {
        doUninstall(parsed.namedPositionals.name, parsed.values.dir, parsed.values.global);
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
        if (command === 'uninstall') {
            console.println(parseArgs.formatHelp(uninstallConfig));
            return;
        }
        if (command === 'run') {
            console.println(parseArgs.formatHelp(runConfig));
            return;
        }
        console.println(parseArgs.formatHelp(defaultConfig, initConfig, installConfig, uninstallConfig, runConfig));
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
            scripts: {},
            dependencies: {},
        };

        writeJsonFile(pkgPath, manifest);
        console.println(`Created ${pkgPath}`);
    }

    function doRun(key, scriptArgs, runDir) {
        const cwd = prepareProjectDirectory(process.cwd(), runDir);
        const manifestPath = path.resolve(cwd, 'package.json');
        const manifest = readOptionalJsonFile(manifestPath);

        if (!manifest) {
            throw new Error(`package.json not found: ${manifestPath}`);
        }
        if (!isRecord(manifest.scripts)) {
            throw new Error(`package.json does not contain a valid scripts object: ${manifestPath}`);
        }

        process.chdir(cwd);

        const scriptLine = normalizeScripts(manifest.scripts)[key];
        if (typeof scriptLine !== 'string' || scriptLine.trim().length === 0) {
            throw new Error(`Script not found or empty: ${key}`);
        }

        const fields = splitFields(scriptLine);
        if (fields.length === 0) {
            throw new Error(`Script produced no executable command: ${key}`);
        }

        const exitCode = process.exec(resolveRunCommand(fields[0], cwd), ...fields.slice(1), ...scriptArgs);
        if (exitCode instanceof Error) {
            throw exitCode;
        }
        if (exitCode !== 0) {
            process.exit(exitCode);
        }
    }

    function resolveRunCommand(command, projectDir) {
        if (typeof command !== 'string' || command.length === 0) {
            return command;
        }
        if (!command.endsWith('.js') || path.isAbsolute(command)) {
            return command;
        }
        const resolved = path.resolve(projectDir, command);
        if (!fs.existsSync(resolved)) {
            return command;
        }
        const stats = fs.statSync(resolved);
        if (!stats.isFile()) {
            return command;
        }
        return resolved;
    }

    function doInstall(request, installDir, globalInstall) {
        const state = createInstallState(installDir, globalInstall);
        const rootPlan = buildRootInstallPlan(request, state);
        const rootDependencyNames = Object.keys(rootPlan.installDependencies).sort();

        if (rootDependencyNames.length === 0) {
            throw new Error('No dependencies to install. Add dependencies to package.json or provide a package name.');
        }

        fs.mkdirSync(state.installRoot, { recursive: true });
        if (rootPlan.requestedTask) {
            installPackageRequest(rootPlan.requestedTask, state);
        }
        for (const depName of rootDependencyNames) {
            if (rootPlan.requestedTask && depName === rootPlan.requestedTask.name) {
                continue;
            }
            installPackageRequest({ name: depName, spec: rootPlan.installDependencies[depName] }, state);
        }

        persistProjectState(rootPlan, state);
    }

    function doUninstall(request, uninstallDir, globalInstall) {
        const state = createInstallState(uninstallDir, globalInstall);
        const task = createUninstallTask(request);
        const dependencies = cloneDependencies(getRootDependencies(state));
        if (!Object.prototype.hasOwnProperty.call(dependencies, task.name)) {
            throw new Error(`Dependency not found: ${task.name}`);
        }

        delete dependencies[task.name];

        const nextManifest = stripLegacyPackageCommandMetadata({
            ...ensureProjectManifest(state),
            scripts: stripManagedPackageCommandScripts(ensureProjectManifest(state).scripts, state.installRoot),
            dependencies: sortRecord(dependencies),
        });

        cleanupPath(packageInstallPath(state.installRoot, task.name));
        removePackageCommandWrappersByOwner(task.name, state);
        cleanupPath(state.lockPath);

        writeJsonFile(state.manifestPath, nextManifest);
        state.projectManifest = nextManifest;
        state.lockFile = null;

        console.println(`Removed ${task.name}`);

        if (Object.keys(dependencies).length > 0) {
            doInstall('', uninstallDir, globalInstall);
        }
    }

    function createInstallState(installDir, globalInstall) {
        const invocationCwd = process.cwd();
        const cwd = prepareProjectDirectory(invocationCwd, installDir, globalInstall);
        const manifestPath = path.resolve(cwd, 'package.json');
        const lockPath = path.resolve(cwd, LOCK_FILE_NAME);
        return {
            cwd,
            invocationCwd,
            globalInstall: !!globalInstall,
            installRoot: path.resolve(cwd, 'node_modules'),
            tempRootBase: resolveTempRootBase(invocationCwd),
            manifestPath,
            lockPath,
            projectManifest: readOptionalJsonFile(manifestPath),
            lockFile: readOptionalJsonFile(lockPath),
            registryUrl: normalizeBaseUrl(process.env.get('PKG_NPM_REGISTRY_URL') || NPM_DEFAULT_REGISTRY_URL),
            githubApiUrl: normalizeBaseUrl(process.env.get('PKG_GITHUB_API_URL') || process.env.get('PKG_MACHBASE_GITHUB_API_URL') || GITHUB_DEFAULT_API_URL),
            requested: new Set(),
            resolvedPackages: {},
            installedPackages: {},
        };
    }

    function buildRootInstallPlan(request, state) {
        const installDependencies = cloneDependencies(getRootDependencies(state));
        let requestedTask = null;
        if (request) {
            requestedTask = createRequestedTask(request);
            installDependencies[requestedTask.name] = requestedSpecifier(requestedTask);
        }
        return { installDependencies, requestedTask };
    }

    function createRequestedTask(request) {
        const requestedTask = parseRequestedPackage(request);
        validatePackageName(requestedTask.name);
        requestedTask.refreshLatest = isGitHubRepositoryPackage(requestedTask.name) && !requestedTask.spec;
        return requestedTask;
    }

    function createUninstallTask(request) {
        const task = parseRequestedPackage(request);
        validatePackageName(task.name);
        if (task.spec) {
            throw new Error(`pkg uninstall does not accept a version specifier: ${request}`);
        }
        return task;
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
        const requestKey = packageRequestKey(task);
        if (state.requested.has(requestKey)) {
            return;
        }
        state.requested.add(requestKey);

        validatePackageName(task.name);

        let staged = null;
        try {
            const locked = findLockedDependency(state.lockFile, task.name, task.spec);
            staged = isGitHubRepositoryPackage(task.name)
                ? stageGitHubPackage(task, state, locked)
                : stageNpmPackage(task, state, locked);

            const installResult = installStagedPackage(staged, state);
            rememberResolvedPackage(task, installResult.manifest, staged.source, staged.installVersion, state);
            rememberInstalledPackage(task, installResult, state);

            const dependencies = normalizeDependencies(installResult.manifest.dependencies);
            const dependencyNames = Object.keys(dependencies).sort();
            for (const depName of dependencyNames) {
                installPackageRequest({ name: depName, spec: dependencies[depName] }, state);
            }
        } finally {
            if (staged && staged.tempRoot) {
                cleanupPath(staged.tempRoot);
                cleanupTempRootBase(state.tempRootBase);
            }
        }
    }

    function stageGitHubPackage(task, state, locked) {
        const source = resolveGitHubPackageSource(task, state, locked);
        const tempRoot = allocateTempRoot(state.tempRootBase, task.name.replace(/[\/]/g, '-'));
        const stageDir = path.join(tempRoot, 'stage');
        try {
            downloadGitHubDirectory(source, state, stageDir);
            return finalizeGitHubStage(task, locked, source, tempRoot, stageDir);
        } catch (err) {
            cleanupPath(tempRoot);
            cleanupTempRootBase(state.tempRootBase);
            throw err;
        }
    }

    function finalizeGitHubStage(task, locked, source, tempRoot, stageDir) {
        const packageRoot = stageDir;
        const manifest = readJsonFile(path.join(stageDir, 'package.json'));
        const requestedRef = parseGitHubRequestedSpecifier(task.spec);

        if (manifest.name !== source.repo) {
            throw new Error(`GitHub package name mismatch: expected ${source.repo}, got ${manifest.name}`);
        }
        if (locked && locked.version && !task.refreshLatest && !task.spec && source.ref !== locked.version) {
            throw new Error(`Locked GitHub package tag mismatch for ${task.name}: expected ${locked.version}, got ${source.ref}`);
        }
        if (requestedRef.ref && source.ref !== requestedRef.ref) {
            throw new Error(`Requested GitHub ${requestedRef.refType} ${requestedRef.ref} does not match resolved ref ${source.ref}`);
        }

        return {
            packageName: task.name,
            manifest,
            packageRoot,
            tempRoot,
            source: formatGitHubPackageSource(source),
            installVersion: source.ref,
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

        const tempRoot = allocateTempRoot(state.tempRootBase, task.name.replace(/[\/]/g, '-'));
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
            packageName: task.name,
            manifest,
            packageRoot,
            tempRoot,
            source: tarballUrl,
            installVersion: resolvedVersion,
        };
    }

    function installStagedPackage(staged, state) {
        const targetDir = packageInstallPath(state.installRoot, staged.packageName);
        const installedManifest = readInstalledManifest(targetDir);
        const installLabel = formatInstalledPackageLabel(staged);

        if (installedManifest && canReuseInstalledPackage(installedManifest, staged)) {
            console.println(`Up to date: ${installLabel}`);
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

        console.println(`Installed ${installLabel}`);

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
            const nextManifest = stripLegacyPackageCommandMetadata({
                ...ensureProjectManifest(state),
                scripts: stripManagedPackageCommandScripts(ensureProjectManifest(state).scripts, state.installRoot),
                dependencies: manifestDependencies,
            });
            writeJsonFile(state.manifestPath, nextManifest);
            state.projectManifest = nextManifest;
        }

        syncInstalledPackageBinWrappers(state);

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
            dependencies[rootPlan.requestedTask.name] = manifestSpecifier(rootPlan.requestedTask, resolved);
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
            scripts: {},
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

    function rememberResolvedPackage(task, manifest, resolvedUrl, resolvedVersion, state) {
        state.resolvedPackages[task.name] = {
            version: resolvedVersion || manifest.version,
            resolved: resolvedUrl,
            specifier: task.spec || '',
            requires: sortRecord(normalizeDependencies(manifest.dependencies)),
        };
    }

    function rememberInstalledPackage(task, installResult, state) {
        state.installedPackages[task.name] = {
            packageName: task.name,
            manifest: installResult.manifest,
            targetDir: installResult.targetDir,
        };
    }

    function writeScriptWrapper(wrapper) {
        fs.mkdirSync(path.dirname(wrapper.filePath), { recursive: true });
        fs.writeFileSync(wrapper.filePath, wrapper.content, 'utf8');
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
        if (spec && !matchesRequestedVersion(packageName, entry.version, spec)) {
            return null;
        }
        return {
            version: entry.version,
            resolved: entry.resolved,
            dependencies: normalizeDependencies(entry.dependencies),
        };
    }

    function manifestSpecifier(task, resolved) {
        if (isGitHubRepositoryPackage(task.name)) {
            const requestedRef = parseGitHubRequestedSpecifier(task.spec);
            if (requestedRef.ref) {
                return formatGitHubRequestedSpecifier(requestedRef.refType, requestedRef.ref);
            }
            const lockedSource = parseGitHubPackageSource(resolved.resolved);
            if (lockedSource && lockedSource.refType && lockedSource.ref) {
                return formatGitHubRequestedSpecifier(lockedSource.refType, lockedSource.ref);
            }
            return resolved.version;
        }
        if (task.spec) {
            return task.spec;
        }
        return `^${resolved.version}`;
    }

    function canReuseInstalledPackage(installedManifest, staged) {
        if (!staged.installVersion || staged.installVersion !== staged.manifest.version) {
            return false;
        }
        return installedManifest.version === staged.installVersion;
    }

    function getLockRootPackage(lockFile) {
        if (!lockFile || !isRecord(lockFile.packages) || !isRecord(lockFile.packages[''])) {
            return null;
        }
        return lockFile.packages[''];
    }

    function allocateTempRoot(baseDir, label) {
        const safeLabel = label.replace(/[^a-zA-Z0-9._-]+/g, '_');
        const tempRoot = path.join(baseDir, `${safeLabel}-${Date.now()}-${Math.floor(Math.random() * 100000)}`);
        fs.mkdirSync(tempRoot, { recursive: true });
        return tempRoot;
    }

    function cleanupPath(targetPath) {
        if (fs.existsSync(targetPath)) {
            fs.rmSync(targetPath, { recursive: true, force: true });
        }
    }

    function cleanupTempRootBase(targetPath) {
        try {
            fs.rmdirSync(targetPath);
        } catch (err) {
            // Ignore non-empty or missing temp roots.
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
        if (isGitHubRepositoryPackage(name)) {
            return;
        }
        if (!/^(?:@[A-Za-z0-9._-]+\/)?[A-Za-z0-9._-]+$/.test(name)) {
            throw new Error(`Invalid package name: ${name}`);
        }
    }

    function parseRequestedPackage(value) {
        if (typeof value !== 'string' || value.length === 0) {
            throw new Error('Package name is required');
        }

        const explicitGitHubPackage = parseGitHubRequestedPackage(value);
        if (explicitGitHubPackage) {
            return explicitGitHubPackage;
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
        if (isGitHubRepositoryPackage(task.name)) {
            return task.spec || '';
        }
        return task.spec || 'latest';
    }

    function isGitHubRepositoryPackage(name) {
        return /^github\.com\/[A-Za-z0-9._-]+\/[A-Za-z0-9._-]+$/.test(name);
    }

    function parseGitHubRepositoryPackage(name) {
        if (!isGitHubRepositoryPackage(name)) {
            return null;
        }
        const segments = name.split('/');
        return {
            owner: segments[1],
            repo: segments[2],
        };
    }

    function resolveGitHubPackageSource(task, state, locked) {
        const parsed = parseGitHubRepositoryPackage(task.name);
        if (!parsed) {
            throw new Error(`Invalid GitHub repository package: ${task.name}`);
        }

        let ref = null;
        const requestedRef = parseGitHubRequestedSpecifier(task.spec);
        if (requestedRef.ref) {
            ref = { ref: requestedRef.ref, refType: requestedRef.refType };
        }
        if (!ref && task.refreshLatest) {
            ref = resolveLatestGitHubRef(parsed, state);
        }
        if (!ref && locked && typeof locked.resolved === 'string') {
            const lockedSource = parseGitHubPackageSource(locked.resolved);
            if (lockedSource && lockedSource.owner === parsed.owner && lockedSource.repo === parsed.repo) {
                ref = {
                    ref: lockedSource.ref,
                    refType: lockedSource.refType || 'tag',
                };
            }
        }
        if (!ref && locked && typeof locked.version === 'string' && locked.version.length > 0) {
            ref = { ref: locked.version, refType: 'tag' };
        }
        if (!ref) {
            ref = resolveLatestGitHubRef(parsed, state);
        }

        return {
            owner: parsed.owner,
            repo: parsed.repo,
            ref: ref.ref,
            refType: ref.refType,
            projectPath: '',
        };
    }

    function resolveLatestGitHubRef(source, state) {
        const packageLabel = `github.com/${source.owner}/${source.repo}`;
        const tagsResult = tryGetGitHubJson(`${state.githubApiUrl}/repos/${encodeURIComponent(source.owner)}/${encodeURIComponent(source.repo)}/tags`);
        if (!tagsResult.error) {
            const payload = tagsResult.value;
            if (Array.isArray(payload) && payload.length > 0) {
                if (!isRecord(payload[0]) || typeof payload[0].name !== 'string' || payload[0].name.length === 0) {
                    throw new Error(`Invalid GitHub tag entry for ${packageLabel}`);
                }
                return { ref: payload[0].name, refType: 'tag' };
            }
        }

        const repoResult = tryGetGitHubJson(`${state.githubApiUrl}/repos/${encodeURIComponent(source.owner)}/${encodeURIComponent(source.repo)}`);
        if (!repoResult.error) {
            const repoPayload = repoResult.value;
            if (isRecord(repoPayload) && typeof repoPayload.default_branch === 'string' && repoPayload.default_branch.length > 0) {
                return { ref: repoPayload.default_branch, refType: 'branch' };
            }
        }

        const reasons = [];
        if (tagsResult.error) {
            reasons.push(`tags lookup failed: ${tagsResult.error.message}`);
        } else {
            reasons.push('no tags found');
        }
        if (repoResult.error) {
            reasons.push(`default branch lookup failed: ${repoResult.error.message}`);
        } else {
            reasons.push('default branch missing from repository metadata');
        }

        let hint = '';
        if (isGitHubNotFoundError(tagsResult.error) || isGitHubNotFoundError(repoResult.error)) {
            hint = ' Check that the repository name is correct and that the repository is public.';
        }
        throw new Error(`Unable to resolve GitHub ref for ${packageLabel} (${reasons.join('; ')})${hint}`);
    }

    function isGitHubNotFoundError(err) {
        return !!(err && err.statusCode === 404);
    }

    function formatGitHubPackageSource(source) {
        const refType = source.refType === 'branch' ? 'branch' : 'tag';
        return `github.com/${source.owner}/${source.repo}#${refType}=${source.ref}`;
    }

    function formatInstalledPackageLabel(staged) {
        if (isGitHubRepositoryPackage(staged.packageName) && staged.source) {
            const parsed = parseGitHubPackageSource(staged.source);
            if (parsed && parsed.refType && parsed.ref) {
                return formatGitHubPackageSource(parsed);
            }
        }
        return `${staged.packageName}@${staged.installVersion}`;
    }

    function parseGitHubPackageSource(sourceValue) {
        const value = String(sourceValue || '').trim();
        let match = /^github\.com\/([A-Za-z0-9._-]+)\/([A-Za-z0-9._-]+)#(tag|branch)=(.+)$/.exec(value);
        if (match) {
            return {
                owner: match[1],
                repo: match[2],
                refType: match[3],
                ref: match[4],
                projectPath: '',
            };
        }
        match = /^github\.com\/([A-Za-z0-9._-]+)\/([A-Za-z0-9._-]+)@(.+)$/.exec(value);
        if (!match) {
            return null;
        }
        return {
            owner: match[1],
            repo: match[2],
            refType: 'tag',
            ref: match[3],
            projectPath: '',
        };
    }

    function parseGitHubRequestedPackage(value) {
        const match = /^(github\.com\/[A-Za-z0-9._-]+\/[A-Za-z0-9._-]+)(?:(?:#(tag|branch)=(.+))|(?:@(.+)))?$/.exec(String(value || '').trim());
        if (!match) {
            return null;
        }
        return {
            name: match[1],
            spec: match[2] ? `#${match[2]}=${match[3]}` : (match[4] ? `#tag=${match[4]}` : ''),
        };
    }

    function parseGitHubRequestedSpecifier(spec) {
        const rawSpec = String(spec || '').trim();
        if (!rawSpec) {
            return { ref: '', refType: '' };
        }
        const match = /^#(tag|branch)=(.+)$/.exec(rawSpec);
        if (match) {
            return {
                refType: match[1],
                ref: match[2],
            };
        }
        return {
            refType: 'tag',
            ref: rawSpec,
        };
    }

    function formatGitHubRequestedSpecifier(refType, ref) {
        const normalizedType = refType === 'branch' ? 'branch' : 'tag';
        return `#${normalizedType}=${ref}`;
    }

    function tryGetGitHubJson(url) {
        try {
            return {
                value: httpGetJson(url, {
                    Accept: 'application/vnd.github+json',
                    'User-Agent': 'neo-pkg',
                }),
                error: null,
            };
        } catch (err) {
            return {
                value: null,
                error: err,
            };
        }
    }

    function resolveInstallDirectory(invocationCwd, installDir, globalInstall) {
        if (globalInstall) {
            return GLOBAL_PROJECT_DIR;
        }
        if (typeof installDir !== 'string' || installDir.trim().length === 0) {
            return invocationCwd;
        }
        const resolved = path.resolve(invocationCwd, installDir.trim());
        if (fs.existsSync(resolved) && !fs.statSync(resolved).isDirectory()) {
            throw new Error(`Install target is not a directory: ${resolved}`);
        }
        return resolved;
    }

    function resolveTempRootBase(invocationCwd) {
        const configuredTempDir = process.env.get('PKG_TMPDIR') || process.env.get('TMPDIR');
        if (typeof configuredTempDir === 'string' && configuredTempDir.trim().length > 0) {
            return path.resolve(configuredTempDir.trim(), '.pkg-tmp');
        }

        const homeDir = process.env.get('HOME');
        if (typeof homeDir === 'string' && homeDir.trim().length > 0 && homeDir.trim() !== '/work') {
            return path.resolve(homeDir.trim(), '.pkg-tmp');
        }

        return path.resolve(invocationCwd, '.pkg-tmp');
    }

    function prepareProjectDirectory(invocationCwd, installDir, globalInstall) {
        const cwd = resolveInstallDirectory(invocationCwd, installDir, globalInstall);
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
        const encodedPath = encodePathSegments(repoPath);
        const apiUrl = encodedPath.length > 0
            ? `${state.githubApiUrl}/repos/${encodeURIComponent(source.owner)}/${encodeURIComponent(source.repo)}/contents/${encodedPath}?ref=${encodeURIComponent(source.ref)}`
            : `${state.githubApiUrl}/repos/${encodeURIComponent(source.owner)}/${encodeURIComponent(source.repo)}/contents?ref=${encodeURIComponent(source.ref)}`;
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

    function relativeProjectPath(entryPath, projectPath) {
        if (!projectPath) {
            return entryPath;
        }
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

    function matchesRequestedVersion(packageName, version, spec) {
        if (isGitHubRepositoryPackage(packageName)) {
            return String(version) === String(parseGitHubRequestedSpecifier(spec).ref);
        }
        return satisfiesVersion(version, spec);
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

    function normalizeScripts(value) {
        if (!isRecord(value)) {
            return {};
        }
        const normalized = {};
        for (const key of Object.keys(value)) {
            const scriptLine = value[key];
            if (typeof scriptLine !== 'string') {
                continue;
            }
            const trimmed = scriptLine.trim();
            if (trimmed.length === 0) {
                continue;
            }
            normalized[key] = trimmed;
        }
        return normalized;
    }

    function normalizeBinCommands(manifest) {
        if (!isRecord(manifest)) {
            return {};
        }
        if (typeof manifest.bin === 'string' && manifest.bin.trim().length > 0) {
            if (typeof manifest.name !== 'string' || manifest.name.trim().length === 0) {
                return {};
            }
            return { [buildPackageCommandKey(manifest.name)]: manifest.bin.trim() };
        }
        if (!isRecord(manifest.bin)) {
            return {};
        }
        const normalized = {};
        for (const alias of Object.keys(manifest.bin)) {
            const target = manifest.bin[alias];
            if (!isValidPackageCommandKey(alias) || typeof target !== 'string' || target.trim().length === 0) {
                continue;
            }
            normalized[alias] = target.trim();
        }
        return sortRecord(normalized);
    }

    function cloneDependencies(value) {
        return { ...normalizeDependencies(value) };
    }

    function cloneScripts(value) {
        return { ...normalizeScripts(value) };
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

    function packageRequestKey(task) {
        return `${task.name}@${task.spec || ''}`;
    }

    function stripManagedPackageCommandScripts(scriptsValue, installRoot) {
        const scripts = cloneScripts(scriptsValue);
        for (const commandKey of Object.keys(scripts)) {
            if (isManagedPackageCommandScript(commandKey, scripts[commandKey], installRoot)) {
                delete scripts[commandKey];
            }
        }
        return sortRecord(scripts);
    }

    function isManagedPackageCommandScript(commandKey, scriptLine, installRoot) {
        return scriptLine === `./node_modules/.bin/${commandKey}.js`
            && readPackageCommandWrapperOwner(packageCommandWrapperPath(installRoot, commandKey)).length > 0;
    }

    function syncInstalledPackageBinWrappers(state) {
        for (const packageName of Object.keys(state.installedPackages).sort()) {
            const installedPackage = state.installedPackages[packageName];
            removePackageCommandWrappersByOwner(packageName, state);
            const binCommands = normalizeBinCommands(installedPackage.manifest);
            for (const alias of Object.keys(binCommands)) {
                const wrapperPath = packageCommandWrapperPath(state.installRoot, alias);
                const wrapperOwner = readPackageCommandWrapperOwner(wrapperPath);
                if (wrapperOwner && wrapperOwner !== packageName) {
                    console.println(`Warning: package bin alias ${alias} from ${packageName} conflicts with ${wrapperOwner}; skipped ${path.relative(state.cwd, wrapperPath)}`);
                    continue;
                }
                if (!wrapperOwner && fs.existsSync(wrapperPath)) {
                    console.println(`Warning: package bin alias ${alias} from ${packageName} conflicts with existing wrapper; skipped ${path.relative(state.cwd, wrapperPath)}`);
                    continue;
                }
                writeScriptWrapper({
                    filePath: wrapperPath,
                    content: buildPackageBinWrapperContent(alias, packageName, wrapperPath, installedPackage.targetDir, binCommands[alias]),
                });
            }
        }
    }

    function stripLegacyPackageCommandMetadata(manifest) {
        const nextManifest = { ...manifest };
        if (isRecord(nextManifest.pkg)) {
            const nextPkg = { ...nextManifest.pkg };
            delete nextPkg.commands;
            if (Object.keys(nextPkg).length > 0) {
                nextManifest.pkg = nextPkg;
            } else {
                delete nextManifest.pkg;
            }
        }
        return nextManifest;
    }

    function isValidPackageCommandKey(value) {
        return /^[A-Za-z0-9._-]+$/.test(String(value || '').trim());
    }

    function buildPackageCommandKey(packageName) {
        if (isGitHubRepositoryPackage(packageName)) {
            const parsed = parseGitHubRepositoryPackage(packageName);
            return parsed.repo;
        }
        if (packageName.startsWith('@')) {
            return packageName.split('/')[1];
        }
        const slashIndex = packageName.lastIndexOf('/');
        if (slashIndex >= 0) {
            return packageName.slice(slashIndex + 1);
        }
        return packageName;
    }

    function packageCommandWrapperPath(installRoot, commandKey) {
        return path.join(installRoot, '.bin', `${commandKey}.js`);
    }

    function readPackageCommandWrapperOwner(filePath) {
        if (!fs.existsSync(filePath)) {
            return '';
        }
        try {
            if (!fs.statSync(filePath).isFile()) {
                return '';
            }
        } catch (err) {
            return '';
        }
        const content = fs.readFileSync(filePath, 'utf8');
        const firstLine = String(content).split('\n')[0] || '';
        const prefix = '// neo-pkg-wrapper-owner:';
        if (!firstLine.startsWith(prefix)) {
            return '';
        }
        return firstLine.slice(prefix.length).trim();
    }

    function removePackageCommandWrapper(commandKey, state) {
        const wrapperPath = packageCommandWrapperPath(state.installRoot, commandKey);
        if (readPackageCommandWrapperOwner(wrapperPath)) {
            cleanupPath(wrapperPath);
        }
    }

    function removePackageCommandWrappersByOwner(packageName, state) {
        const binDir = path.join(state.installRoot, '.bin');
        if (!fs.existsSync(binDir) || !fs.statSync(binDir).isDirectory()) {
            return;
        }
        for (const entry of fs.readdirSync(binDir)) {
            const filePath = path.join(binDir, entry);
            if (readPackageCommandWrapperOwner(filePath) === packageName) {
                cleanupPath(filePath);
            }
        }
    }

    function buildPackageBinWrapperContent(commandKey, packageName, wrapperPath, packageDir, targetPath) {
        const relativePackageDir = path.relative(path.dirname(wrapperPath), packageDir) || '.';
        return [
            `// neo-pkg-wrapper-owner:${packageName}`,
            '(() => {',
            "    'use strict';",
            '',
            "    const process = require('process');",
            "    const path = require('path');",
            '',
            `    const packageDir = path.join(path.dirname(process.argv[1]), ${JSON.stringify(relativePackageDir)});`,
            `    const packageCommand = ${JSON.stringify(commandKey)};`,
            `    const targetPath = ${JSON.stringify(targetPath)};`,
            "    const args = process.argv.slice(2);",
            '    process.chdir(packageDir);',
            '    const command = path.isAbsolute(targetPath) ? targetPath : path.join(packageDir, targetPath);',
            '    const exitCode = process.exec(command, ...args);',
            '    if (exitCode instanceof Error) {',
            '        throw exitCode;',
            '    }',
            '    if (exitCode !== 0) {',
            '        process.exit(exitCode);',
            '    }',
            '})();',
            '',
        ].join('\n');
    }
})();