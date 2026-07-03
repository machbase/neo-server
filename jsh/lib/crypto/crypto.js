'use strict';

const nativeCrypto = require('@jsh/crypto');

/**
 * Generate an auth key pair for Machbase challenge authentication.
 * Supported key types are `ecdsa`(P-256) and `rsa`(2048).
 * @param {string} [type='ecdsa'] key type (`ecdsa` or `rsa`)
 * @returns {{privateKey: string, publicKey: string}}
 */
function generateAuthKeyPair(type = 'ecdsa') {
    return nativeCrypto.generateAuthKeyPair(type);
}

/**
 * Generate an X.509 certificate.
 * @param {{
 *   days: number,
 *   cn?: string,
 *   o?: string[],
 *   ou?: string[],
 *   l?: string[],
 *   st?: string[],
 *   c?: string[],
 *   dns?: string[],
 *   uri?: string[],
 *   san?: string[]
 * }} request certificate request: validity days, subject fields, DNS names, URIs, and SAN values
 * @param {string} publicKey PEM-encoded public key
 * @param {string} signerPrivateKey PEM-encoded private key used to sign the certificate
 * @returns {string} PEM-encoded certificate
 */
function generateX509Certificate(request, publicKey, signerPrivateKey) {
    return nativeCrypto.generateX509Certificate(request, publicKey, signerPrivateKey);
}

/**
 * Write file to host OS filesystem.
 * This helper is intended for paths prefixed with `@` in commands.
 * @param {string} path host path without `@`
 * @param {string} content file content
 * @param {number} [mode=0o600] file mode
 */
function writeHostFile(path, content, mode = 0o600) {
    nativeCrypto.writeHostFile(path, content, mode);
}

module.exports = {
    generateAuthKeyPair,
    generateX509Certificate,
    writeHostFile,
};
