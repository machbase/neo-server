jsh = require("@jsh/process")

r = jsh.readdir(".", (ent) => {
    fname = ent.name.padEnd(30, " ")
    flag = ent.isDir ? (ent.virtual ? "v" : "d") : "-"
    flag += ent.readOnly ? "r-" : "rw"
    fsize = (""+ent.size).padStart(10, " ")
    jsh.print(flag, fsize, " ", fname, "\n")
    return true
})

if (r !== undefined) {
    jsh.print("Error: ", r, "\n")
}