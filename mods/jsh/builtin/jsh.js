jsh = require("@jsh/process")
jsh.print("Welcome to JSH runtime.\n\n")
jsh.print("    This is an JSH command line runtime in BETA stge.\n")
jsh.print("    The commands and features are subjects to change.\n")
jsh.print("    Type 'exit' to exit the shell.\n")
jsh.print("\n")
function printResult(r) {
	if( r == undefined) {
		return
	}
	if( r.value != undefined) {
		jsh.print(r.value, "\n")
	} else {
		jsh.print(r, "\n")
	}
}

var alive = true;
while(alive) {
    jsh.print("jsh", jsh.cwd(), "> ")
    line = jsh.readLine()
	if(line == undefined || line == "" || line == "\n") {
		continue
	}
	line = line.trim()
    parts = jsh.parseCommandLine(line)
    for( i = 0; i < parts.length; i++) {
        p = parts[i]
        args = p.args
        if(args[0] == "exit") {
            alive = false
            break
        } else if(args[0] == "cd") {
            r = jsh.cd(...args.slice(1))
            printResult(r)
        } else if(args[0] == "pwd") {
            r = jsh.cwd(...args.slice(1))
            jsh.print(r, "\n")
        } else {
            try {
                r = jsh.exec(args[0], ...args.slice(1))
                printResult(r)
            } catch(e) {
                jsh.print(e.message, "\n")
            }
        }
    }
}