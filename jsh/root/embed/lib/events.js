'use strict';

// Simple EventEmitter implementation for JSH
class EventEmitter {
    constructor() {
        this._events = {};
        this._maxListeners = 10;
    }
    
    on(event, listener) {
        if (typeof listener !== 'function') {
            throw new TypeError('The listener must be a function');
        }
        
        if (!this._events[event]) {
            this._events[event] = [];
        }
        
        this._events[event].push(listener);
        
        // Warn if too many listeners
        if (this._events[event].length > this._maxListeners) {
            console.warn(`MaxListenersExceededWarning: Possible EventEmitter memory leak detected. ${this._events[event].length} ${event} listeners added.`);
        }
        
        return this;
    }
    
    addListener(event, listener) {
        return this.on(event, listener);
    }
    
    once(event, listener) {
        const onceWrapper = (...args) => {
            listener.apply(this, args);
            this.removeListener(event, onceWrapper);
        };
        onceWrapper.listener = listener;
        return this.on(event, onceWrapper);
    }
    
    removeListener(event, listener) {
        if (!this._events[event]) {
            return this;
        }
        
        const listeners = this._events[event];
        for (let i = listeners.length - 1; i >= 0; i--) {
            if (listeners[i] === listener || listeners[i].listener === listener) {
                listeners.splice(i, 1);
                break;
            }
        }
        
        if (listeners.length === 0) {
            delete this._events[event];
        }
        
        return this;
    }
    
    off(event, listener) {
        return this.removeListener(event, listener);
    }
    
    removeAllListeners(event) {
        if (event) {
            delete this._events[event];
        } else {
            this._events = {};
        }
        return this;
    }
    
    emit(event, ...args) {
        if (!this._events[event]) {
            return false;
        }
        
        const listeners = this._events[event].slice();
        for (const listener of listeners) {
            try {
                listener.apply(this, args);
            } catch (err) {
                // If there's an error listener, emit error event
                if (event !== 'error' && this._events['error']) {
                    this.emit('error', err);
                } else if (event === 'error') {
                    throw err;
                }
            }
        }
        
        return true;
    }
    
    listeners(event) {
        return this._events[event] ? this._events[event].slice() : [];
    }
    
    listenerCount(event) {
        return this._events[event] ? this._events[event].length : 0;
    }
    
    eventNames() {
        return Object.keys(this._events);
    }
    
    setMaxListeners(n) {
        this._maxListeners = n;
        return this;
    }
    
    getMaxListeners() {
        return this._maxListeners;
    }
}

module.exports = EventEmitter;

