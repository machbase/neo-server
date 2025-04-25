try {
    var a = 1;
    var b = 2;
    var c = a + b;
    if (c != 3) {
        throw new Error("Test failed: c is not equal to 3");
    }
    c.undefinedFunction(); // This will throw an error
} catch (e) {
    console.log("Error: " + e);
    console.log("Error: " + e.message);
}