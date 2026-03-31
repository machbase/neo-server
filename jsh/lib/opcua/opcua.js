'use strict';

/**
 * OPC UA module for JSH
 * Provides a thin wrapper around the native OPC UA client bindings.
 */

const _opcua = require('@jsh/opcua');

/**
 * OPC UA client wrapper.
 *
 * Composes the native client returned by `_opcua.Client` and exposes
 * a JavaScript class interface.
 *
 * @param {...any} args - Constructor arguments forwarded to the native OPC UA client.
 * The first argument should be an options object.
 * @param {Object} [args[0]] - Client connection options.
 * @param {string} args[0].endpoint - OPC UA endpoint URL.
 * @param {number} [args[0].readRetryInterval=100] - Retry interval in milliseconds for recoverable read errors.
 * @param {number} [args[0].messageSecurityMode=MessageSecurityMode.None] - Security mode to use for the connection.
 */
class Client {
    constructor(...args) {
        this._client = Reflect.construct(_opcua.Client, args);
    }

    /**
     * Closes the OPC UA connection.
     * @returns {any} Native close result.
     */
    close() {
        return this._client.close(...arguments);
    }

    /**
     * Reads one or more OPC UA nodes.
     * @param {Object} request - Read request object.
     * @param {Array<string>} request.nodes - OPC UA node IDs to read.
     * @param {number} [request.maxAge=0] - Maximum age in milliseconds.
     * @param {number} [request.timestampsToReturn=TimestampsToReturn.Neither] - Timestamp return mode.
     * @returns {Array<Object>} Read results for each node.
     */
    read(request) {
        return this._client.read(request);
    }

    /**
     * Writes one or more OPC UA node values.
     * @param {...Object} writes - Write entries.
     * @param {string} writes[].node - OPC UA node ID to write.
     * @param {any} writes[].value - Value to write.
     * @returns {Object} Write response.
     */
    write(...writes) {
        return this._client.write(...writes);
    }

    browse(request) {
        return this._client.browse(request);
    }
}

/**
 * OPC UA message security mode constants.
 */
const MessageSecurityMode = _opcua.MessageSecurityMode;

/**
 * OPC UA timestamp return mode constants for read operations.
 */
const TimestampsToReturn = _opcua.TimestampsToReturn;

/**
 * OPC UA browse direction constants.
 */
const BrowseDirection = _opcua.BrowseDirection;

/**
 * OPC UA node class constants.
 */
const NodeClass = _opcua.NodeClass;

/**
 * OPC UA browse result mask constants.
 */
const BrowseResultMask = _opcua.BrowseResultMask;

module.exports = {
    Client,
    MessageSecurityMode,
    TimestampsToReturn,
    BrowseDirection,
    NodeClass,
    BrowseResultMask,
};