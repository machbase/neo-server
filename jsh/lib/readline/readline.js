'use strict';

const _readline = require('@jsh/readline');

class ReadLine {
	constructor(options) {
		this.raw = _readline.NewReadLine(this, options || {});
		this.options = options || {};
	}
	readLine(options) {
		return this.raw.readLine({ ...this.options, ...options });
	}
	addHistory(line) {
		this.raw.addHistory(line);
	}
	close() {
		this.raw.close();
	}

	static Backspace = "\x7F";
	static CtrlA = "\x01";
	static CtrlB = "\x02";
	static CtrlC = "\x03";
	static CtrlD = "\x04";
	static CtrlE = "\x05";
	static CtrlF = "\x06";
	static CtrlG = "\x07";
	static CtrlH = "\x08";
	static CtrlI = "\x09";
	static CtrlJ = "\x0A";
	static CtrlK = "\x0B";
	static CtrlL = "\x0C";
	static CtrlM = "\x0D";
	static CtrlN = "\x0E";
	static CtrlO = "\x0F";
	static CtrlP = "\x10";
	static CtrlQ = "\x11";
	static CtrlR = "\x12";
	static CtrlS = "\x13";
	static CtrlT = "\x14";
	static CtrlU = "\x15";
	static CtrlV = "\x16";
	static CtrlW = "\x17";
	static CtrlX = "\x18";
	static CtrlY = "\x19";
	static CtrlZ = "\x1A";
	static CtrlLBracket = "\x1B"; // C-[
	static CtrlBackslash = "\x1C"; // C-\
	static CtrlRBracket = "\x1D"; // C-]
	static CtrlCaret = "\x1E"; // C-^
	static CtrlUnderbar = "\x1F";
	static CtrlHome = "\x1B[1;5H";
	static CtrlEnd = "\x1B[1;5F";
	static CtrlPageUp = "\x1B[5;5~";
	static CtrlPageDown = "\x1B[6;5~";
	static Enter = "\r";
	static Escape = "\x1B";
	static AltA = "\x1Ba";
	static AltB = "\x1Bb";
	static ALTBackspace = "\x1B\b";
	static AltC = "\x1Bc";
	static AltD = "\x1Bd";
	static AltE = "\x1Be";
	static AltF = "\x1Bf";
	static AltG = "\x1Bg";
	static AltH = "\x1Bh";
	static AltI = "\x1Bi";
	static AltJ = "\x1Bj";
	static AltK = "\x1Bk";
	static AltL = "\x1Bl";
	static AltM = "\x1Bm";
	static AltN = "\x1Bn";
	static AltO = "\x1Bo";
	static AltP = "\x1Bp";
	static AltQ = "\x1Bq";
	static AltR = "\x1Br";
	static AltS = "\x1Bs";
	static AltT = "\x1Bt";
	static AltU = "\x1Bu";
	static AltV = "\x1Bv";
	static AltW = "\x1Bw";
	static AltX = "\x1Bx";
	static AltY = "\x1By";
	static AltZ = "\x1Bz";
	static AltOEM2 = "\x1B/";
	static Clear = "\x0C";
	static Ctrl = "\x11";
	static CtrlBreak = "\x03";
	static Delete = "\x1B[3~";
	static Down = "\x1B[B";
	static CtrlDown = "\x1B[1;5B";
	static End = "\x1B[F";
	static F10 = "\x1B[21~";
	static F11 = "\x1B[23~";
	static F12 = "\x1B[24~";
	static F13 = "\x7C";
	static F14 = "\x7D";
	static F15 = "\x7E";
	static F16 = "\x7F";
	static F17 = "\x80";
	static F18 = "\x81";
	static F19 = "\x82";
	static F1 = "\x1B[OP";
	static F20 = "\x83";
	static F21 = "\x84";
	static F22 = "\x85";
	static F23 = "\x86";
	static F24 = "\x87";
	static F2 = "\x1B[OQ";
	static F3 = "\x1B[OR";
	static F4 = "\x1B[OS";
	static F5 = "\x1B[15~";
	static F6 = "\x1B[16~";
	static F7 = "\x1B[17~";
	static F8 = "\x1B[18~";
	static F9 = "\x1B[20~";
	static Home = "\x1B[H";
	static Left = "\x1B[D";
	static CtrlLeft = "\x1B[1;5D";
	static PageDown = "\x1B[6~";
	static PageUp = "\x1B[5~";
	static Pause = "\x13";
	static Right = "\x1B[C";
	static CtrlRight = "\x1B[1;5C";
	static Up = "\x1B[A";
	static CtrlUp = "\x1B[1;5A";
	static ShiftTab = "\x1B[Z";
}

module.exports = {
	ReadLine: ReadLine,
}