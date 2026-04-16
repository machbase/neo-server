'use strict';

/**
 * Git module for JSH.
 * Provides simple remote inspection and clone helpers backed by go-git.
 */

const _git = require('@jsh/git');

/**
 * A single reference discovered from a remote repository.
 * @typedef {Object} GitRemoteRef
 * @property {string} name Full ref name such as refs/heads/main.
 * @property {string} shortName Short ref name such as main or v1.0.0.
 * @property {string} hash Commit hash for the ref.
 * @property {boolean} isBranch True when the ref is a branch.
 * @property {boolean} isTag True when the ref is a tag.
 * @property {boolean} isRemote True when the ref is a remote-tracking ref.
 * @property {boolean} isHEAD True when the ref represents HEAD.
 * @property {string} [target] Symbolic target when the ref is symbolic.
 */

/**
 * Clone options for a repository checkout.
 * @typedef {Object} GitCloneOptions
 * @property {string} [ref=''] Tag or branch name to checkout.
 * @property {'tag'|'branch'|string} [refType=''] Ref type for the checkout target.
 * @property {number} [depth=1] Clone depth. Use 0 for a full clone.
 * @property {boolean} [singleBranch=true] Clone only the selected branch when possible.
 * @property {boolean} [removeGitDir=false] Remove the .git directory after clone.
 */

/**
 * Result returned after cloning a repository.
 * @typedef {Object} GitCloneResult
 * @property {string} headRef Checked out HEAD ref name.
 * @property {string} headHash Checked out HEAD commit hash.
 */

/**
 * List all refs exposed by a remote repository without cloning it.
 *
 * @param {string} url Remote Git URL.
 * @returns {GitRemoteRef[]} Remote refs.
 */
function listRemoteRefs(url) {
    return _git.listRemoteRefs(String(url || '').trim());
}

/**
 * Clone a remote repository into a target directory.
 *
 * @param {string} url Remote Git URL.
 * @param {string} dir Target directory path.
 * @param {GitCloneOptions} [options={}] Clone options.
 * @returns {GitCloneResult} Information about the checked out HEAD.
 */
function cloneRepository(url, dir, options = {}) {
    return _git.cloneRepository(
        String(url || '').trim(),
        String(dir || '').trim(),
        options || {}
    );
}

module.exports = {
    listRemoteRefs,
    cloneRepository,
};
