try {
    const { ReadLine } = require('readline');
    const r = new ReadLine({
        prompt: function (line) {
            return line === 0 ? "get > " : "... > ";
        },
        submitOnEnterWhen: (lines, idx) => {
            return lines[idx].endsWith(";");
        },
    });
    const line = r.readLine();
    if (line instanceof Error) {
        throw line;
    }
    console.println("--- line ---\n" + line);
} catch (e) {
    console.println("ERR:", e.message);
}