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
 * Returns the system boot time as a timestamp
 * @returns {number} System boot time as a timestamp
 */
function bootTime() {
    return _os.bootTime();
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
 * Returns information about the host system
 * @returns {Object} Host information object
 */
function hostInfo() {
    return _os.hostInfo();
}

/**
 * Returns the number of CPU cores
 * @param {boolean} [logical=false] - Whether to count logical cores (default: false)
 * @returns {number} Number of CPU cores
 */
function cpuCounts(logical = false) {
    return _os.cpuCounts(logical);
}

/**
 * Returns the CPU usage percentage
 * @param {number} [intervalSec=0] - Interval in seconds to calculate CPU usage (default: 0)
 * @param {boolean} [perCPU=false] - Whether to return per-CPU usage (default: false)
 * @returns {number|Array<number>} CPU usage percentage or array of percentages per CPU
 */
function cpuPercent(intervalSec = 0, perCPU = false) {
    return _os.cpuPercent(intervalSec, perCPU);
}

/**
 * Returns information about disk partitions
 * @param {boolean} [all=false] - Whether to include all partitions (default: false)
 * @returns {Array<Object>} Array of disk partition information
 */
function diskPartitions(all = false) {
    return _os.diskPartitions(all);
}

/**
 * Returns information about disk usage for a given path
 * @param {string} path - Path to check disk usage
 * @returns {Object} Disk usage information
 */
function diskUsage(path) {
    return _os.diskUsage(path);
}

/**
 * Returns information about disk I/O counters
 * @param {Array<string>} [names=[]] - Names of disks to include (default: all)
 * @returns {Object} Disk I/O counters information
 */
function diskIOCounters(names = []) {
    return _os.diskIOCounters(names);
}

/**
 * Returns network protocol counters
 * @param {Array<string>} [proto=[]] - Protocols to include (default: all)
 * @returns {Object} Network protocol counters information
 */
function netProtoCounters(proto = []) {
    return _os.netProtoCounters(proto);
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
    cpuCounts,
    cpuPercent,
    diskPartitions,
    diskUsage,
    diskIOCounters,
    endianness,
    freemem,
    homedir,
    hostname,
    loadavg,
    networkInterfaces,
    netProtoCounters,
    platform,
    release,
    tmpdir,
    totalmem,
    type,
    uptime,
    bootTime,
    userInfo,
    hostInfo,
    constants,
    EOL
};
