'use strict';
module.exports = {
	name: 'help',
	description: 'Display help documents',
	usage: 'Usage: help [namespace:]command [subcommand...]',
	options: {
		help: { type: 'boolean', short: 'h', description: 'Show this help message', default: false },
	},
};