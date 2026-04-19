'use strict';

/**
 * OPC UA module for JSH
 * Provides a thin wrapper around the native OPC UA client bindings.
 */

const _opcua = require('@jsh/opcua');

/**
 * OPC UA client wrapper.
 *
 * Wraps the native client returned by `_opcua.newClient(opt)` and exposes
 * a JavaScript class interface.
 *
 * @param {Object} opt - Client connection options.
 * @param {string} opt.endpoint - OPC UA endpoint URL.
 * @param {number} [opt.readRetryInterval=100] - Retry interval in milliseconds for recoverable read errors.
 * @param {number} [opt.messageSecurityMode=MessageSecurityMode.None] - Security mode to use for the connection.
 */
class Client {
    constructor(opt) {
        if (!opt) {
            throw new Error("missing client options");
        }
        if (opt.messageSecurityMode === undefined) {
            opt.messageSecurityMode = MessageSecurityMode.None;
        }
        this._client = _opcua.newClient(opt);
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
        if (request && !request.timestampsToReturn) {
            request.timestampsToReturn = TimestampsToReturn.Neither;
        }
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

    /**
     * Browses references for one or more OPC UA nodes.
     *
     * @param {Object} request - Browse request object.
     * @param {Array<string>} request.nodes - OPC UA node IDs to browse.
     * @param {number} [request.browseDirection] - Browse direction, typically one of {@link BrowseDirection}.
     * @param {string} [request.referenceTypeId] - Reference type NodeID to follow when browsing, for example "ns=0;i=31".
     * @param {number} [request.nodeClassMask] - Bitmask of {@link NodeClass} values to include in results.
     * @param {number} [request.resultMask] - Bitmask of {@link BrowseResultMask} values selecting fields to return.
     * @param {number} [request.requestedMaxReferencesPerNode=0] - Maximum references returned per node before the server paginates with a continuation point.
     * @returns {Array<Object>} Browse results for each requested node.
     */
    browse(request) {
        if (request && request.browseDirection === undefined) {
            request.browseDirection = BrowseDirection.Forward;
        }
        if (request && request.includeSubtypes === undefined) {
            request.includeSubtypes = true;
        }
        if (request && request.resultMask === undefined) {
            request.resultMask = BrowseResultMask.All;
        }
        return this._client.browse(request);
    }

    /**
     * Continues a paginated browse request using one or more continuation points.
     *
     * @param {Object} request - BrowseNext request object.
     * @param {Array<string>} request.continuationPoints - Base64-encoded continuation points returned by {@link browse} or {@link browseNext}.
     * @param {boolean} [request.releaseContinuationPoints=false] - Whether the server should release the continuation points instead of returning more references.
     * @returns {Array<Object>} Browse results for each continuation point.
     */
    browseNext(request) {
        return this._client.browseNext(request);
    }

    /**
     * Returns the direct children of a given OPC UA node.
     *
     * @param {Object} request - Children request object.
     * @param {string} request.node - OPC UA node ID whose children should be returned.
     * @param {number} [request.nodeClassMask] - Bitmask of {@link NodeClass} values to include in child results.
     * @returns {Array<Object>} Child node descriptions for the requested node.
     */
    children(request) {
        return this._client.children(request);
    }

    attributes(request) {
        return this._client.attributes(request);
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

/**
 * OPC UA attribute ID constants.
 */
const AttributeID = _opcua.AttributeID;

/**
 * OPC UA status code constants.
 */
const StatusCode = _opcua.StatusCode;

module.exports = {
    Client,
    MessageSecurityMode,
    TimestampsToReturn,
    BrowseDirection,
    NodeClass,
    BrowseResultMask,
    AttributeID,
    StatusCode,
};