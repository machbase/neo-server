jsh = require("@jsh/process")
jsh.print("JSH shell\n\n")

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
    line = jsh.readline()
	if(line == undefined || line == "" || line == "\n") {
		continue
	}
	line = line.trim()
	args = line.split(/[ ,]+/)
    if(args[0] == "exit") {
        alive = false
	} else if(args[0] == "cd") {
		r = jsh.chdir(args[1])
		printResult(r)
	} else if(args[0] == "pwd") {
		jsh.print(jsh.cwd(), "\n")
    } else {
		try {
			r = jsh.exec(args[0], ...args.slice(1))
			printResult(r)
		} catch(e) {
			jsh.print(e.message, "\n")
		}
    }
}