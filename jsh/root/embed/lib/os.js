'use strict';

/**
 * OS module - Node.js compatible os module for JSH
 * Provides operating system-related utility methods and properties
 */

const _os = require('@jsh/os');

/**
 * Returns the operating system CPU architecture
 * @returns {string} CPU architecture (e.g., 'arm64', 'amd64', 'x64')
 */
function arch() {
    return _os.arch();
}

/**
 * Returns an array of objects containing information about each logical CPU core
 * @returns {Array<Object>} Array of CPU information objects
 */
function cpus() {
    return _os.cpus();
}

/**
 * Returns a string identifying the endianness of the CPU
 * @returns {string} 'BE' for big endian, 'LE' for little endian
 */
function endianness() {
    return _os.endianness();
}

/**
 * Returns the amount of free system memory in bytes
 * @returns {number} Free memory in bytes
 */
function freemem() {
    return _os.freemem();
}

/**
 * Returns the home directory of the current user
 * @returns {string} Home directory path
 */
function homedir() {
    return _os.homedir();
}

/**
 * Returns the hostname of the operating system
 * @returns {string} Hostname
 */
function hostname() {
    return _os.hostname();
}

/**
 * Returns an array containing the 1, 5, and 15 minute load averages
 * @returns {Array<number>} [1min, 5min, 15min] load averages
 */
function loadavg() {
    return _os.loadavg();
}

/**
 * Returns an object containing network interfaces that have been assigned a network address
 * @returns {Object} Network interfaces information
 */
function networkInterfaces() {
    return _os.networkInterfaces();
}

/**
 * Returns a string identifying the operating system platform
 * @returns {string} Platform (e.g., 'darwin', 'linux', 'windows')
 */
function platform() {
    return _os.platform();
}

/**
 * Returns the operating system release
 * @returns {string} OS release version
 */
function release() {
    return _os.release();
}

/**
 * Returns the operating system's default directory for temporary files
 * @returns {string} Temp directory path
 */
function tmpdir() {
    return _os.tmpdir();
}

/**
 * Returns the total amount of system memory in bytes
 * @returns {number} Total memory in bytes
 */
function totalmem() {
    return _os.totalmem();
}

/**
 * Returns the operating system name
 * @returns {string} OS type (e.g., 'Darwin', 'Linux', 'Windows_NT')
 */
function type() {
    return _os.type();
}

/**
 * Returns the system uptime in seconds
 * @returns {number} System uptime in seconds
 */
function uptime() {
    return _os.uptime();
}

/**
 * Returns information about the currently effective user
 * @param {Object} [options] - Optional configuration
 * @param {string} [options.encoding='utf8'] - Character encoding (currently not used)
 * @returns {Object} User information object
 */
function userInfo(options) {
    return _os.userInfo(options);
}

/**
 * OS constants - signals, priority levels, etc.
 */
const constants = _os.constants;

/**
 * End-of-line marker for the current operating system
 * '\n' on POSIX, '\r\n' on Windows
 */
const EOL = _os.EOL;

// Export all functions and constants
module.exports = {
    arch,
    cpus,
    endianness,
    freemem,
    homedir,
    hostname,
    loadavg,
    networkInterfaces,
    platform,
    release,
    tmpdir,
    totalmem,
    type,
    uptime,
    userInfo,
    constants,
    EOL
};
