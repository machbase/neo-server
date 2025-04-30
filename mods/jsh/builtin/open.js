jsh = require("@jsh/process")
args = jsh.args()
if( args.length != 2) {
    jsh.print("Usage: open <file>","\n")
} else {
    try {
        file = args[1];
        result = jsh.exists(file);
        if( result.isDir) {
            jsh.print("Cannot open a directory\n")
        } else {
            jsh.print("Opening file: ", result.path, "\n")
            jsh.openEditor(result.path)
        }
    } catch(e) {
        jsh.print("Error:", e, "\n")
    }
}
