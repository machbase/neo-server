'use strict';

// POSIX-style path implementation for JSH
// Provides cross-platform path utilities similar to Node.js path module

const CHAR_FORWARD_SLASH = 47; // /
const CHAR_BACKWARD_SLASH = 92; // \
const CHAR_DOT = 46; // .
const CHAR_COLON = 58; // :

function isPosixPathSeparator(code) {
    return code === CHAR_FORWARD_SLASH;
}

function isPathSeparator(code) {
    return code === CHAR_FORWARD_SLASH || code === CHAR_BACKWARD_SLASH;
}

function isWindowsDeviceRoot(code) {
    return (code >= 65 && code <= 90) || (code >= 97 && code <= 122); // A-Z or a-z
}

// Resolves . and .. elements in a path with directory names
function normalizeString(path, allowAboveRoot, separator, isPathSeparator) {
    let res = '';
    let lastSegmentLength = 0;
    let lastSlash = -1;
    let dots = 0;
    let code = 0;
    
    for (let i = 0; i <= path.length; ++i) {
        if (i < path.length) {
            code = path.charCodeAt(i);
        } else if (isPathSeparator(code)) {
            break;
        } else {
            code = CHAR_FORWARD_SLASH;
        }
        
        if (isPathSeparator(code)) {
            if (lastSlash === i - 1 || dots === 1) {
                // NOOP
            } else if (dots === 2) {
                if (res.length < 2 || lastSegmentLength !== 2 ||
                    res.charCodeAt(res.length - 1) !== CHAR_DOT ||
                    res.charCodeAt(res.length - 2) !== CHAR_DOT) {
                    if (res.length > 2) {
                        const lastSlashIndex = res.lastIndexOf(separator);
                        if (lastSlashIndex === -1) {
                            res = '';
                            lastSegmentLength = 0;
                        } else {
                            res = res.slice(0, lastSlashIndex);
                            lastSegmentLength = res.length - 1 - res.lastIndexOf(separator);
                        }
                        lastSlash = i;
                        dots = 0;
                        continue;
                    } else if (res.length !== 0) {
                        res = '';
                        lastSegmentLength = 0;
                        lastSlash = i;
                        dots = 0;
                        continue;
                    }
                }
                if (allowAboveRoot) {
                    res += res.length > 0 ? `${separator}..` : '..';
                    lastSegmentLength = 2;
                }
            } else {
                if (res.length > 0) {
                    res += `${separator}${path.slice(lastSlash + 1, i)}`;
                } else {
                    res = path.slice(lastSlash + 1, i);
                }
                lastSegmentLength = i - lastSlash - 1;
            }
            lastSlash = i;
            dots = 0;
        } else if (code === CHAR_DOT && dots !== -1) {
            ++dots;
        } else {
            dots = -1;
        }
    }
    return res;
}

// POSIX implementation
const posix = {
    sep: '/',
    delimiter: ':',
    
    resolve(...args) {
        let resolvedPath = '';
        let resolvedAbsolute = false;
        
        for (let i = args.length - 1; i >= -1 && !resolvedAbsolute; i--) {
            const path = i >= 0 ? args[i] : process.cwd();
            
            if (typeof path !== 'string') {
                throw new TypeError('Arguments to path.resolve must be strings');
            }
            
            if (!path) {
                continue;
            }
            
            resolvedPath = `${path}/${resolvedPath}`;
            resolvedAbsolute = path.charCodeAt(0) === CHAR_FORWARD_SLASH;
        }
        
        resolvedPath = normalizeString(resolvedPath, !resolvedAbsolute, '/', isPosixPathSeparator);
        
        if (resolvedAbsolute) {
            return `/${resolvedPath}`;
        }
        return resolvedPath.length > 0 ? resolvedPath : '.';
    },
    
    normalize(path) {
        if (typeof path !== 'string') {
            throw new TypeError('Path must be a string');
        }
        
        if (path.length === 0) {
            return '.';
        }
        
        const isAbsolute = path.charCodeAt(0) === CHAR_FORWARD_SLASH;
        const trailingSeparator = path.charCodeAt(path.length - 1) === CHAR_FORWARD_SLASH;
        
        path = normalizeString(path, !isAbsolute, '/', isPosixPathSeparator);
        
        if (path.length === 0) {
            if (isAbsolute) {
                return '/';
            }
            return trailingSeparator ? './' : '.';
        }
        if (trailingSeparator) {
            path += '/';
        }
        
        return isAbsolute ? `/${path}` : path;
    },
    
    isAbsolute(path) {
        if (typeof path !== 'string') {
            throw new TypeError('Path must be a string');
        }
        return path.length > 0 && path.charCodeAt(0) === CHAR_FORWARD_SLASH;
    },
    
    join(...args) {
        if (args.length === 0) {
            return '.';
        }
        
        let joined;
        for (let i = 0; i < args.length; ++i) {
            const arg = args[i];
            if (typeof arg !== 'string') {
                throw new TypeError('Arguments to path.join must be strings');
            }
            if (arg.length > 0) {
                if (joined === undefined) {
                    joined = arg;
                } else {
                    joined += `/${arg}`;
                }
            }
        }
        
        if (joined === undefined) {
            return '.';
        }
        
        return posix.normalize(joined);
    },
    
    relative(from, to) {
        if (typeof from !== 'string') {
            throw new TypeError('from must be a string');
        }
        if (typeof to !== 'string') {
            throw new TypeError('to must be a string');
        }
        
        if (from === to) {
            return '';
        }
        
        from = posix.resolve(from);
        to = posix.resolve(to);
        
        if (from === to) {
            return '';
        }
        
        const fromStart = 1;
        const fromEnd = from.length;
        const fromLen = fromEnd - fromStart;
        const toStart = 1;
        const toLen = to.length - toStart;
        
        const length = (fromLen < toLen ? fromLen : toLen);
        let lastCommonSep = -1;
        let i = 0;
        for (; i < length; i++) {
            const fromCode = from.charCodeAt(fromStart + i);
            if (fromCode !== to.charCodeAt(toStart + i)) {
                break;
            } else if (fromCode === CHAR_FORWARD_SLASH) {
                lastCommonSep = i;
            }
        }
        if (i === length) {
            if (toLen > length) {
                if (to.charCodeAt(toStart + i) === CHAR_FORWARD_SLASH) {
                    return to.slice(toStart + i + 1);
                }
                if (i === 0) {
                    return to.slice(toStart + i);
                }
            } else if (fromLen > length) {
                if (from.charCodeAt(fromStart + i) === CHAR_FORWARD_SLASH) {
                    lastCommonSep = i;
                } else if (i === 0) {
                    lastCommonSep = 0;
                }
            }
        }
        
        let out = '';
        for (i = fromStart + lastCommonSep + 1; i <= fromEnd; ++i) {
            if (i === fromEnd || from.charCodeAt(i) === CHAR_FORWARD_SLASH) {
                out += out.length === 0 ? '..' : '/..';
            }
        }
        
        return `${out}${to.slice(toStart + lastCommonSep)}`;
    },
    
    dirname(path) {
        if (typeof path !== 'string') {
            throw new TypeError('Path must be a string');
        }
        
        if (path.length === 0) {
            return '.';
        }
        
        const hasRoot = path.charCodeAt(0) === CHAR_FORWARD_SLASH;
        let end = -1;
        let matchedSlash = true;
        
        for (let i = path.length - 1; i >= 1; --i) {
            if (path.charCodeAt(i) === CHAR_FORWARD_SLASH) {
                if (!matchedSlash) {
                    end = i;
                    break;
                }
            } else {
                matchedSlash = false;
            }
        }
        
        if (end === -1) {
            return hasRoot ? '/' : '.';
        }
        if (hasRoot && end === 1) {
            return '//';
        }
        return path.slice(0, end);
    },
    
    basename(path, ext) {
        if (typeof path !== 'string') {
            throw new TypeError('Path must be a string');
        }
        if (ext !== undefined && typeof ext !== 'string') {
            throw new TypeError('ext must be a string');
        }
        
        let start = 0;
        let end = -1;
        let matchedSlash = true;
        
        if (ext !== undefined && ext.length > 0 && ext.length <= path.length) {
            if (ext === path) {
                return '';
            }
            let extIdx = ext.length - 1;
            let firstNonSlashEnd = -1;
            for (let i = path.length - 1; i >= 0; --i) {
                const code = path.charCodeAt(i);
                if (code === CHAR_FORWARD_SLASH) {
                    if (!matchedSlash) {
                        start = i + 1;
                        break;
                    }
                } else {
                    if (firstNonSlashEnd === -1) {
                        matchedSlash = false;
                        firstNonSlashEnd = i + 1;
                    }
                    if (extIdx >= 0) {
                        if (code === ext.charCodeAt(extIdx)) {
                            if (--extIdx === -1) {
                                end = i;
                            }
                        } else {
                            extIdx = -1;
                            end = firstNonSlashEnd;
                        }
                    }
                }
            }
            
            if (start === end) {
                end = firstNonSlashEnd;
            } else if (end === -1) {
                end = path.length;
            }
            return path.slice(start, end);
        }
        
        for (let i = path.length - 1; i >= 0; --i) {
            if (path.charCodeAt(i) === CHAR_FORWARD_SLASH) {
                if (!matchedSlash) {
                    start = i + 1;
                    break;
                }
            } else if (end === -1) {
                matchedSlash = false;
                end = i + 1;
            }
        }
        
        if (end === -1) {
            return '';
        }
        return path.slice(start, end);
    },
    
    extname(path) {
        if (typeof path !== 'string') {
            throw new TypeError('Path must be a string');
        }
        
        let startDot = -1;
        let startPart = 0;
        let end = -1;
        let matchedSlash = true;
        let preDotState = 0;
        
        for (let i = path.length - 1; i >= 0; --i) {
            const code = path.charCodeAt(i);
            if (code === CHAR_FORWARD_SLASH) {
                if (!matchedSlash) {
                    startPart = i + 1;
                    break;
                }
                continue;
            }
            if (end === -1) {
                matchedSlash = false;
                end = i + 1;
            }
            if (code === CHAR_DOT) {
                if (startDot === -1) {
                    startDot = i;
                } else if (preDotState !== 1) {
                    preDotState = 1;
                }
            } else if (startDot !== -1) {
                preDotState = -1;
            }
        }
        
        if (startDot === -1 ||
            end === -1 ||
            preDotState === 0 ||
            (preDotState === 1 && startDot === end - 1 && startDot === startPart + 1)) {
            return '';
        }
        return path.slice(startDot, end);
    },
    
    parse(path) {
        if (typeof path !== 'string') {
            throw new TypeError('Path must be a string');
        }
        
        const ret = { root: '', dir: '', base: '', ext: '', name: '' };
        if (path.length === 0) {
            return ret;
        }
        
        const isAbsolute = path.charCodeAt(0) === CHAR_FORWARD_SLASH;
        let start;
        if (isAbsolute) {
            ret.root = '/';
            start = 1;
        } else {
            start = 0;
        }
        
        let startDot = -1;
        let startPart = 0;
        let end = -1;
        let matchedSlash = true;
        let i = path.length - 1;
        let preDotState = 0;
        
        for (; i >= start; --i) {
            const code = path.charCodeAt(i);
            if (code === CHAR_FORWARD_SLASH) {
                if (!matchedSlash) {
                    startPart = i + 1;
                    break;
                }
                continue;
            }
            if (end === -1) {
                matchedSlash = false;
                end = i + 1;
            }
            if (code === CHAR_DOT) {
                if (startDot === -1) {
                    startDot = i;
                } else if (preDotState !== 1) {
                    preDotState = 1;
                }
            } else if (startDot !== -1) {
                preDotState = -1;
            }
        }
        
        if (end !== -1) {
            const start2 = startPart === 0 && isAbsolute ? 1 : startPart;
            if (startDot === -1 ||
                preDotState === 0 ||
                (preDotState === 1 && startDot === end - 1 && startDot === startPart + 1)) {
                ret.base = ret.name = path.slice(start2, end);
            } else {
                ret.name = path.slice(start2, startDot);
                ret.base = path.slice(start2, end);
                ret.ext = path.slice(startDot, end);
            }
        }
        
        if (startPart > 0) {
            ret.dir = path.slice(0, startPart - 1);
        } else if (isAbsolute) {
            ret.dir = '/';
        }
        
        return ret;
    },
    
    format(pathObject) {
        if (pathObject === null || typeof pathObject !== 'object') {
            throw new TypeError('pathObject must be an object');
        }
        
        const dir = pathObject.dir || pathObject.root;
        const base = pathObject.base || `${pathObject.name || ''}${pathObject.ext || ''}`;
        
        if (!dir) {
            return base;
        }
        
        return dir === pathObject.root ? `${dir}${base}` : `${dir}/${base}`;
    }
};

// Windows implementation (for cross-platform compatibility)
const win32 = {
    sep: '\\',
    delimiter: ';',
    
    resolve(...args) {
        let resolvedDevice = '';
        let resolvedTail = '';
        let resolvedAbsolute = false;
        
        for (let i = args.length - 1; i >= -1; i--) {
            let path;
            if (i >= 0) {
                path = args[i];
                if (typeof path !== 'string') {
                    throw new TypeError('Arguments to path.resolve must be strings');
                }
                if (!path) {
                    continue;
                }
            } else if (!resolvedDevice) {
                path = process.cwd();
            } else {
                path = process.env['=' + resolvedDevice] || process.cwd();
                
                if (path === undefined ||
                    (path.slice(0, 2).toLowerCase() !== resolvedDevice.toLowerCase() &&
                     path.charCodeAt(2) === CHAR_BACKWARD_SLASH)) {
                    path = `${resolvedDevice}\\`;
                }
            }
            
            const len = path.length;
            let rootEnd = 0;
            let device = '';
            let isAbsolute = false;
            const code = path.charCodeAt(0);
            
            if (len === 1) {
                if (isPathSeparator(code)) {
                    rootEnd = 1;
                    isAbsolute = true;
                }
            } else if (isPathSeparator(code)) {
                isAbsolute = true;
                
                if (isPathSeparator(path.charCodeAt(1))) {
                    let j = 2;
                    let last = j;
                    while (j < len && !isPathSeparator(path.charCodeAt(j))) {
                        j++;
                    }
                    if (j < len && j !== last) {
                        const firstPart = path.slice(last, j);
                        last = j;
                        while (j < len && isPathSeparator(path.charCodeAt(j))) {
                            j++;
                        }
                        if (j < len && j !== last) {
                            last = j;
                            while (j < len && !isPathSeparator(path.charCodeAt(j))) {
                                j++;
                            }
                            if (j === len || j !== last) {
                                device = `\\\\${firstPart}\\${path.slice(last, j)}`;
                                rootEnd = j;
                            }
                        }
                    }
                } else {
                    rootEnd = 1;
                }
            } else if (isWindowsDeviceRoot(code) &&
                       path.charCodeAt(1) === CHAR_COLON) {
                device = path.slice(0, 2);
                rootEnd = 2;
                if (len > 2 && isPathSeparator(path.charCodeAt(2))) {
                    isAbsolute = true;
                    rootEnd = 3;
                }
            }
            
            if (device.length > 0) {
                if (resolvedDevice.length > 0) {
                    if (device.toLowerCase() !== resolvedDevice.toLowerCase()) {
                        continue;
                    }
                } else {
                    resolvedDevice = device;
                }
            }
            
            if (resolvedAbsolute) {
                if (resolvedDevice.length > 0) {
                    break;
                }
            } else {
                resolvedTail = `${path.slice(rootEnd)}\\${resolvedTail}`;
                resolvedAbsolute = isAbsolute;
                if (isAbsolute && resolvedDevice.length > 0) {
                    break;
                }
            }
        }
        
        resolvedTail = normalizeString(resolvedTail, !resolvedAbsolute, '\\', isPathSeparator);
        
        return resolvedAbsolute ?
            `${resolvedDevice}\\${resolvedTail}` :
            `${resolvedDevice}${resolvedTail}` || '.';
    },
    
    normalize(path) {
        if (typeof path !== 'string') {
            throw new TypeError('Path must be a string');
        }
        
        const len = path.length;
        if (len === 0) {
            return '.';
        }
        
        let rootEnd = 0;
        let device;
        let isAbsolute = false;
        const code = path.charCodeAt(0);
        
        if (len === 1) {
            return isPathSeparator(code) ? '\\' : path;
        }
        
        if (isPathSeparator(code)) {
            isAbsolute = true;
            
            if (isPathSeparator(path.charCodeAt(1))) {
                let j = 2;
                let last = j;
                while (j < len && !isPathSeparator(path.charCodeAt(j))) {
                    j++;
                }
                if (j < len && j !== last) {
                    const firstPart = path.slice(last, j);
                    last = j;
                    while (j < len && isPathSeparator(path.charCodeAt(j))) {
                        j++;
                    }
                    if (j < len && j !== last) {
                        last = j;
                        while (j < len && !isPathSeparator(path.charCodeAt(j))) {
                            j++;
                        }
                        if (j === len) {
                            return `\\\\${firstPart}\\${path.slice(last)}\\`;
                        }
                        if (j !== last) {
                            device = `\\\\${firstPart}\\${path.slice(last, j)}`;
                            rootEnd = j;
                        }
                    }
                }
            } else {
                rootEnd = 1;
            }
        } else if (isWindowsDeviceRoot(code) && path.charCodeAt(1) === CHAR_COLON) {
            device = path.slice(0, 2);
            rootEnd = 2;
            if (len > 2 && isPathSeparator(path.charCodeAt(2))) {
                isAbsolute = true;
                rootEnd = 3;
            }
        }
        
        let tail = rootEnd < len ?
            normalizeString(path.slice(rootEnd), !isAbsolute, '\\', isPathSeparator) :
            '';
        
        if (tail.length === 0 && !isAbsolute) {
            tail = '.';
        }
        if (tail.length > 0 && isPathSeparator(path.charCodeAt(len - 1))) {
            tail += '\\';
        }
        
        if (device === undefined) {
            return isAbsolute ? `\\${tail}` : tail;
        }
        return isAbsolute ? `${device}\\${tail}` : `${device}${tail}`;
    },
    
    isAbsolute(path) {
        if (typeof path !== 'string') {
            throw new TypeError('Path must be a string');
        }
        
        const len = path.length;
        if (len === 0) {
            return false;
        }
        
        const code = path.charCodeAt(0);
        return isPathSeparator(code) ||
            (len > 2 &&
             isWindowsDeviceRoot(code) &&
             path.charCodeAt(1) === CHAR_COLON &&
             isPathSeparator(path.charCodeAt(2)));
    },
    
    join(...args) {
        if (args.length === 0) {
            return '.';
        }
        
        let joined;
        let firstPart;
        for (let i = 0; i < args.length; ++i) {
            const arg = args[i];
            if (typeof arg !== 'string') {
                throw new TypeError('Arguments to path.join must be strings');
            }
            if (arg.length > 0) {
                if (joined === undefined) {
                    joined = firstPart = arg;
                } else {
                    joined += `\\${arg}`;
                }
            }
        }
        
        if (joined === undefined) {
            return '.';
        }
        
        let needsReplace = true;
        let slashCount = 0;
        if (typeof firstPart === 'string' && isPathSeparator(firstPart.charCodeAt(0))) {
            ++slashCount;
            const firstLen = firstPart.length;
            if (firstLen > 1 && isPathSeparator(firstPart.charCodeAt(1))) {
                ++slashCount;
                if (firstLen > 2) {
                    if (isPathSeparator(firstPart.charCodeAt(2))) {
                        ++slashCount;
                    } else {
                        needsReplace = false;
                    }
                }
            }
        }
        if (needsReplace) {
            while (slashCount < joined.length && isPathSeparator(joined.charCodeAt(slashCount))) {
                slashCount++;
            }
            if (slashCount >= 2) {
                joined = `\\${joined.slice(slashCount)}`;
            }
        }
        
        return win32.normalize(joined);
    },
    
    relative(from, to) {
        if (typeof from !== 'string') {
            throw new TypeError('from must be a string');
        }
        if (typeof to !== 'string') {
            throw new TypeError('to must be a string');
        }
        
        if (from === to) {
            return '';
        }
        
        from = win32.resolve(from);
        to = win32.resolve(to);
        
        if (from === to) {
            return '';
        }
        
        const fromOrig = from;
        const toOrig = to;
        from = from.toLowerCase();
        to = to.toLowerCase();
        
        if (from === to) {
            return '';
        }
        
        let fromStart = 0;
        while (fromStart < from.length && from.charCodeAt(fromStart) === CHAR_BACKWARD_SLASH) {
            fromStart++;
        }
        let fromEnd = from.length;
        while (fromEnd - 1 > fromStart && from.charCodeAt(fromEnd - 1) === CHAR_BACKWARD_SLASH) {
            fromEnd--;
        }
        const fromLen = fromEnd - fromStart;
        
        let toStart = 0;
        while (toStart < to.length && to.charCodeAt(toStart) === CHAR_BACKWARD_SLASH) {
            toStart++;
        }
        let toEnd = to.length;
        while (toEnd - 1 > toStart && to.charCodeAt(toEnd - 1) === CHAR_BACKWARD_SLASH) {
            toEnd--;
        }
        const toLen = toEnd - toStart;
        
        const length = (fromLen < toLen ? fromLen : toLen);
        let lastCommonSep = -1;
        let i = 0;
        for (; i < length; i++) {
            const fromCode = from.charCodeAt(fromStart + i);
            if (fromCode !== to.charCodeAt(toStart + i)) {
                break;
            } else if (fromCode === CHAR_BACKWARD_SLASH) {
                lastCommonSep = i;
            }
        }
        
        if (i !== length) {
            if (lastCommonSep === -1) {
                return toOrig;
            }
        } else {
            if (toLen > length) {
                if (to.charCodeAt(toStart + i) === CHAR_BACKWARD_SLASH) {
                    return toOrig.slice(toStart + i + 1);
                }
                if (i === 2) {
                    return toOrig.slice(toStart + i);
                }
            }
            if (fromLen > length) {
                if (from.charCodeAt(fromStart + i) === CHAR_BACKWARD_SLASH) {
                    lastCommonSep = i;
                } else if (i === 2) {
                    lastCommonSep = 3;
                }
            }
            if (lastCommonSep === -1) {
                lastCommonSep = 0;
            }
        }
        
        let out = '';
        for (i = fromStart + lastCommonSep + 1; i <= fromEnd; ++i) {
            if (i === fromEnd || from.charCodeAt(i) === CHAR_BACKWARD_SLASH) {
                out += out.length === 0 ? '..' : '\\..';
            }
        }
        
        toStart += lastCommonSep;
        
        if (out.length > 0) {
            return `${out}${toOrig.slice(toStart, toEnd)}`;
        }
        
        if (toOrig.charCodeAt(toStart) === CHAR_BACKWARD_SLASH) {
            ++toStart;
        }
        return toOrig.slice(toStart, toEnd);
    },
    
    dirname(path) {
        if (typeof path !== 'string') {
            throw new TypeError('Path must be a string');
        }
        
        const len = path.length;
        if (len === 0) {
            return '.';
        }
        
        let rootEnd = -1;
        let offset = 0;
        const code = path.charCodeAt(0);
        
        if (len === 1) {
            return isPathSeparator(code) ? path : '.';
        }
        
        if (isPathSeparator(code)) {
            rootEnd = offset = 1;
            
            if (isPathSeparator(path.charCodeAt(1))) {
                let j = 2;
                let last = j;
                while (j < len && !isPathSeparator(path.charCodeAt(j))) {
                    j++;
                }
                if (j < len && j !== last) {
                    last = j;
                    while (j < len && isPathSeparator(path.charCodeAt(j))) {
                        j++;
                    }
                    if (j < len && j !== last) {
                        last = j;
                        while (j < len && !isPathSeparator(path.charCodeAt(j))) {
                            j++;
                        }
                        if (j === len) {
                            return path;
                        }
                        if (j !== last) {
                            rootEnd = offset = j + 1;
                        }
                    }
                }
            }
        } else if (isWindowsDeviceRoot(code) && path.charCodeAt(1) === CHAR_COLON) {
            rootEnd = len > 2 && isPathSeparator(path.charCodeAt(2)) ? 3 : 2;
            offset = rootEnd;
        }
        
        let end = -1;
        let matchedSlash = true;
        for (let i = len - 1; i >= offset; --i) {
            if (isPathSeparator(path.charCodeAt(i))) {
                if (!matchedSlash) {
                    end = i;
                    break;
                }
            } else {
                matchedSlash = false;
            }
        }
        
        if (end === -1) {
            if (rootEnd === -1) {
                return '.';
            }
            end = rootEnd;
        }
        return path.slice(0, end);
    },
    
    basename(path, ext) {
        if (typeof path !== 'string') {
            throw new TypeError('Path must be a string');
        }
        if (ext !== undefined && typeof ext !== 'string') {
            throw new TypeError('ext must be a string');
        }
        
        let start = 0;
        let end = -1;
        let matchedSlash = true;
        
        if (path.length >= 2 &&
            isWindowsDeviceRoot(path.charCodeAt(0)) &&
            path.charCodeAt(1) === CHAR_COLON) {
            start = 2;
        }
        
        if (ext !== undefined && ext.length > 0 && ext.length <= path.length) {
            if (ext === path) {
                return '';
            }
            let extIdx = ext.length - 1;
            let firstNonSlashEnd = -1;
            for (let i = path.length - 1; i >= start; --i) {
                const code = path.charCodeAt(i);
                if (isPathSeparator(code)) {
                    if (!matchedSlash) {
                        start = i + 1;
                        break;
                    }
                } else {
                    if (firstNonSlashEnd === -1) {
                        matchedSlash = false;
                        firstNonSlashEnd = i + 1;
                    }
                    if (extIdx >= 0) {
                        if (code === ext.charCodeAt(extIdx)) {
                            if (--extIdx === -1) {
                                end = i;
                            }
                        } else {
                            extIdx = -1;
                            end = firstNonSlashEnd;
                        }
                    }
                }
            }
            
            if (start === end) {
                end = firstNonSlashEnd;
            } else if (end === -1) {
                end = path.length;
            }
            return path.slice(start, end);
        }
        
        for (let i = path.length - 1; i >= start; --i) {
            if (isPathSeparator(path.charCodeAt(i))) {
                if (!matchedSlash) {
                    start = i + 1;
                    break;
                }
            } else if (end === -1) {
                matchedSlash = false;
                end = i + 1;
            }
        }
        
        if (end === -1) {
            return '';
        }
        return path.slice(start, end);
    },
    
    extname(path) {
        if (typeof path !== 'string') {
            throw new TypeError('Path must be a string');
        }
        
        let start = 0;
        let startDot = -1;
        let startPart = 0;
        let end = -1;
        let matchedSlash = true;
        let preDotState = 0;
        
        if (path.length >= 2 &&
            path.charCodeAt(1) === CHAR_COLON &&
            isWindowsDeviceRoot(path.charCodeAt(0))) {
            start = startPart = 2;
        }
        
        for (let i = path.length - 1; i >= start; --i) {
            const code = path.charCodeAt(i);
            if (isPathSeparator(code)) {
                if (!matchedSlash) {
                    startPart = i + 1;
                    break;
                }
                continue;
            }
            if (end === -1) {
                matchedSlash = false;
                end = i + 1;
            }
            if (code === CHAR_DOT) {
                if (startDot === -1) {
                    startDot = i;
                } else if (preDotState !== 1) {
                    preDotState = 1;
                }
            } else if (startDot !== -1) {
                preDotState = -1;
            }
        }
        
        if (startDot === -1 ||
            end === -1 ||
            preDotState === 0 ||
            (preDotState === 1 && startDot === end - 1 && startDot === startPart + 1)) {
            return '';
        }
        return path.slice(startDot, end);
    },
    
    parse(path) {
        if (typeof path !== 'string') {
            throw new TypeError('Path must be a string');
        }
        
        const ret = { root: '', dir: '', base: '', ext: '', name: '' };
        const len = path.length;
        if (len === 0) {
            return ret;
        }
        
        let rootEnd = 0;
        let code = path.charCodeAt(0);
        
        if (len === 1) {
            if (isPathSeparator(code)) {
                ret.root = ret.dir = path;
                return ret;
            }
            ret.base = ret.name = path;
            return ret;
        }
        
        if (isPathSeparator(code)) {
            rootEnd = 1;
            if (isPathSeparator(path.charCodeAt(1))) {
                let j = 2;
                let last = j;
                while (j < len && !isPathSeparator(path.charCodeAt(j))) {
                    j++;
                }
                if (j < len && j !== last) {
                    last = j;
                    while (j < len && isPathSeparator(path.charCodeAt(j))) {
                        j++;
                    }
                    if (j < len && j !== last) {
                        last = j;
                        while (j < len && !isPathSeparator(path.charCodeAt(j))) {
                            j++;
                        }
                        if (j === len) {
                            rootEnd = j;
                        } else if (j !== last) {
                            rootEnd = j + 1;
                        }
                    }
                }
            }
        } else if (isWindowsDeviceRoot(code) && path.charCodeAt(1) === CHAR_COLON) {
            if (len <= 2) {
                ret.root = ret.dir = path;
                return ret;
            }
            rootEnd = 2;
            if (isPathSeparator(path.charCodeAt(2))) {
                if (len === 3) {
                    ret.root = ret.dir = path;
                    return ret;
                }
                rootEnd = 3;
            }
        }
        if (rootEnd > 0) {
            ret.root = path.slice(0, rootEnd);
        }
        
        let startDot = -1;
        let startPart = rootEnd;
        let end = -1;
        let matchedSlash = true;
        let i = path.length - 1;
        let preDotState = 0;
        
        for (; i >= rootEnd; --i) {
            code = path.charCodeAt(i);
            if (isPathSeparator(code)) {
                if (!matchedSlash) {
                    startPart = i + 1;
                    break;
                }
                continue;
            }
            if (end === -1) {
                matchedSlash = false;
                end = i + 1;
            }
            if (code === CHAR_DOT) {
                if (startDot === -1) {
                    startDot = i;
                } else if (preDotState !== 1) {
                    preDotState = 1;
                }
            } else if (startDot !== -1) {
                preDotState = -1;
            }
        }
        
        if (end !== -1) {
            if (startPart === 0 && rootEnd === 0) {
                ret.base = ret.name = path.slice(0, end);
            } else {
                ret.base = path.slice(startPart, end);
                if (startDot === -1 ||
                    preDotState === 0 ||
                    (preDotState === 1 && startDot === end - 1 && startDot === startPart + 1)) {
                    ret.name = ret.base;
                } else {
                    ret.name = path.slice(startPart, startDot);
                    ret.ext = path.slice(startDot, end);
                }
            }
        }
        
        if (startPart > 0 && startPart !== rootEnd) {
            ret.dir = path.slice(0, startPart - 1);
        } else {
            ret.dir = ret.root;
        }
        
        return ret;
    },
    
    format(pathObject) {
        if (pathObject === null || typeof pathObject !== 'object') {
            throw new TypeError('pathObject must be an object');
        }
        
        const dir = pathObject.dir || pathObject.root;
        const base = pathObject.base || `${pathObject.name || ''}${pathObject.ext || ''}`;
        
        if (!dir) {
            return base;
        }
        
        return dir === pathObject.root ? `${dir}${base}` : `${dir}\\${base}`;
    }
};

// Default to POSIX for JSH (Linux environment)
module.exports = posix;
module.exports.posix = posix;
module.exports.win32 = win32;
